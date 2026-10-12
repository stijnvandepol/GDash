package auth

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/totp"
)

const (
	testPassword = "correct horse battery"
	homeIP       = "192.168.1.109"
)

// enableTwoStep turns two-step login on for the account of session and
// returns its secret and recovery codes.
func enableTwoStep(t *testing.T, s *Service, session string) ([]byte, []string) {
	t.Helper()
	ctx := context.Background()
	enrolment, err := s.StartTwoStep(ctx, session, testPassword, homeIP)
	if err != nil {
		t.Fatalf("StartTwoStep: %v", err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrolment.Secret)
	if err != nil || len(secret) != totp.SecretBytes {
		t.Fatalf("enrolment secret %q: %v", enrolment.Secret, err)
	}
	if !strings.Contains(enrolment.URI, "secret="+enrolment.Secret) || !strings.HasPrefix(enrolment.QR, "<svg") {
		t.Fatalf("enrolment = %+v", enrolment)
	}
	codes, err := s.EnableTwoStep(ctx, session, totp.Code(secret, totp.Step(s.now())), homeIP)
	if err != nil {
		t.Fatalf("EnableTwoStep: %v", err)
	}
	return secret, codes
}

// passwordStep logs in with the password and returns the pending login.
func passwordStep(t *testing.T, s *Service, ip string) string {
	t.Helper()
	_, _, err := s.Login(context.Background(), Credentials{"stijn", testPassword}, ip)
	var needed *CodeNeeded
	if !errors.As(err, &needed) || needed.Pending == "" {
		t.Fatalf("Login with two-step on: err = %v, want *CodeNeeded", err)
	}
	if strings.Contains(err.Error(), needed.Pending) {
		t.Fatal("the error text holds the pending login")
	}
	return needed.Pending
}

func TestTwoStepLogin(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	// The enrolment used the current step; wait for the next one.
	clock.t = clock.t.Add(totp.Period * time.Second)

	pending := passwordStep(t, s, homeIP)
	if _, _, _, err := s.FinishLogin(ctx, pending, "000000", homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("wrong code: err = %v, want ErrWrongCode", err)
	}
	code := totp.Code(secret, totp.Step(clock.t))
	session, user, use, err := s.FinishLogin(ctx, pending, code[:3]+" "+code[3:], homeIP)
	if err != nil || user.Username != "stijn" || use.Recovery {
		t.Fatalf("FinishLogin = %+v, %+v, %v", user, use, err)
	}
	if _, err := s.Authenticate(ctx, session); err != nil {
		t.Fatalf("session after two-step login: %v", err)
	}
	if _, _, _, err := s.FinishLogin(ctx, pending, code, homeIP); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("pending login used twice: err = %v, want ErrLoginExpired", err)
	}

	// A code works once, also for a new login within its 30 seconds.
	again := passwordStep(t, s, homeIP)
	if _, _, _, err := s.FinishLogin(ctx, again, code, homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("replayed code: err = %v, want ErrWrongCode", err)
	}
	clock.t = clock.t.Add(totp.Period * time.Second)
	if _, _, _, err := s.FinishLogin(ctx, again, totp.Code(secret, totp.Step(clock.t)), homeIP); err != nil {
		t.Fatalf("next code: %v", err)
	}
}

func TestTwoStepCodeWindow(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	clock.t = clock.t.Add(10 * totp.Period * time.Second)
	now := totp.Step(clock.t)
	for _, c := range []struct {
		step int64
		ok   bool
	}{{now - 2, false}, {now + 2, false}, {now - 1, true}, {now, true}, {now + 1, true}} {
		pending := passwordStep(t, s, homeIP)
		_, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, c.step), homeIP)
		if (err == nil) != c.ok {
			t.Fatalf("code of step now%+d: err = %v, want accepted %v", c.step-now, err, c.ok)
		}
	}
}

func TestRecoveryCodesWorkOnce(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	_, codes := enableTwoStep(t, s, setUp(t, s))
	if len(codes) != recoveryCodeCount {
		t.Fatalf("%d recovery codes, want %d", len(codes), recoveryCodeCount)
	}
	pending := passwordStep(t, s, homeIP)
	_, _, use, err := s.FinishLogin(ctx, pending, strings.ToUpper(codes[3]), homeIP)
	if err != nil || !use.Recovery || use.RecoveryLeft != recoveryCodeCount-1 {
		t.Fatalf("recovery code = %+v, %v", use, err)
	}
	pending = passwordStep(t, s, homeIP)
	if _, _, _, err := s.FinishLogin(ctx, pending, codes[3], homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("recovery code used twice: err = %v, want ErrWrongCode", err)
	}
	status, err := s.TwoStep(ctx, User{ID: 1})
	if err != nil || !status.Enabled || status.RecoveryCodesLeft != recoveryCodeCount-1 {
		t.Fatalf("TwoStep status = %+v, %v", status, err)
	}
}

func TestWrongCodesAreRateLimited(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	clock.t = clock.t.Add(totp.Period * time.Second)
	for range maxFailures {
		// The right password every time must not clear the wrong codes.
		pending := passwordStep(t, s, "192.168.1.66")
		if _, _, _, err := s.FinishLogin(ctx, pending, "000000", "192.168.1.66"); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code: err = %v", err)
		}
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", testPassword}, "192.168.1.66"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("password after %d wrong codes: err = %v, want ErrRateLimited", maxFailures, err)
	}
	// Another device in the home still gets in.
	pending := passwordStep(t, s, "192.168.1.20")
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), "192.168.1.20"); err != nil {
		t.Fatalf("login from another address: %v", err)
	}
}

func TestPendingLoginExpires(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	pending := passwordStep(t, s, homeIP)
	clock.t = clock.t.Add(pendingLifetime)
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), homeIP); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("code after %v: err = %v, want ErrLoginExpired", pendingLifetime, err)
	}
	if _, _, _, err := s.FinishLogin(ctx, "made-up", "123456", homeIP); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("unknown pending login: err = %v, want ErrLoginExpired", err)
	}
}

func TestPendingLoginEndsWhenThePasswordChanges(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	secret, _ := enableTwoStep(t, s, session)
	pending := passwordStep(t, s, homeIP)
	if err := s.ChangePassword(ctx, session, testPassword, "a new long password", homeIP); err != nil {
		t.Fatal(err)
	}
	clock.t = clock.t.Add(totp.Period * time.Second)
	if _, _, _, err := s.FinishLogin(ctx, pending, totp.Code(secret, totp.Step(clock.t)), homeIP); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("code after a password change: err = %v, want ErrLoginExpired", err)
	}
}

func TestEnablingTwoStep(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	session := setUp(t, s)
	other, _, err := s.Login(ctx, Credentials{"stijn", testPassword}, "192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTwoStep(ctx, session, "wrong password!", homeIP); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("StartTwoStep with a wrong password: err = %v", err)
	}
	if _, err := s.EnableTwoStep(ctx, session, "123456", homeIP); !errors.Is(err, ErrNoSetup) {
		t.Fatalf("EnableTwoStep without a setup: err = %v, want ErrNoSetup", err)
	}
	if _, err := s.StartTwoStep(ctx, session, testPassword, homeIP); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableTwoStep(ctx, session, "000000", homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("EnableTwoStep with a wrong code: err = %v, want ErrWrongCode", err)
	}
	if on, _ := s.twoStepOn(ctx, 1); on {
		t.Fatal("two-step login is on before its first code")
	}
	// A login before the first code still needs only the password.
	if _, _, err := s.Login(ctx, Credentials{"stijn", testPassword}, homeIP); err != nil {
		t.Fatalf("login during setup: %v", err)
	}

	enableTwoStep(t, s, session)
	if _, err := s.Authenticate(ctx, other); !errors.Is(err, ErrNoSession) {
		t.Fatalf("other session after turning two-step on: err = %v, want ErrNoSession", err)
	}
	if _, err := s.Authenticate(ctx, session); err != nil {
		t.Fatalf("own session after turning two-step on: %v", err)
	}
	if _, err := s.StartTwoStep(ctx, session, testPassword, homeIP); !errors.Is(err, ErrTwoStepOn) {
		t.Fatalf("StartTwoStep while on: err = %v, want ErrTwoStepOn", err)
	}
}

func TestTwoStepSecretIsSealed(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	secret, _ := enableTwoStep(t, s, setUp(t, s))
	stored, err := s.store.TwoStep(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	encoded := totp.EncodeSecret(secret)
	if strings.Contains(stored.SecretSealed, encoded) || strings.Contains(stored.SecretSealed, string(secret)) {
		t.Fatal("the secret is stored readable")
	}
	opened, err := s.box.Open(stored.SecretSealed)
	if err != nil || opened != string(secret) {
		t.Fatalf("sealed secret does not open to the secret: %v", err)
	}
}

func TestDisablingTwoStep(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	session := setUp(t, s)
	secret, _ := enableTwoStep(t, s, session)
	clock.t = clock.t.Add(totp.Period * time.Second)
	code := totp.Code(secret, totp.Step(clock.t))
	if _, err := s.DisableTwoStep(ctx, session, "wrong password!", code, homeIP); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("DisableTwoStep with a wrong password: err = %v", err)
	}
	if _, err := s.DisableTwoStep(ctx, session, testPassword, "000000", homeIP); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("DisableTwoStep with a wrong code: err = %v", err)
	}
	if _, err := s.DisableTwoStep(ctx, session, testPassword, code, homeIP); err != nil {
		t.Fatalf("DisableTwoStep: %v", err)
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", testPassword}, homeIP); err != nil {
		t.Fatalf("login after turning two-step off: %v", err)
	}
	if _, err := s.DisableTwoStep(ctx, session, testPassword, code, homeIP); !errors.Is(err, ErrTwoStepOff) {
		t.Fatalf("DisableTwoStep while off: err = %v, want ErrTwoStepOff", err)
	}
}

func TestResetPasswordTurnsTwoStepOff(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	enableTwoStep(t, s, setUp(t, s))
	password, wasOn, err := s.ResetPassword(ctx, "stijn")
	if err != nil || !wasOn {
		t.Fatalf("ResetPassword: two-step was on %v, err %v", wasOn, err)
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", password}, homeIP); err != nil {
		t.Fatalf("login after the reset: %v", err)
	}
}
