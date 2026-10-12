package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stijnvandepol/gdash/internal/store"
)

// Limits on codes from the app, per account and on top of the login limits
// per address, which someone who knows the password evades by changing
// address. Codes are only checked after the right password, so nobody
// without it can run into these limits. The count is stored with the
// two-step login, so a restart lifts neither a wait nor the stop. Recovery
// codes, 80 bits each, cannot be guessed: they neither wait nor count here,
// so the owner always has a way in, and only the login limits apply to
// them.
const (
	// maxPendingCodes wrong codes end a pending login: the password has to
	// be typed again.
	maxPendingCodes = 5
	// freeCodeFailures wrong codes in a row cost no wait: the slips of one
	// pending login.
	freeCodeFailures = 5
	// firstCodeWait is the wait after the first wrong code beyond the free
	// ones; each next wrong code doubles it, up to maxCodeWait. That allows
	// at most 10 wrong codes in any hour, and one an hour after that.
	firstCodeWait = 2 * time.Minute
	maxCodeWait   = time.Hour
	// maxCodeFailures wrong codes in a row stop codes from the app until a
	// recovery code is used or `gdash reset-password` runs. NIST SP 800-63B
	// section 5.2.2 allows at most 100; with 50, and 3 codes accepted per
	// guess, a guesser has at most a 50 * 3 / 10^6 = 0.015% chance before
	// the owner has to act, and needs about 40 hours to get there.
	maxCodeFailures = 50
	// countTries bounds the rereads when parallel codes race to be counted.
	countTries = 3
)

// ErrTwoStepLocked is returned for a code from the app after
// maxCodeFailures wrong codes in a row. Recovery codes still work, and one
// of them lifts the stop.
var ErrTwoStepLocked = errors.New("too many wrong two-step codes in a row; use a recovery code")

// CodeWait is returned for a code that comes too soon after wrong ones.
// errors.Is(err, ErrRateLimited) holds for it.
type CodeWait struct {
	Wait time.Duration
}

func (e *CodeWait) Error() string {
	return fmt.Sprintf("too many wrong two-step codes, wait %v", e.Wait)
}

// Is makes a CodeWait an ErrRateLimited.
func (e *CodeWait) Is(target error) bool { return target == ErrRateLimited }

// codeWait returns how long the next code waits after the last of failures
// wrong codes in a row.
func codeWait(failures int) time.Duration {
	if failures <= freeCodeFailures {
		return 0
	}
	wait := firstCodeWait
	for range failures - freeCodeFailures - 1 {
		wait *= 2
		if wait >= maxCodeWait {
			return maxCodeWait
		}
	}
	return wait
}

// codeAllowed returns nil when a code from the app may be checked now for
// t, or why not: ErrTwoStepLocked or a *CodeWait.
func codeAllowed(t store.TwoStep, now time.Time) error {
	if t.FailedCodes >= maxCodeFailures {
		return ErrTwoStepLocked
	}
	if wait := t.LastFailedAt.Add(codeWait(t.FailedCodes)).Sub(now); wait > 0 {
		return &CodeWait{Wait: wait}
	}
	return nil
}

// countCode reads the two-step login of userID and counts a code from the
// app for it as wrong before the code is checked; a right code then ends
// the run. state says whether the login is in the state the code is for. It
// returns the login as counted, store.ErrNotFound, the error of state,
// ErrTwoStepLocked, a *CodeWait, or ErrRateLimited when parallel codes keep
// it busy.
func (s *Service) countCode(ctx context.Context, userID int64, state func(store.TwoStep) error) (store.TwoStep, error) {
	for range countTries {
		t, err := s.store.TwoStep(ctx, userID)
		if err != nil {
			return store.TwoStep{}, err
		}
		if err := state(t); err != nil {
			return store.TwoStep{}, err
		}
		now := s.now()
		if err := codeAllowed(t, now); err != nil {
			return store.TwoStep{}, err
		}
		counted, err := s.store.CountCodeAttempt(ctx, userID, t.SecretSealed, t.FailedCodes, now)
		if err != nil {
			return store.TwoStep{}, err
		}
		if counted {
			t.FailedCodes++
			t.LastFailedAt = now
			return t, nil
		}
	}
	return store.TwoStep{}, ErrRateLimited
}

// refused reports whether err turned a code away before it was checked. A
// refused code is no guess, so it counts towards no limit.
func refused(err error) bool {
	return !errors.Is(err, ErrWrongCode) && (errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTwoStepLocked))
}

// codeCheck says what a right code clears, as the password checks do.
type codeCheck bool

const (
	// finishesLogin clears the address's failures too, as Login does.
	finishesLogin codeCheck = true
	// inSession clears only the account's own failures, as Confirm does:
	// the address may have failed for other usernames, and those keep
	// counting.
	inSession codeCheck = false
)

// limited runs check as an attempt of username from clientIP, counted like
// a password attempt: it stays a failure unless check succeeds, or refuses
// the code before checking it.
func (s *Service) limited(username, clientIP string, kind codeCheck, check func() error) error {
	keys, account := attemptKeys(username, clientIP)
	if !s.attempt(keys, account) {
		return ErrRateLimited
	}
	if err := check(); err != nil {
		if refused(err) {
			s.limiter.Release(keys...)
			s.accountLimiter.Release(account)
		}
		return err
	}
	if kind == finishesLogin {
		s.limiter.Reset(keys...)
	} else {
		s.limiter.Reset(keys[1])
		s.limiter.Release(keys[0])
	}
	s.accountLimiter.Reset(account)
	return nil
}
