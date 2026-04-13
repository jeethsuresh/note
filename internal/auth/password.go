package auth

import (
	"crypto/sha256"
	"crypto/subtle"
)

// SecurePasswordEqual compares two UTF-8 strings in time independent of their
// contents (but not their lengths) by comparing SHA-256 digests with
// constant-time equality. Length is still observable; callers should use a
// fixed-width derived secret for stronger guarantees.
func SecurePasswordEqual(expected, provided string) bool {
	eh := sha256.Sum256([]byte(expected))
	ph := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(eh[:], ph[:]) == 1
}
