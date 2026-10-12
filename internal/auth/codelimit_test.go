package auth

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/totp"
)

// wrongCode returns a 6-digit code that no accepted step of secret has at
// now, so a test never guesses right by accident.
func wrongCode(secret []byte, now time.Time) string {
	step := totp.Step(now)
	for _, guess := range []string{"000000", "111111", "222222", "333333"} {
		if guess != totp.Code(secret, step-1) && guess != totp.Code(secret, step) && guess != totp.Code(secret, step+1) {
			return guess
		}
	}
	panic("four codes accepted at once")
}

// address returns a distinct client address per i, as a guesser who
// changes address after every try has.
func address(i int) string { return fmt.Sprintf("10.0.%d.%d", i/250, i%250+1) }

func failedCodes(t *testing.T, s *Service) int {
	t.Helper()
	got, err := s.store.TwoStep(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return got.FailedCodes
}

// The waits allow at most 10 wrong codes in any hour, and the stop comes
// only after about 40 hours of guessing as fast as allowed.
func TestCodeWaitSchedule(t *testing.T) {
	if codeWait(freeCodeFailures) != 0 || codeWait(freeCodeFailures+1) != firstCodeWait || codeWait(maxCodeFailures) != maxCodeWait {
		t.Fatalf("codeWait(5, 6, 50) = %v, %v, %v", codeWait(freeCodeFailures), codeWait(freeCodeFailures+1), codeWait(maxCodeFailures))
	}
	var at []time.Duration
	var now time.Duration
	for failures := 0; failures < maxCodeFailures; failures++ {
		now += codeWait(failures)
		at = append(at, now)
	}
	for i := range at {
		inHour := 0
		for _, a := range at[i:] {
			if a-at[i] < time.Hour {
				inHour++
			}
		}
		if inHour > 10 {
			t.Fatalf("%d wrong codes fit in the hour from %v", inHour, at[i])
		}
	}
	if total := at[len(at)-1]; total < 36*time.Hour {
		t.Fatalf("the stop is reached after %v of guessing", total)
	}
}

// Someone who knows the password and changes address after every guess
// gets 6 codes, then waits that grow, then a stop that only a recovery code
// lifts, also after a restart.
func TestWrongCodesWaitAndThenStop(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, recovery := enableTwoStep(t, s, setUp(t, s))
	start := clock.t
	guess := func(i int) error {
		pending := passwordStep(t, s, address(i))
		_, _, _, err := s.FinishLogin(ctx, pending, wrongCode(secret, clock.t), address(i))
		return err
	}
	for i := 1; i <= maxCodeFailures; i++ {
		err := guess(i)
		var wait *CodeWait
		if errors.As(err, &wait) {
			if i <= freeCodeFailures+1 {
				t.Fatalf("wrong code %d had to wait %v", i, wait.Wait)
			}
			if i == freeCodeFailures+2 && wait.Wait != firstCodeWait {
				t.Fatalf("wrong code %d waits %v, want %v", i, wait.Wait, firstCodeWait)
			}
			clock.t = clock.t.Add(wait.Wait)
			err = guess(i)
		} else if i > freeCodeFailures+1 {
			t.Fatalf("wrong code %d did not wait: err = %v", i, err)
		}
		if !errors.Is(err, ErrWrongCode) || errors.Is(err, ErrTwoStepLocked) != (i == maxCodeFailures) {
			t.Fatalf("wrong code %d: err = %v", i, err)
		}
	}
	if took := clock.t.Sub(start); took < 36*time.Hour {
		t.Fatalf("%d wrong codes took %v", maxCodeFailures, took)
	}

	clock.t = clock.t.Add(maxCodeWait)
	pending := passwordStep(t, s, homeIP)
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), homeIP); !errors.Is(err, ErrTwoStepLocked) || errors.Is(err, ErrWrongCode) {
		t.Fatalf("right app code at the stop: err = %v, want ErrTwoStepLocked", err)
	}
	// A restart keeps the stop.
	restarted, err := NewService(s.store, s.box, testToken)
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = clock.now
	pending = passwordStep(t, restarted, homeIP)
	if _, _, _, err := restarted.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), homeIP); !errors.Is(err, ErrTwoStepLocked) {
		t.Fatalf("right app code at the stop after a restart: err = %v, want ErrTwoStepLocked", err)
	}
	if _, _, use, err := restarted.FinishLogin(ctx, pending, recovery[0], homeIP); err != nil || !use.Recovery {
		t.Fatalf("recovery code at the stop = %+v, %v", use, err)
	}
	if n := failedCodes(t, restarted); n != 0 {
		t.Fatalf("%d wrong codes counted after the recovery code", n)
	}
	pending = passwordStep(t, restarted, homeIP)
	if _, _, _, err := restarted.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), homeIP); err != nil {
		t.Fatalf("app code after the recovery code: %v", err)
	}
}

// Recovery codes, 80 bits each, cannot be guessed: they do not wait, and
// wrong ones do not count, so the owner always has a way in.
func TestRecoveryCodesSkipTheWait(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, recovery := enableTwoStep(t, s, setUp(t, s))
	for i := range freeCodeFailures + 1 {
		pending := passwordStep(t, s, address(i))
		if _, _, _, err := s.FinishLogin(ctx, pending, wrongCode(secret, clock.t), address(i)); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code %d: err = %v", i, err)
		}
	}
	pending := passwordStep(t, s, homeIP)
	if _, _, _, err := s.FinishLogin(ctx, pending, "aaaa-bbbb-cccc-dddd", homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("wrong recovery code while app codes wait: err = %v, want ErrWrongCode", err)
	}
	if n := failedCodes(t, s); n != freeCodeFailures+1 {
		t.Fatalf("%d wrong codes counted, want %d: a wrong recovery code does not count", n, freeCodeFailures+1)
	}
	if _, _, use, err := s.FinishLogin(ctx, pending, recovery[0], homeIP); err != nil || !use.Recovery {
		t.Fatalf("recovery code while app codes wait = %+v, %v", use, err)
	}
	if n := failedCodes(t, s); n != 0 {
		t.Fatalf("%d wrong codes counted after a recovery code", n)
	}
}

func TestPendingLoginEndsAfterFiveWrongCodes(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	clock.t = clock.t.Add(totp.Period * time.Second)
	pending := passwordStep(t, s, homeIP)
	for i := 1; i <= maxPendingCodes; i++ {
		_, _, _, err := s.FinishLogin(ctx, pending, wrongCode(secret, clock.t), address(i))
		if !errors.Is(err, ErrWrongCode) || errors.Is(err, ErrLoginExpired) != (i == maxPendingCodes) {
			t.Fatalf("wrong code %d: err = %v", i, err)
		}
	}
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), address(99)); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("right code after %d wrong ones: err = %v, want ErrLoginExpired", maxPendingCodes, err)
	}
	pending = passwordStep(t, s, address(99))
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), address(99)); err != nil {
		t.Fatalf("right code after the password again: %v", err)
	}
}

// Parallel codes cannot pass the limits before one of them counts, and
// every code that was checked is counted.
func TestParallelCodesAreCounted(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	secret, _ := enableTwoStep(t, s, session)
	clock.t = clock.t.Add(totp.Period * time.Second)
	code := wrongCode(secret, clock.t)
	const parallel = 20
	run := func(try func(i int) error) (wrong int) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i := range parallel {
			wg.Go(func() {
				err := try(i)
				if !errors.Is(err, ErrWrongCode) && !errors.Is(err, ErrRateLimited) && !errors.Is(err, ErrLoginExpired) {
					t.Errorf("parallel code %d: err = %v", i, err)
				}
				if errors.Is(err, ErrWrongCode) {
					mu.Lock()
					wrong++
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		return wrong
	}

	pending := passwordStep(t, s, homeIP)
	wrong := run(func(i int) error {
		_, _, _, err := s.FinishLogin(ctx, pending, code, address(i))
		return err
	})
	if wrong < 1 || wrong > maxPendingCodes || failedCodes(t, s) != wrong {
		t.Fatalf("one pending login: %d wrong codes checked, %d counted; want 1 to %d, all counted", wrong, failedCodes(t, s), maxPendingCodes)
	}

	before := failedCodes(t, s)
	wrong = run(func(i int) error {
		_, err := s.DisableTwoStep(ctx, session, testPassword, code, address(100+i))
		return err
	})
	if total := failedCodes(t, s); total != before+wrong || total > freeCodeFailures+1 {
		t.Fatalf("turning off: %d wrong codes checked, %d counted in all; want at most %d in all", wrong, total, freeCodeFailures+1)
	}
}

// Only someone with the right password gets codes checked, so nobody else
// can make the owner wait or stop.
func TestCodeLimitsNeedThePassword(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	enableTwoStep(t, s, setUp(t, s))
	for i := range 20 {
		if _, _, err := s.Login(ctx, Credentials{"stijn", "a wrong password"}, address(i)); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("wrong password %d: err = %v", i, err)
		}
		if _, _, _, err := s.FinishLogin(ctx, fmt.Sprintf("made-up-%d", i), "123456", address(i)); !errors.Is(err, ErrLoginExpired) {
			t.Fatalf("made-up pending login: err = %v", err)
		}
	}
	if n := failedCodes(t, s); n != 0 {
		t.Fatalf("%d wrong codes counted without the password", n)
	}
}

// The first code of a setup goes through the same limits; a new setup,
// which asks for the password again, starts over.
func TestSetupCodesAreLimited(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	enrolment, err := s.StartTwoStep(ctx, session, testPassword, homeIP)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrolment.Secret)
	if err != nil {
		t.Fatal(err)
	}
	code := wrongCode(secret, clock.t)
	for i := range freeCodeFailures + 1 {
		if _, err := s.EnableTwoStep(ctx, session, code, address(i)); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong first code %d: err = %v", i, err)
		}
	}
	var wait *CodeWait
	if _, err := s.EnableTwoStep(ctx, session, code, address(50)); !errors.As(err, &wait) {
		t.Fatalf("first code after %d wrong ones: err = %v, want a *CodeWait", freeCodeFailures+1, err)
	}
	enableTwoStep(t, s, session)
}
