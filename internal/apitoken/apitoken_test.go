package apitoken

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func newTestService(t *testing.T) (*Service, *store.Store, *testClock) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "gdash.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clock := &testClock{t: time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)}
	s := New(st)
	s.now = clock.now
	s.failures = ratelimit.New(clock.now, maxFailures, failureWindow)
	return s, st, clock
}

func TestCreateShowsSecretOnceAndStoresOnlyItsHash(t *testing.T) {
	ctx := context.Background()
	s, st, _ := newTestService(t)
	made, err := s.Create(ctx, "  Home Assistant ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if made.Name != "Home Assistant" || !strings.HasPrefix(made.Token, Prefix) || len(made.Token) < 40 {
		t.Fatalf("Create = %+v", made)
	}
	listed, err := st.APITokens(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatalf("APITokens = %+v, %v", listed, err)
	}
	if _, err := st.APITokenByHash(ctx, made.Token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the secret itself is stored: err = %v", err)
	}
}

func TestCreateValidatesTheName(t *testing.T) {
	s, _, _ := newTestService(t)
	for name, input := range map[string]string{
		"empty":           "",
		"blank":           "   ",
		"too long":        strings.Repeat("a", maxNameLength+1),
		"control chars":   "washing\nmachine",
		"escape sequence": "x\x1b[31m",
	} {
		_, err := s.Create(context.Background(), input)
		if fields := validation.Fields(err); len(fields) != 1 || fields[0].Field != "name" {
			t.Errorf("%s: err = %v, want one error on name", name, err)
		}
	}
	if _, err := s.Create(context.Background(), strings.Repeat("é", maxNameLength)); err != nil {
		t.Errorf("a name of %d characters is refused: %v", maxNameLength, err)
	}
}

func TestCreateStopsAtTheLimit(t *testing.T) {
	s, _, _ := newTestService(t)
	for i := range store.MaxAPITokens {
		if _, err := s.Create(context.Background(), fmt.Sprintf("token %d", i)); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
	if _, err := s.Create(context.Background(), "one too many"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()
	s, st, clock := newTestService(t)
	made, err := s.Create(ctx, "Home Assistant")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Authenticate(ctx, made.Token, "192.168.1.5")
	if err != nil || got.ID != made.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	// The first use is recorded; a second one within a minute is not.
	clock.t = clock.t.Add(10 * time.Second)
	if _, err := s.Authenticate(ctx, made.Token, "192.168.1.5"); err != nil {
		t.Fatal(err)
	}
	listed, _ := st.APITokens(ctx)
	if listed[0].LastUsedAt == nil || !listed[0].LastUsedAt.Equal(time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("LastUsedAt = %v, want the first use", listed[0].LastUsedAt)
	}
	clock.t = clock.t.Add(2 * time.Minute)
	if _, err := s.Authenticate(ctx, made.Token, "192.168.1.5"); err != nil {
		t.Fatal(err)
	}
	listed, _ = st.APITokens(ctx)
	if !listed[0].LastUsedAt.Equal(clock.t) {
		t.Fatalf("LastUsedAt = %v, want %v", listed[0].LastUsedAt, clock.t)
	}

	for name, secret := range map[string]string{
		"unknown":        Prefix + strings.Repeat("A", 43),
		"without prefix": strings.TrimPrefix(made.Token, Prefix),
		"empty":          "",
	} {
		if _, err := s.Authenticate(ctx, secret, "192.168.1.5"); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}

	if _, err := st.DeleteAPIToken(ctx, made.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, made.Token, "192.168.1.5"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token: err = %v, want ErrInvalidToken", err)
	}
}

func TestWrongTokensBlockAnAddress(t *testing.T) {
	ctx := context.Background()
	s, _, clock := newTestService(t)
	made, err := s.Create(ctx, "Home Assistant")
	if err != nil {
		t.Fatal(err)
	}
	for range maxFailures {
		if _, err := s.Authenticate(ctx, "nope", "10.0.0.9"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
		}
	}
	if _, err := s.Authenticate(ctx, made.Token, "10.0.0.9"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("blocked address with a good token: err = %v, want ErrRateLimited", err)
	}
	if _, err := s.Authenticate(ctx, made.Token, "10.0.0.10"); err != nil {
		t.Fatalf("another address: %v", err)
	}
	clock.t = clock.t.Add(failureWindow + time.Second)
	if _, err := s.Authenticate(ctx, made.Token, "10.0.0.9"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}
