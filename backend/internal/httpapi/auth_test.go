package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/auth"
)

// fakeAccounts: one known user with one valid session token, no database.
type fakeAccounts struct {
	loggedOut string
}

var testUser = auth.User{ID: 7, Email: "user@example.com"}

func (f *fakeAccounts) Register(ctx context.Context, email, password string) (auth.User, string, error) {
	switch {
	case email == "taken@example.com":
		return auth.User{}, "", auth.ErrEmailTaken
	case len(password) < 8:
		return auth.User{}, "", auth.ErrPasswordTooShort
	}
	return testUser, "new-token", nil
}

func (f *fakeAccounts) Login(ctx context.Context, email, password string) (auth.User, string, error) {
	switch {
	case email == "blocked@example.com":
		return auth.User{}, "", auth.ErrTooManyAttempts
	case password != "right password":
		return auth.User{}, "", auth.ErrInvalidCredentials
	}
	return testUser, "login-token", nil
}

func (f *fakeAccounts) Logout(ctx context.Context, token string) error {
	f.loggedOut = token
	return nil
}

func (f *fakeAccounts) UserBySession(ctx context.Context, token string) (auth.User, bool, error) {
	if token == "boom" {
		return auth.User{}, false, errors.New("db down")
	}
	return testUser, token == "valid-token", nil
}

const testOrigin = "https://shop.example"

func authRequest(t *testing.T, accounts Accounts, secure bool, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{Accounts: accounts, AllowedOrigin: testOrigin, SecureCookies: secure}).ServeHTTP(rec, req)
	return rec
}

func sessionCookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func TestRegisterAndLoginSetTheSessionCookie(t *testing.T) {
	for _, tc := range []struct {
		path, body, token string
		status            int
	}{
		{"/api/auth/register", `{"email":"new@example.com","password":"long enough"}`, "new-token", http.StatusCreated},
		{"/api/auth/login", `{"email":"user@example.com","password":"right password"}`, "login-token", http.StatusOK},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := authRequest(t, &fakeAccounts{}, true, http.MethodPost, tc.path, tc.body, nil)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.status, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), `"email":"user@example.com"`) {
				t.Errorf("body = %s", rec.Body)
			}
			c := sessionCookieOf(rec)
			if c == nil || c.Value != tc.token {
				t.Fatalf("session cookie = %+v, want value %q", c, tc.token)
			}
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 30*24*60*60 {
				t.Errorf("cookie flags = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/, 30 days", c)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("a response that sets a session must not be cacheable")
			}
		})
	}

	t.Run("locally the cookie is not Secure (plain http://localhost)", func(t *testing.T) {
		rec := authRequest(t, &fakeAccounts{}, false, http.MethodPost, "/api/auth/login",
			`{"email":"user@example.com","password":"right password"}`, nil)
		if c := sessionCookieOf(rec); c == nil || c.Secure {
			t.Errorf("cookie = %+v, want one without Secure", c)
		}
	})
}

func TestAuthErrors(t *testing.T) {
	tests := []struct {
		name, path, body string
		status           int
		code             string
	}{
		{"short password", "/api/auth/register", `{"email":"a@example.com","password":"short"}`, 400, "invalid_request"},
		{"taken address", "/api/auth/register", `{"email":"taken@example.com","password":"long enough"}`, 409, "email_taken"},
		{"wrong password", "/api/auth/login", `{"email":"user@example.com","password":"wrong"}`, 401, "invalid_credentials"},
		{"too many attempts", "/api/auth/login", `{"email":"blocked@example.com","password":"x"}`, 429, "too_many_attempts"},
		{"not JSON", "/api/auth/login", `email=user@example.com`, 400, "invalid_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := authRequest(t, &fakeAccounts{}, true, http.MethodPost, tt.path, tt.body, nil)
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), `"code":"`+tt.code+`"`) {
				t.Errorf("status = %d, body = %s; want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			if sessionCookieOf(rec) != nil {
				t.Error("a failed request set a session cookie")
			}
		})
	}
}

func TestMe(t *testing.T) {
	tests := []struct {
		name, cookie string
		wantBody     string
		wantCleared  bool
	}{
		{"signed in", "valid-token", `{"user":{"id":7,"email":"user@example.com"}}`, false},
		{"no cookie", "", `{"user":null}`, false},
		{"expired or unknown token: the cookie is removed", "old-token", `{"user":null}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.cookie != "" {
				headers["Cookie"] = sessionCookie + "=" + tt.cookie
			}
			rec := authRequest(t, &fakeAccounts{}, true, http.MethodGet, "/api/auth/me", "", headers)
			if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != tt.wantBody {
				t.Errorf("status = %d, body = %s; want %s", rec.Code, rec.Body, tt.wantBody)
			}
			c := sessionCookieOf(rec)
			if cleared := c != nil && c.MaxAge < 0; cleared != tt.wantCleared {
				t.Errorf("cookie cleared = %v, want %v", cleared, tt.wantCleared)
			}
		})
	}

	t.Run("a failing session lookup is a 500, not 'signed out'", func(t *testing.T) {
		rec := authRequest(t, &fakeAccounts{}, true, http.MethodGet, "/api/auth/me", "", map[string]string{"Cookie": sessionCookie + "=boom"})
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})
}

func TestLogout(t *testing.T) {
	accounts := &fakeAccounts{}
	rec := authRequest(t, accounts, true, http.MethodPost, "/api/auth/logout", "", map[string]string{"Cookie": sessionCookie + "=valid-token"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if accounts.loggedOut != "valid-token" {
		t.Errorf("logged out token = %q, want the cookie's token", accounts.loggedOut)
	}
	if c := sessionCookieOf(rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it deleted", c)
	}

	// Without a cookie, logout still succeeds.
	if rec := authRequest(t, &fakeAccounts{}, true, http.MethodPost, "/api/auth/logout", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("logout without cookie: status = %d", rec.Code)
	}
}

func TestOriginCheck(t *testing.T) {
	body := `{"email":"user@example.com","password":"right password"}`
	tests := []struct {
		name   string
		origin string
		want   int
	}{
		{"our frontend", testOrigin, http.StatusOK},
		{"no Origin header (not a browser page)", "", http.StatusOK},
		{"another site", "https://evil.example", http.StatusForbidden},
		{"same host, other scheme", "http://shop.example", http.StatusForbidden},
		{"look-alike host", "https://shop.example.evil.example", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.origin != "" {
				headers["Origin"] = tt.origin
			}
			rec := authRequest(t, &fakeAccounts{}, true, http.MethodPost, "/api/auth/login", body, headers)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusForbidden && sessionCookieOf(rec) != nil {
				t.Error("a refused request set a cookie")
			}
		})
	}

	t.Run("reading is allowed from anywhere", func(t *testing.T) {
		rec := authRequest(t, &fakeAccounts{}, true, http.MethodGet, "/api/auth/me", "", map[string]string{"Origin": "https://evil.example"})
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}
