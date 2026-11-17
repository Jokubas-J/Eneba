package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// newSessionToken returns a new random session token (what the cookie holds) and
// its SHA-256 hash (what the database stores).
//
// 32 random bytes from the operating system's secure generator: impossible to guess
// (2^256 values) and not derived from anything about the user.
//
// The database keeps only the hash, so someone who reads the sessions table (a
// leaked backup, a SQL injection elsewhere) still cannot use the sessions in it.
// SHA-256 is enough here, unlike for passwords: the token is already random and
// long, so there is nothing to guess and no need for a slow hash.
func newSessionToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
