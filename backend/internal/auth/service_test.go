package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Jokubas-J/Eneba/backend/internal/auth"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

func newService(t *testing.T) (*auth.Service, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	// Each test starts without accounts. (Orders reference users; the test database
	// has none yet, and TRUNCATE ... CASCADE would also empty them if it had.)
	if _, err := pool.Exec(ctx, `TRUNCATE users, sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	service, err := auth.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	return service, pool, ctx
}

func TestRegisterLoginLogout(t *testing.T) {
	service, pool, ctx := newService(t)

	user, token, err := service.Register(ctx, "  Jonas@Example.com ", "a long enough password")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Email != "jonas@example.com" || user.ID == 0 || token == "" {
		t.Fatalf("Register() = %+v, %q", user, token)
	}

	t.Run("the password and the token are not stored in plain text", func(t *testing.T) {
		var hash, tokenHash string
		err := pool.QueryRow(ctx, `
			SELECT u.password_hash, s.token_hash FROM users u JOIN sessions s ON s.user_id = u.id`).Scan(&hash, &tokenHash)
		if err != nil {
			t.Fatal(err)
		}
		if hash == "a long enough password" || tokenHash == token {
			t.Error("a secret is stored as it is")
		}
	})

	t.Run("the session from registering identifies the user", func(t *testing.T) {
		got, ok, err := service.UserBySession(ctx, token)
		if err != nil || !ok || got != user {
			t.Errorf("UserBySession() = %+v, %v, %v", got, ok, err)
		}
	})

	t.Run("the same address cannot register twice, in any letter case", func(t *testing.T) {
		if _, _, err := service.Register(ctx, "JONAS@example.com", "another password"); !errors.Is(err, auth.ErrEmailTaken) {
			t.Errorf("error = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("login with the right password, any letter case in the address", func(t *testing.T) {
		got, loginToken, err := service.Login(ctx, "JONAS@EXAMPLE.COM", "a long enough password")
		if err != nil || got != user || loginToken == "" || loginToken == token {
			t.Errorf("Login() = %+v, %q, %v; want the user and a new token", got, loginToken, err)
		}
	})

	t.Run("wrong password and unknown address give the same error", func(t *testing.T) {
		_, _, wrong := service.Login(ctx, "jonas@example.com", "not the password")
		_, _, unknown := service.Login(ctx, "nobody@example.com", "a long enough password")
		_, _, malformed := service.Login(ctx, "not an address", "a long enough password")
		for _, err := range []error{wrong, unknown, malformed} {
			if !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Errorf("error = %v, want ErrInvalidCredentials", err)
			}
		}
	})

	t.Run("logout ends the session everywhere", func(t *testing.T) {
		if err := service.Logout(ctx, token); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := service.UserBySession(ctx, token); ok {
			t.Error("the session still works after logout")
		}
		// Logging out twice, or with no token, is not an error.
		if err := service.Logout(ctx, token); err != nil {
			t.Errorf("second Logout() error = %v", err)
		}
		if err := service.Logout(ctx, ""); err != nil {
			t.Errorf("Logout(\"\") error = %v", err)
		}
	})
}

func TestRegisterValidation(t *testing.T) {
	service, _, ctx := newService(t)

	tests := []struct {
		email, password string
		want            error
	}{
		{"not-an-address", "a long enough password", auth.ErrInvalidEmail},
		{"user@example.com", "short", auth.ErrPasswordTooShort},
	}
	for _, tt := range tests {
		if _, _, err := service.Register(ctx, tt.email, tt.password); !errors.Is(err, tt.want) {
			t.Errorf("Register(%q, %q) error = %v, want %v", tt.email, tt.password, err, tt.want)
		}
	}
}

func TestSessions(t *testing.T) {
	service, pool, ctx := newService(t)
	_, token, err := service.Register(ctx, "user@example.com", "a long enough password")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("unknown and empty tokens are simply not signed in", func(t *testing.T) {
		for _, tok := range []string{"", "made-up-token"} {
			if _, ok, err := service.UserBySession(ctx, tok); ok || err != nil {
				t.Errorf("UserBySession(%q) = %v, %v; want not signed in, no error", tok, ok, err)
			}
		}
	})

	t.Run("an expired session does not work", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE sessions SET expires_at = now() - interval '1 second'`); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := service.UserBySession(ctx, token); ok {
			t.Error("an expired session was accepted")
		}
	})

	t.Run("expired sessions are removed at the next login", func(t *testing.T) {
		if _, _, err := service.Login(ctx, "user@example.com", "a long enough password"); err != nil {
			t.Fatal(err)
		}
		var expired int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE expires_at <= now()`).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if expired != 0 {
			t.Errorf("%d expired sessions left", expired)
		}
	})
}

func TestLoginIsLimitedAfterRepeatedFailures(t *testing.T) {
	service, _, ctx := newService(t)
	if _, _, err := service.Register(ctx, "user@example.com", "a long enough password"); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, _, err := service.Login(ctx, "user@example.com", "wrong password"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("error = %v, want ErrInvalidCredentials", err)
		}
	}
	// Now even the right password is refused for this address, without being checked.
	if _, _, err := service.Login(ctx, "User@Example.com", "a long enough password"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Errorf("error = %v, want ErrTooManyAttempts", err)
	}
}
