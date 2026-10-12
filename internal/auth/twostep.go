package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/qr"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
	"github.com/stijnvandepol/gdash/internal/totp"
)

const (
	// issuer names GDash in the authenticator app.
	issuer = "GDash"
	// recoveryCodeCount codes cover years of a lost phone now and then.
	recoveryCodeCount = 10
	// recoveryCodeBytes is 80 bits: too many to guess online, and enough
	// that the plain SHA-256 stored for each cannot be reversed offline.
	recoveryCodeBytes = 10
	// recoveryCodeLength is the length of a recovery code without dashes.
	recoveryCodeLength = recoveryCodeBytes * 8 / 5
	// recoveryCodeGroup splits a code into groups of four for reading.
	recoveryCodeGroup = 4
)

var (
	// ErrWrongCode is returned for a code that is not right now, was used
	// before, or is not one of the recovery codes.
	ErrWrongCode = errors.New("the code is not correct")
	// ErrLoginExpired is returned when the code of a login comes too late,
	// or for a login GDash does not know: the password is asked again.
	ErrLoginExpired = errors.New("the login has expired")
	// ErrTwoStepOn is returned when two-step login is set up while it is on.
	ErrTwoStepOn = errors.New("two-step login is already on")
	// ErrTwoStepOff is returned when two-step login is turned off while it
	// is off.
	ErrTwoStepOff = errors.New("two-step login is off")
	// ErrNoSetup is returned for a first code without a setup waiting.
	ErrNoSetup = errors.New("two-step login has not been set up")
)

var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// Enrolment is what an authenticator app needs: the secret as text to type,
// as an otpauth:// address, and as a QR code (SVG) of that address. It is
// shown once; the server never returns the secret again.
type Enrolment struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QR     string `json:"qr"`
}

// CodeUse says how a second step was proven.
type CodeUse struct {
	// Recovery is true when a recovery code was used, with RecoveryLeft
	// codes remaining.
	Recovery     bool
	RecoveryLeft int
}

// TwoStepStatus is the two-step login of an account, for its settings.
type TwoStepStatus struct {
	Enabled           bool `json:"enabled"`
	RecoveryCodesLeft int  `json:"recoveryCodesLeft"`
}

// TwoStep returns the two-step status of user.
func (s *Service) TwoStep(ctx context.Context, user User) (TwoStepStatus, error) {
	on, err := s.twoStepOn(ctx, user.ID)
	if err != nil || !on {
		return TwoStepStatus{}, err
	}
	left, err := s.store.RecoveryCodesLeft(ctx, user.ID)
	return TwoStepStatus{Enabled: true, RecoveryCodesLeft: left}, err
}

// StartTwoStep makes a new secret for the user of session after checking the
// password again, so a session left open in a browser cannot tie the
// account to a stranger's phone. Two-step login stays off until
// EnableTwoStep gets a code of the secret. It returns ErrTwoStepOn, or the
// errors of Confirm.
func (s *Service) StartTwoStep(ctx context.Context, session, password, clientIP string) (Enrolment, error) {
	user, err := s.Confirm(ctx, session, password, clientIP)
	if err != nil {
		return Enrolment{}, err
	}
	secret, err := totp.NewSecret()
	if err != nil {
		return Enrolment{}, err
	}
	sealed, err := s.box.Seal(string(secret))
	if err != nil {
		return Enrolment{}, err
	}
	if err := s.store.StartTwoStep(ctx, user.ID, sealed, s.now()); errors.Is(err, store.ErrConflict) {
		return Enrolment{}, ErrTwoStepOn
	} else if err != nil {
		return Enrolment{}, err
	}
	uri := totp.URI(issuer, user.Username, secret)
	code, err := qr.Encode([]byte(uri))
	if err != nil {
		return Enrolment{}, fmt.Errorf("draw the two-step QR code: %w", err)
	}
	return Enrolment{Secret: totp.EncodeSecret(secret), URI: uri, QR: code.SVG()}, nil
}

// EnableTwoStep turns two-step login on for the user of session once code
// is a current code of the secret StartTwoStep made, and ends the user's
// other sessions, which only had the password. It returns the recovery
// codes, which exist nowhere else afterwards. Wrong codes count towards the
// login limits. It returns ErrNoSetup, ErrTwoStepOn, ErrWrongCode,
// ErrRateLimited or ErrNoSession.
func (s *Service) EnableTwoStep(ctx context.Context, session, code, clientIP string) ([]string, error) {
	user, err := s.Authenticate(ctx, session)
	if err != nil {
		return nil, err
	}
	setup, err := s.store.TwoStep(ctx, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNoSetup
	}
	if err != nil {
		return nil, err
	}
	if setup.Enabled {
		return nil, ErrTwoStepOn
	}
	var step int64
	err = s.limited(user.Username, clientIP, func() error {
		secret, err := s.box.Open(setup.SecretSealed)
		if err != nil {
			return fmt.Errorf("open two-step secret of user %d: %w", user.ID, err)
		}
		var ok bool
		if step, ok = totp.Match([]byte(secret), normalizeCode(code), s.now(), 0); !ok {
			return ErrWrongCode
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := s.store.EnableTwoStep(ctx, user.ID, step, hashes, token.Hash(session)); errors.Is(err, store.ErrNotFound) {
		// Another request turned it on, or off, in between.
		return nil, ErrNoSetup
	} else if err != nil {
		return nil, err
	}
	return codes, nil
}

// DisableTwoStep turns two-step login off for the user of session. It
// needs the password and a code, so neither a session left open nor a
// password alone is enough. It returns ErrWrongPassword, ErrWrongCode,
// ErrTwoStepOff, ErrRateLimited or ErrNoSession.
func (s *Service) DisableTwoStep(ctx context.Context, session, password, code, clientIP string) (CodeUse, error) {
	user, err := s.Confirm(ctx, session, password, clientIP)
	if err != nil {
		return CodeUse{}, err
	}
	var use CodeUse
	err = s.limited(user.Username, clientIP, func() error {
		use, err = s.verifyCode(ctx, user.ID, code)
		return err
	})
	if err != nil {
		return CodeUse{}, err
	}
	if _, err := s.store.DeleteTwoStep(ctx, user.ID); err != nil {
		return CodeUse{}, err
	}
	return use, nil
}

func (s *Service) twoStepOn(ctx context.Context, userID int64) (bool, error) {
	t, err := s.store.TwoStep(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return t.Enabled, err
}

// verifyCode checks a code of an account with two-step login on: a code
// from the app, of a time step not used before, or one of the recovery
// codes, which is then used up. It returns ErrWrongCode or ErrTwoStepOff.
func (s *Service) verifyCode(ctx context.Context, userID int64, code string) (CodeUse, error) {
	t, err := s.store.TwoStep(ctx, userID)
	if errors.Is(err, store.ErrNotFound) || err == nil && !t.Enabled {
		return CodeUse{}, ErrTwoStepOff
	}
	if err != nil {
		return CodeUse{}, err
	}
	code = normalizeCode(code)
	if len(code) == recoveryCodeLength {
		left, ok, err := s.store.UseRecoveryCode(ctx, userID, token.Hash(code))
		if err != nil {
			return CodeUse{}, err
		}
		if !ok {
			return CodeUse{}, ErrWrongCode
		}
		return CodeUse{Recovery: true, RecoveryLeft: left}, nil
	}
	secret, err := s.box.Open(t.SecretSealed)
	if err != nil {
		return CodeUse{}, fmt.Errorf("open two-step secret of user %d: %w", userID, err)
	}
	step, ok := totp.Match([]byte(secret), code, s.now(), t.LastStep)
	if !ok {
		return CodeUse{}, ErrWrongCode
	}
	// A parallel request with the same code may have used the step since
	// it was read; only one of them wins.
	used, err := s.store.UseTwoStepStep(ctx, userID, step)
	if err != nil {
		return CodeUse{}, err
	}
	if !used {
		return CodeUse{}, ErrWrongCode
	}
	return CodeUse{}, nil
}

// limited runs check as an attempt of username from clientIP, counted like
// a password attempt: it stays a failure unless check succeeds. On success
// only the account's own failures are cleared, as Confirm does: the address
// may have failed for other usernames, and those keep counting.
func (s *Service) limited(username, clientIP string, check func() error) error {
	keys, account := attemptKeys(username, clientIP)
	if !s.attempt(keys, account) {
		return ErrRateLimited
	}
	if err := check(); err != nil {
		return err
	}
	s.limiter.Reset(keys[1])
	s.limiter.Release(keys[0])
	s.accountLimiter.Reset(account)
	return nil
}

// normalizeCode accepts a code as people type or paste it: with spaces or
// dashes, in either case.
func normalizeCode(code string) string {
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	return strings.ToLower(code)
}

// newRecoveryCodes returns recovery codes as shown ("abcd-efgh-ijkl-mnop")
// and the hashes of their normalised form, which is all that is stored.
func newRecoveryCodes() (codes, hashes []string, err error) {
	for range recoveryCodeCount {
		raw := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		code := strings.ToLower(recoveryEncoding.EncodeToString(raw))
		var groups []string
		for i := 0; i < len(code); i += recoveryCodeGroup {
			groups = append(groups, code[i:i+recoveryCodeGroup])
		}
		codes = append(codes, strings.Join(groups, "-"))
		hashes = append(hashes, token.Hash(code))
	}
	return codes, hashes, nil
}
