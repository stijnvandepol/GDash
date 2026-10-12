// Package totp computes and checks time-based one-time codes (RFC 6238)
// as authenticator apps make them: HMAC-SHA-1, 6 digits, a new code every
// 30 seconds. Those are the only parameters every app supports, so they
// are fixed rather than configurable.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"time"
)

const (
	// Period is how long one code is valid, in seconds.
	Period = 30
	// Digits is the length of a code.
	Digits = 6
	// SecretBytes is the size of a secret: the 160 bits RFC 4226 recommends
	// for HMAC-SHA-1.
	SecretBytes = 20
	// skew is how many steps before and after the current one still count,
	// for a phone whose clock is a little off or a code typed at the end of
	// its 30 seconds.
	skew = 1
)

// encoding is how apps expect a secret to be typed: base32 without padding.
var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a random secret.
func NewSecret() ([]byte, error) {
	secret := make([]byte, SecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate two-step secret: %w", err)
	}
	return secret, nil
}

// EncodeSecret returns secret as the base32 text a person types into an app.
func EncodeSecret(secret []byte) string { return encoding.EncodeToString(secret) }

// URI returns the otpauth:// address an app reads from a QR code, in the
// Key URI Format that Google Authenticator defined and other apps follow.
func URI(issuer, account string, secret []byte) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	query := url.Values{}
	query.Set("secret", EncodeSecret(secret))
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprint(Digits))
	query.Set("period", fmt.Sprint(Period))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// Step returns the time step that t falls in.
func Step(t time.Time) int64 { return t.Unix() / Period }

// Code returns the code of secret for one time step (RFC 4226, section 5.3).
func Code(secret []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, value%1_000_000)
}

// Match returns the time step whose code is code, among the step of now and
// the one before and after it, taking only steps after usedStep: a code
// that was used once is refused, even within its 30 seconds. ok is false
// when no step matches.
func Match(secret []byte, code string, now time.Time, usedStep int64) (step int64, ok bool) {
	if len(code) != Digits {
		return 0, false
	}
	current := Step(now)
	// Newest first: if two steps ever had the same code, the later one is
	// recorded, which refuses the most replays.
	for s := current + skew; s >= current-skew; s-- {
		if s <= usedStep {
			break
		}
		if subtle.ConstantTimeCompare([]byte(Code(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}
