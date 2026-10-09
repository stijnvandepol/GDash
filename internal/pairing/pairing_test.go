package pairing

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/nookboard/internal/displays"
	"github.com/stijnvandepol/nookboard/internal/ratelimit"
	"github.com/stijnvandepol/nookboard/internal/store"
)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func newTestService(t *testing.T) (*Service, *testClock) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "nookboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clock := &testClock{t: time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)}
	for _, id := range []string{"kitchen", "office"} {
		if err := st.CreateDisplay(ctx, displays.Display{ID: id, Name: id, CreatedAt: clock.t}); err != nil {
			t.Fatal(err)
		}
	}
	s := New(st)
	s.now = clock.now
	s.limiter = ratelimit.New(clock.now, maxCodeRequests, requestWindow)
	return s, clock
}

func TestCodePairingFlow(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)

	req, err := s.Begin(ctx, "kitchen", "192.168.1.160")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if len(req.Code) != 9 || req.Code[4] != '-' {
		t.Fatalf("code %q, want XXXX-XXXX", req.Code)
	}
	if _, _, err := s.Claim(req.PollToken); !errors.Is(err, ErrPending) {
		t.Fatalf("Claim before approval: err = %v, want ErrPending", err)
	}

	// The admin types the code without the dash and in lower case.
	if err := s.Approve(ctx, "kitchen", strings.ToLower(strings.ReplaceAll(req.Code, "-", ""))); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	token, displayID, err := s.Claim(req.PollToken)
	if err != nil || displayID != "kitchen" || token == "" {
		t.Fatalf("Claim = %q, %q, %v", token, displayID, err)
	}
	if got, err := s.Authenticate(ctx, token); err != nil || got != "kitchen" {
		t.Fatalf("Authenticate = %q, %v", got, err)
	}
	if _, _, err := s.Claim(req.PollToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second Claim: err = %v, want ErrInvalidToken", err)
	}
}

func TestApproveRejectsWrongDisplayAndExpiredCode(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	req, err := s.Begin(ctx, "kitchen", "192.168.1.160")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Approve(ctx, "office", req.Code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Approve for another display: err = %v, want ErrInvalidCode", err)
	}
	if err := s.Approve(ctx, "kitchen", "AAAA-AAAA"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Approve with unknown code: err = %v, want ErrInvalidCode", err)
	}
	clock.t = clock.t.Add(codeTTL + time.Second)
	if err := s.Approve(ctx, "kitchen", req.Code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Approve after expiry: err = %v, want ErrInvalidCode", err)
	}
}

func TestBeginRejectsUnknownDisplayAndLimitsRequests(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	if _, err := s.Begin(ctx, "garage", "10.0.0.9"); !errors.Is(err, ErrUnknownDisplay) {
		t.Fatalf("Begin for unknown display: err = %v, want ErrUnknownDisplay", err)
	}
	for range maxCodeRequests - 1 {
		if _, err := s.Begin(ctx, "kitchen", "10.0.0.9"); err != nil {
			t.Fatalf("Begin: %v", err)
		}
	}
	if _, err := s.Begin(ctx, "kitchen", "10.0.0.9"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Begin over the limit: err = %v, want ErrRateLimited", err)
	}
	if _, err := s.Begin(ctx, "kitchen", "10.0.0.10"); err != nil {
		t.Fatalf("Begin from another address: %v", err)
	}
}

func TestLaunchTokenIsSingleUseAndExpires(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)

	launchToken, err := s.NewLaunchToken("kitchen")
	if err != nil {
		t.Fatal(err)
	}
	displayToken, displayID, err := s.Exchange(ctx, launchToken)
	if err != nil || displayID != "kitchen" {
		t.Fatalf("Exchange = %q, %v", displayID, err)
	}
	if got, err := s.Authenticate(ctx, displayToken); err != nil || got != "kitchen" {
		t.Fatalf("Authenticate = %q, %v", got, err)
	}
	if _, _, err := s.Exchange(ctx, launchToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second Exchange: err = %v, want ErrInvalidToken", err)
	}

	expiring, err := s.NewLaunchToken("kitchen")
	if err != nil {
		t.Fatal(err)
	}
	clock.t = clock.t.Add(launchTTL + time.Second)
	if _, _, err := s.Exchange(ctx, expiring); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Exchange after expiry: err = %v, want ErrInvalidToken", err)
	}
}

func TestEveryLaunchReplacesTheCastToken(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	var tokens []string
	for range 3 {
		launchToken, err := s.NewLaunchToken("kitchen")
		if err != nil {
			t.Fatal(err)
		}
		displayToken, _, err := s.Exchange(ctx, launchToken)
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, displayToken)
	}
	if _, err := s.Authenticate(ctx, tokens[0]); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("first cast token still valid after relaunches: err = %v", err)
	}
	if _, err := s.Authenticate(ctx, tokens[2]); err != nil {
		t.Fatalf("latest cast token: %v", err)
	}
}

func TestUnpairRevokesScreens(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	req, err := s.Begin(ctx, "kitchen", "192.168.1.160")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Approve(ctx, "kitchen", req.Code); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.Claim(req.PollToken)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.Unpair(ctx, "kitchen"); err != nil || n != 1 {
		t.Fatalf("Unpair = %d, %v", n, err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate after Unpair: err = %v, want ErrInvalidToken", err)
	}
}

func TestCodesUseOnlyUnambiguousCharacters(t *testing.T) {
	for range 200 {
		code, err := randomCode()
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(code, "01ILO") {
			t.Fatalf("code %q contains a character that looks like another", code)
		}
	}
}
