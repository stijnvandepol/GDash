// Package token creates bearer tokens and the hashes Nookboard stores instead
// of them, so a leaked database cannot be used to log in or show a display.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// New returns a random 256-bit token, URL-safe.
func New() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Hash returns the hex SHA-256 of a token. Tokens are random and long, so
// a fast hash is enough; a password hash is only needed for guessable
// secrets.
func Hash(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}
