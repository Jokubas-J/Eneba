// Package auth handles accounts and sign-in: password hashing, sessions, and the
// rules for registering and logging in. The HTTP handlers live in package httpapi.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	// The Go team's crypto extensions (maintained alongside the standard library).
	// The standard library has no Argon2, the password hash OWASP recommends first.
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters: OWASP's first recommended setting (19 MiB of memory, 2
// passes, 1 thread). Memory is what makes guessing expensive on a GPU: every guess
// needs its own 19 MiB. On our server one hash takes roughly 30-50 ms, which a
// person signing in does not notice and an attacker trying millions of passwords does.
const (
	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

var errMalformedHash = errors.New("malformed password hash")

// HashPassword returns the password's Argon2id hash in the common "PHC string"
// format, which stores the algorithm, parameters and salt together with the hash:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
//
// Keeping the parameters in the string means they can be raised later without
// breaking the hashes of existing users: each hash is checked with its own.
func HashPassword(password string) (string, error) {
	// A random salt per password: two users with the same password get different
	// hashes, and precomputed tables of hashes are useless.
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, encode(salt), encode(hash)), nil
}

// CheckPassword reports whether password matches an encoded hash from HashPassword.
func CheckPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errMalformedHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errMalformedHash
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	// ConstantTimeCompare takes the same time whether the first byte or the last one
	// differs, so the response time reveals nothing about how close a guess was.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
