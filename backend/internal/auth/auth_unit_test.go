package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %q, want the PHC format with our parameters", hash)
	}
	if strings.Contains(hash, "correct horse") {
		t.Error("the hash contains the password")
	}

	ok, err := CheckPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Errorf("right password: ok = %v, err = %v", ok, err)
	}
	ok, err = CheckPassword("correct horse battery stapler", hash)
	if err != nil || ok {
		t.Errorf("wrong password: ok = %v, err = %v", ok, err)
	}

	// Same password, different salt: the stored hashes must differ.
	again, _ := HashPassword("correct horse battery staple")
	if again == hash {
		t.Error("two hashes of the same password are identical (salt not random?)")
	}
}

func TestCheckPasswordRejectsMalformedHashes(t *testing.T) {
	for _, encoded := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA", // argon2i, not argon2id
		"$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=x,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$!!!$aGFzaA",
	} {
		if ok, err := CheckPassword("anything", encoded); err == nil || ok {
			t.Errorf("CheckPassword(%q) = %v, %v; want an error", encoded, ok, err)
		}
	}
}

func TestSessionToken(t *testing.T) {
	token, hash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 { // 32 bytes in unpadded base64url
		t.Errorf("token length = %d, want 43", len(token))
	}
	if hash != hashToken(token) || len(hash) != 64 || hash == token {
		t.Errorf("hash = %q, want the 64-character SHA-256 of the token", hash)
	}
	other, _, _ := newSessionToken()
	if other == token {
		t.Error("two tokens are identical")
	}
}

func TestNormalizeEmail(t *testing.T) {
	valid := map[string]string{
		"user@example.com":       "user@example.com",
		"  User@Example.COM  ":   "user@example.com",
		"first.last+tag@mail.lt": "first.last+tag@mail.lt",
	}
	for input, want := range valid {
		if got, err := normalizeEmail(input); err != nil || got != want {
			t.Errorf("normalizeEmail(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "user", "user@", "@example.com", "user@localhost", "Name <user@example.com>",
		"user@example.com, other@example.com", strings.Repeat("a", 250) + "@example.com"} {
		if _, err := normalizeEmail(input); err != ErrInvalidEmail {
			t.Errorf("normalizeEmail(%q) error = %v, want ErrInvalidEmail", input, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := map[string]error{
		"1234567":                 ErrPasswordTooShort,
		"12345678":                nil,
		"ąčęėįšųū":                nil, // 8 characters (16 bytes)
		strings.Repeat("x", 128):  nil,
		strings.Repeat("x", 129):  ErrPasswordTooLong,
		"        ":                nil, // spaces are allowed: no composition rules
		"password with spaces ok": nil,
	}
	for password, want := range tests {
		if got := validatePassword(password); got != want {
			t.Errorf("validatePassword(%q) = %v, want %v", password, got, want)
		}
	}
}

func TestLoginLimiter(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter()
	limiter.now = func() time.Time { return now }

	for range maxFailedLogins {
		if !limiter.allow("a@x.com") {
			t.Fatal("blocked before the limit was reached")
		}
		limiter.fail("a@x.com")
	}
	if limiter.allow("a@x.com") {
		t.Error("allowed after the limit was reached")
	}
	if !limiter.allow("b@x.com") {
		t.Error("another address was blocked too")
	}

	now = now.Add(failedLoginWindow + time.Second)
	if !limiter.allow("a@x.com") {
		t.Error("still blocked after the window passed")
	}

	limiter.fail("a@x.com")
	limiter.reset("a@x.com") // a successful login clears the count
	for range maxFailedLogins - 1 {
		limiter.fail("a@x.com")
	}
	if !limiter.allow("a@x.com") {
		t.Error("blocked although the successful login had reset the count")
	}
}
