package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
	"github.com/stijnvandepol/gdash/internal/totp"
)

func (s *Service) twoStepOn(ctx context.Context, userID int64) (bool, error) {
	t, err := s.store.TwoStep(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return t.Enabled, err
}

// waitsForCode returns ErrTwoStepOn unless t is a setup that waits for its
// first code.
func waitsForCode(t store.TwoStep) error {
	if t.Enabled {
		return ErrTwoStepOn
	}
	return nil
}

// isOn returns ErrTwoStepOff unless two-step login t is on.
func isOn(t store.TwoStep) error {
	if !t.Enabled {
		return ErrTwoStepOff
	}
	return nil
}

// verifyCode checks a code of an account with two-step login on: a code
// from the app, of a time step not used before, or one of the recovery
// codes, which is then used up. Codes from the app count towards the limits
// in codelimit.go. It returns ErrWrongCode, ErrWrongCode joined with
// ErrTwoStepLocked for the wrong code that reaches the stop,
// ErrTwoStepLocked, a *CodeWait, ErrRateLimited or ErrTwoStepOff.
func (s *Service) verifyCode(ctx context.Context, userID int64, code string) (CodeUse, error) {
	code = normalizeCode(code)
	if len(code) == recoveryCodeLength {
		return s.useRecoveryCode(ctx, userID, code)
	}
	t, err := s.countCode(ctx, userID, isOn)
	if errors.Is(err, store.ErrNotFound) {
		return CodeUse{}, ErrTwoStepOff
	}
	if err != nil {
		return CodeUse{}, err
	}
	err = s.checkAppCode(ctx, t, userID, code)
	if errors.Is(err, ErrWrongCode) && t.FailedCodes == maxCodeFailures {
		return CodeUse{}, errors.Join(err, ErrTwoStepLocked)
	}
	return CodeUse{}, err
}

// useRecoveryCode uses up recovery code, normalised, of userID. It returns
// ErrWrongCode or ErrTwoStepOff.
func (s *Service) useRecoveryCode(ctx context.Context, userID int64, code string) (CodeUse, error) {
	on, err := s.twoStepOn(ctx, userID)
	if err != nil {
		return CodeUse{}, err
	}
	if !on {
		return CodeUse{}, ErrTwoStepOff
	}
	left, ok, err := s.store.UseRecoveryCode(ctx, userID, token.Hash(code))
	if err != nil {
		return CodeUse{}, err
	}
	if !ok {
		return CodeUse{}, ErrWrongCode
	}
	return CodeUse{Recovery: true, RecoveryLeft: left}, nil
}

// checkAppCode checks code, normalised, against two-step login t of
// userID. It returns ErrWrongCode for a code that is not right now or was
// used before.
func (s *Service) checkAppCode(ctx context.Context, t store.TwoStep, userID int64, code string) error {
	secret, err := s.box.Open(t.SecretSealed)
	if err != nil {
		return fmt.Errorf("open two-step secret of user %d: %w", userID, err)
	}
	step, ok := totp.Match([]byte(secret), code, s.now(), t.LastStep)
	if !ok {
		return ErrWrongCode
	}
	// A parallel request with the same code may have used the step since
	// it was read; only one of them wins.
	used, err := s.store.UseTwoStepStep(ctx, userID, step)
	if err != nil {
		return err
	}
	if !used {
		return ErrWrongCode
	}
	return nil
}

// normalizeCode accepts a code as people type or paste it: with spaces or
// dashes, in either case. Any white space counts as a space: a code copied
// from a password manager or a document may hold tabs or non-breaking
// spaces.
func normalizeCode(code string) string {
	code = strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, code)
	return strings.ToLower(code)
}
