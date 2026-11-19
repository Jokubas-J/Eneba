package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/auth"
)

// Accounts is what the HTTP layer needs from the auth service (declared here, by
// the code that uses it, so handler tests can pass a fake).
type Accounts interface {
	Register(ctx context.Context, email, password string) (auth.User, string, error)
	Login(ctx context.Context, email, password string) (auth.User, string, error)
	Logout(ctx context.Context, token string) error
	UserBySession(ctx context.Context, token string) (auth.User, bool, error)
}

// sessionCookie is the name of the cookie that carries the session token.
const sessionCookie = "session"

// Limit on request bodies: a login form is a few hundred bytes. Without a limit a
// client could make the server read an endless body into memory.
const maxBodyBytes = 64 << 10 // 64 KiB

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// userResponse is {"user": {...}} or {"user": null} when nobody is signed in.
type userResponse struct {
	User *auth.User `json:"user"`
}

// authHandlers groups the auth endpoints with what they share.
type authHandlers struct {
	accounts Accounts
	// secureCookies adds the Secure flag (cookie only sent over HTTPS). On in
	// production; off locally, where the dev server is plain http://localhost.
	secureCookies bool
}

func (h authHandlers) register(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if !decodeJSON(w, r, &body) {
		return
	}
	user, token, err := h.accounts.Register(r.Context(), body.Email, body.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrPasswordTooShort), errors.Is(err, auth.ErrPasswordTooLong):
		writeError(w, http.StatusBadRequest, "invalid_request", capitalize(err.Error()))
		return
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", capitalize(err.Error()))
		return
	case errors.Is(err, auth.ErrTooManySignups):
		w.Header().Set("Retry-After", "600")
		writeError(w, http.StatusTooManyRequests, "too_many_signups", capitalize(err.Error()))
		return
	case err != nil:
		writeInternalError(w, r, err)
		return
	}
	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, userResponse{User: &user})
}

func (h authHandlers) login(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if !decodeJSON(w, r, &body) {
		return
	}
	user, token, err := h.accounts.Login(r.Context(), body.Email, body.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", capitalize(err.Error()))
		return
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", capitalize(err.Error()))
		return
	case err != nil:
		writeInternalError(w, r, err)
		return
	}
	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, userResponse{User: &user})
}

// logout always succeeds from the visitor's point of view: whatever the state of
// the session, afterwards they are signed out.
func (h authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.accounts.Logout(r.Context(), cookie.Value); err != nil {
			writeInternalError(w, r, err)
			return
		}
	}
	h.clearSessionCookie(w)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h authHandlers) me(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.currentUser(r)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	if !ok {
		// An expired or unknown token: remove the useless cookie from the browser.
		if _, err := r.Cookie(sessionCookie); err == nil {
			h.clearSessionCookie(w)
		}
		writeJSON(w, http.StatusOK, userResponse{User: nil})
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: &user})
}

// currentUser returns the signed-in user of a request. It is a function called by
// the handlers that need it, not a middleware run for every request, so browsing
// the catalog costs no session lookup.
func (h authHandlers) currentUser(r *http.Request) (auth.User, bool, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.User{}, false, nil // no cookie: not signed in
	}
	return h.accounts.UserBySession(r.Context(), cookie.Value)
}

// setSessionCookie stores the token in the browser.
//
//   - HttpOnly: JavaScript in the page cannot read it, so a script injected into the
//     page (XSS) cannot steal the session.
//   - Secure (production): only sent over HTTPS.
//   - SameSite=Lax: the browser does not send it with requests started by other
//     sites (form posts, fetch), the main defence against cross-site request forgery.
//   - Path=/: sent with every request to our domain.
func (h authHandlers) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration / time.Second),
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	// A response that sets a session must never be stored by a cache in between.
	w.Header().Set("Cache-Control", "no-store")
}

func (h authHandlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // "delete now"
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// decodeJSON reads a JSON request body into v. On failure it writes a 400 response
// and returns false, so a handler can simply return.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "The request body is not valid JSON.")
		return false
	}
	return true
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:] + "."
}

// checkOrigin rejects state-changing requests (POST, PUT, PATCH, DELETE) that a
// browser sends from another site.
//
// SameSite=Lax already keeps the session cookie off such requests in current
// browsers; this is the second layer the OWASP CSRF guidance recommends, and it also
// covers requests that need no cookie (a forged login form). Browsers always send
// an Origin header with these requests and scripts cannot change it. Requests
// without Origin are not from a browser page (curl, Stripe's webhooks later) and
// cannot carry a victim's cookies, so they pass.
func checkOrigin(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" && origin != allowedOrigin {
				writeError(w, http.StatusForbidden, "forbidden_origin", "Requests from other sites are not allowed.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
