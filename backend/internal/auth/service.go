package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors the HTTP layer turns into responses.
var (
	ErrInvalidEmail       = errors.New("enter a valid e-mail address")
	ErrPasswordTooShort   = fmt.Errorf("the password must have at least %d characters", minPasswordLength)
	ErrPasswordTooLong    = fmt.Errorf("the password must have at most %d characters", maxPasswordLength)
	ErrEmailTaken         = errors.New("an account with this e-mail address already exists")
	ErrInvalidCredentials = errors.New("wrong e-mail address or password")
	ErrTooManyAttempts    = errors.New("too many failed sign-in attempts, try again later")
	ErrTooManySignups     = errors.New("too many new accounts right now, please try again in a few minutes")
)

const (
	// NIST SP 800-63B: at least 8 characters, and no composition rules ("one digit,
	// one symbol"), which mostly produce predictable passwords.
	minPasswordLength = 8
	// An upper bound so nobody can make the server hash a megabyte-long password.
	maxPasswordLength = 128
	maxEmailLength    = 254 // the longest address the e-mail standards allow

	// SessionDuration is how long a sign-in lasts. It is also the cookie's lifetime.
	SessionDuration = 30 * 24 * time.Hour

	// Failed logins per e-mail address, within the window, before refusing further
	// attempts for that address until the window has passed.
	maxFailedLogins   = 5
	failedLoginWindow = 15 * time.Minute

	// New accounts for the whole site within the window. Site-wide rather than per
	// client: like the login limiter, it does not trust a client IP taken from a
	// proxy header. Counted in the database, so it survives restarts. The trade-off:
	// a flood of sign-ups pauses registration for everyone for a few minutes; a real
	// shop would add e-mail verification or a CAPTCHA.
	MaxSignupsPerWindow = 30
	signupWindow        = 10 * time.Minute
)

// User is the signed-in account as the rest of the application sees it.
type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// Service implements registration, login, logout and session lookup.
type Service struct {
	pool    *pgxpool.Pool
	limiter *loginLimiter
	// dummyHash is checked when an unknown e-mail address tries to log in, so the
	// answer takes as long as for a known address with a wrong password. Otherwise
	// the response time would tell an attacker which addresses have accounts.
	dummyHash string
}

func NewService(pool *pgxpool.Pool) (*Service, error) {
	dummy, err := HashPassword("not a real password")
	if err != nil {
		return nil, err
	}
	return &Service{pool: pool, limiter: newLoginLimiter(), dummyHash: dummy}, nil
}

// normalizeEmail validates an address and returns the form it is stored in.
// Addresses are compared case-insensitively ("A@x.com" is "a@x.com"), so they are
// stored lower-cased; the users table also checks that with a CHECK constraint.
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > maxEmailLength {
		return "", ErrInvalidEmail
	}
	// ParseAddress also accepts "Name <a@b.com>"; requiring the result to equal the
	// input allows only the bare address.
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func validatePassword(password string) error {
	// Characters, not bytes: "ąčęėįšųūž" is 9 characters but 18 bytes.
	switch n := utf8.RuneCountInString(password); {
	case n < minPasswordLength:
		return ErrPasswordTooShort
	case n > maxPasswordLength:
		return ErrPasswordTooLong
	}
	return nil
}

// Register creates an account and signs it in. It returns the user and the session
// token for the cookie.
func (s *Service) Register(ctx context.Context, email, password string) (User, string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, "", err
	}
	if err := validatePassword(password); err != nil {
		return User{}, "", err
	}
	// Checked before hashing: Argon2 is deliberately expensive, so a stream of sign-ups
	// should not get to make the server hash.
	var recent int
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE created_at > now() - make_interval(secs => $1)`,
		signupWindow.Seconds()).Scan(&recent)
	if err != nil {
		return User{}, "", fmt.Errorf("count recent sign-ups: %w", err)
	}
	if recent >= MaxSignupsPerWindow {
		return User{}, "", ErrTooManySignups
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, "", err
	}

	user := User{Email: email}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, email, hash).Scan(&user.ID)
	// The UNIQUE constraint on email decides "already registered". Checking with a
	// SELECT first would leave a gap in which two requests could both pass.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return User{}, "", ErrEmailTaken
	}
	if err != nil {
		return User{}, "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

// Login checks the credentials and starts a session.
//
// Unknown address and wrong password give the same error, so the form cannot be
// used to find out who has an account.
func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, "", ErrInvalidCredentials
	}
	if !s.limiter.allow(email) {
		return User{}, "", ErrTooManyAttempts
	}

	var user User
	var hash string
	err = s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = $1`, email).Scan(&user.ID, &user.Email, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		CheckPassword(password, s.dummyHash) // same work as a real check; result ignored
		s.limiter.fail(email)
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", fmt.Errorf("load user: %w", err)
	}

	ok, err := CheckPassword(password, hash)
	if err != nil {
		return User{}, "", err
	}
	if !ok {
		s.limiter.fail(email)
		return User{}, "", ErrInvalidCredentials
	}
	s.limiter.reset(email)

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func (s *Service) createSession(ctx context.Context, userID int64) (string, error) {
	token, hash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hash, time.Now().Add(SessionDuration))
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	// Housekeeping: this user's expired sessions are of no use any more. Done here,
	// at a moment the user is active, instead of with a background job.
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= now()`, userID); err != nil {
		return "", fmt.Errorf("remove expired sessions: %w", err)
	}
	return token, nil
}

// UserBySession returns the user a session token belongs to. ok is false when the
// token is unknown or expired; that is not an error, just "not signed in".
func (s *Service) UserBySession(ctx context.Context, token string) (user User, ok bool, err error) {
	if token == "" {
		return User{}, false, nil
	}
	err = s.pool.QueryRow(ctx, `
		SELECT u.id, u.email
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)).Scan(&user.ID, &user.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("load session: %w", err)
	}
	return user, true, nil
}

// Logout ends the session. Deleting the row (not just clearing the cookie) means the
// token stops working everywhere, even if a copy of it exists somewhere.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// loginLimiter counts failed logins per e-mail address in memory.
//
// Per address rather than per IP address: behind Render's proxy the client IP comes
// from a header that would have to be trusted, and an attacker can change IPs
// cheaply. The trade-off: someone who knows an address can block its logins for 15
// minutes by failing five times. In memory means the counts are lost on restart and
// are not shared between several server instances; we run one.
type loginLimiter struct {
	mu       sync.Mutex // the HTTP server handles requests concurrently
	failures map[string]*failureCount
	now      func() time.Time // replaceable in tests
}

type failureCount struct {
	count int
	since time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: map[string]*failureCount{}, now: time.Now}
}

func (l *loginLimiter) allow(email string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.failures[email]
	if !ok {
		return true
	}
	if l.now().Sub(f.since) > failedLoginWindow {
		delete(l.failures, email) // the window has passed: start over
		return true
	}
	return f.count < maxFailedLogins
}

func (l *loginLimiter) fail(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.failures[email]
	if !ok || l.now().Sub(f.since) > failedLoginWindow {
		l.failures[email] = &failureCount{count: 1, since: l.now()}
		return
	}
	f.count++
}

func (l *loginLimiter) reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, email)
}
