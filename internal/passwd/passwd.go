// Package passwd provides minimal, dependency-free password hashing helpers for
// the Sales dashboard accounts.
//
// NOTE: it uses salted SHA-256, which is adequate for this internal demo. For a
// production system, replace Hash/Verify with a memory-hard KDF such as bcrypt
// or argon2 (golang.org/x/crypto) — the call sites stay the same.
package passwd

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// NewSalt returns a random 16-byte salt as a hex string.
func NewSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// Hash returns the salted SHA-256 hex digest of the password.
func Hash(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

// Verify reports whether password matches the stored salt+hash (constant time).
func Verify(password, salt, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(Hash(password, salt)), []byte(hash)) == 1
}
