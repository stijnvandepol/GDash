// Package secret encrypts small secrets, such as the iCal addresses of
// calendars, before they are stored. The key lives in its own file next to
// the database, so a copied database or backup alone does not reveal them.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

const keySize = 32 // AES-256

// ErrUnreadable means a sealed value cannot be opened: it was sealed with
// another key, or it was changed.
var ErrUnreadable = errors.New("secret cannot be decrypted with this key")

// Box seals and opens secrets with one key, using AES-256-GCM.
type Box struct {
	aead cipher.AEAD
	// macKey is derived from the key, so fingerprints and encryption never
	// use the same key material.
	macKey []byte
}

// fingerprintLabel separates the fingerprint key from the encryption key.
const fingerprintLabel = "gdash fingerprint v1"

// fingerprintBytes is long enough that two configurations never collide in
// practice, and short enough to read in a cache file.
const fingerprintBytes = 16

// LoadOrCreateKey reads the key at path, or creates one there readable only
// by the current user.
func LoadOrCreateKey(path string) (*Box, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		key = make([]byte, keySize)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("create secret key: %w", err)
		}
		// O_EXCL: never overwrite a key another process just wrote.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create secret key: %w", err)
		}
		defer f.Close()
		if _, err := f.Write(key); err != nil {
			return nil, fmt.Errorf("write secret key: %w", err)
		}
		if err := f.Sync(); err != nil {
			return nil, fmt.Errorf("write secret key: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("read secret key: %w", err)
	}
	return NewBox(key)
}

// NewBox uses key, which must be 32 bytes.
func NewBox(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("secret key must be %d bytes, found %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(fingerprintLabel))
	return &Box{aead: aead, macKey: mac.Sum(nil)}, nil
}

// Fingerprint returns a keyed hash of data: equal for equal data on this
// installation, but without the key file it reveals nothing about data,
// even when data is guessable, such as a calendar address.
func (b *Box) Fingerprint(data []byte) string {
	mac := hmac.New(sha256.New, b.macKey)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)[:fingerprintBytes])
}

// Seal encrypts plaintext with a fresh random nonce and returns it as text.
func (b *Box) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("create nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

// Open decrypts a value from Seal. It returns ErrUnreadable for a value
// sealed with another key or changed after sealing.
func (b *Box) Open(sealed string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < b.aead.NonceSize() {
		return "", ErrUnreadable
	}
	nonce, ciphertext := raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrUnreadable
	}
	return string(plaintext), nil
}
