package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/totp"
)

// With two-step login on, the owner's own password confirmations, such as
// for a backup, must not wipe wrong codes from the login limits; nor can
// someone with a session and the password wipe them that way.
func TestConfirmKeepsWrongCodes(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	secret, _ := enableTwoStep(t, s, session)
	clock.t = clock.t.Add(totp.Period * time.Second)
	// A small account limit shows the effect before the code limits act.
	const accountLimit = 3
	s.accountLimiter = ratelimit.New(clock.now, accountLimit, failureWindow)
	fail := func(i int) {
		t.Helper()
		pending := passwordStep(t, s, address(i))
		if _, _, _, err := s.FinishLogin(ctx, pending, wrongCode(secret, clock.t), address(i)); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code from %s: err = %v", address(i), err)
		}
	}
	fail(1)
	fail(2)
	if _, err := s.Confirm(ctx, session, testPassword, address(3)); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	fail(4)
	if _, _, err := s.Login(ctx, Credentials{"stijn", testPassword}, address(5)); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("login after %d wrong codes and a confirmation: err = %v, want ErrRateLimited", accountLimit, err)
	}
}

// Without two-step login, a right password confirms as a login does and
// clears the account's failures.
func TestConfirmClearsWrongPasswordsWithoutTwoStep(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	s.accountLimiter = ratelimit.New(clock.now, 3, failureWindow)
	for i := range 2 {
		if _, err := s.Confirm(ctx, session, "a wrong password", address(i)); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("wrong password: err = %v", err)
		}
	}
	if _, err := s.Confirm(ctx, session, testPassword, address(3)); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	for i := range 2 {
		if _, err := s.Confirm(ctx, session, "a wrong password", address(10+i)); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("wrong password after a right one: err = %v", err)
		}
	}
}
