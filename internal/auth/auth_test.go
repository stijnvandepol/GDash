package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

const testToken = "setup-token-for-tests"

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func newTestService(t *testing.T) (*Service, *testClock) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "daycast.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s, err := NewService(st, testToken)
	if err != nil {
		t.Fatal(err)
	}
	clock := &testClock{t: time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)}
	s.now = clock.now
	s.limiter = newLimiter(clock.now)
	return s, clock
}

func setUp(t *testing.T, s *Service) string {
	t.Helper()
	token, _, err := s.Setup(context.Background(), testToken, Credentials{"stijn", "correct horse battery"}, "192.168.1.109")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return token
}

func TestSetupCreatesAccountAndSession(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	if needed, err := s.SetupNeeded(ctx); err != nil || !needed {
		t.Fatalf("SetupNeeded = %v, %v; want true", needed, err)
	}
	token := setUp(t, s)
	u, err := s.Authenticate(ctx, token)
	if err != nil || u.Username != "stijn" {
		t.Fatalf("Authenticate = %+v, %v", u, err)
	}
	if needed, _ := s.SetupNeeded(ctx); needed {
		t.Fatal("SetupNeeded still true after setup")
	}
}

func TestSetupOnlyOnce(t *testing.T) {
	s, _ := newTestService(t)
	setUp(t, s)
	_, _, err := s.Setup(context.Background(), testToken, Credentials{"attacker", "another long password"}, "192.168.1.66")
	if !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second Setup: err = %v, want ErrSetupDone", err)
	}
}

func TestSetupRejectsWrongTokenAndRateLimits(t *testing.T) {
	s, _ := newTestService(t)
	for range maxFailures {
		_, _, err := s.Setup(context.Background(), "guess", Credentials{"stijn", "correct horse battery"}, "192.168.1.66")
		if !errors.Is(err, ErrInvalidSetupToken) {
			t.Fatalf("Setup with wrong token: err = %v", err)
		}
	}
	_, _, err := s.Setup(context.Background(), testToken, Credentials{"stijn", "correct horse battery"}, "192.168.1.66")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Setup after %d failures: err = %v, want ErrRateLimited", maxFailures, err)
	}
}

func TestSetupValidatesAccount(t *testing.T) {
	s, _ := newTestService(t)
	_, _, err := s.Setup(context.Background(), testToken, Credentials{"stijn van de pol", "short"}, "192.168.1.109")
	fields := validation.Fields(err)
	if len(fields) != 2 || fields[0].Field != "username" || fields[1].Field != "password" {
		t.Fatalf("Setup error = %v, want username and password field errors", err)
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	setUp(t, s)

	if _, _, err := s.Login(ctx, Credentials{"STIJN", "correct horse battery"}, "192.168.1.109"); err != nil {
		t.Fatalf("Login with other case of username: %v", err)
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", "wrong password!"}, "192.168.1.109"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login with wrong password: err = %v", err)
	}
	if _, _, err := s.Login(ctx, Credentials{"nobody", "correct horse battery"}, "192.168.1.109"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login of unknown user: err = %v", err)
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", strings.Repeat("x", 500)}, "192.168.1.109"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login with oversized password: err = %v", err)
	}
}

func TestLoginRateLimitPerIPAndExpiry(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	setUp(t, s)
	for range maxFailures {
		_, _, _ = s.Login(ctx, Credentials{"stijn", "wrong password!"}, "192.168.1.66")
	}
	if _, _, err := s.Login(ctx, Credentials{"stijn", "correct horse battery"}, "192.168.1.66"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Login after failures: err = %v, want ErrRateLimited", err)
	}
	clock.t = clock.t.Add(failureWindow + time.Second)
	if _, _, err := s.Login(ctx, Credentials{"stijn", "correct horse battery"}, "192.168.1.66"); err != nil {
		t.Fatalf("Login after the window passed: %v", err)
	}
}

func TestSessionExpiresWhenIdle(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	token := setUp(t, s)

	clock.t = clock.t.Add(sessionIdleTimeout - time.Minute)
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatalf("Authenticate just before idle timeout: %v", err)
	}
	// The previous call refreshed last-seen, so the session lives on.
	clock.t = clock.t.Add(sessionIdleTimeout - time.Minute)
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatalf("Authenticate after refresh: %v", err)
	}
	clock.t = clock.t.Add(sessionIdleTimeout + time.Minute)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Authenticate after idle timeout: err = %v, want ErrNoSession", err)
	}
}

func TestSessionHasAbsoluteLimit(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	token := setUp(t, s)
	start := clock.t

	// Daily use keeps the session from going idle, up to the absolute limit.
	for clock.t.Sub(start) < sessionAbsoluteTimeout-24*time.Hour {
		clock.t = clock.t.Add(24 * time.Hour)
		if _, err := s.Authenticate(ctx, token); err != nil {
			t.Fatalf("Authenticate after %v: %v", clock.t.Sub(start), err)
		}
	}
	clock.t = start.Add(sessionAbsoluteTimeout + time.Minute)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Authenticate after absolute timeout: err = %v, want ErrNoSession", err)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	token := setUp(t, s)
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Authenticate after logout: err = %v", err)
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := verifyPassword(hash, "correct horse battery"); err != nil || !ok {
		t.Fatalf("verify correct password = %v, %v", ok, err)
	}
	if ok, _ := verifyPassword(hash, "Correct horse battery"); ok {
		t.Fatal("verify accepted a different password")
	}
	if _, err := verifyPassword("$argon2id$v=19$m=1,t=1,p=1$!!!$abc", "x"); err == nil {
		t.Fatal("verify accepted a malformed hash")
	}
}
