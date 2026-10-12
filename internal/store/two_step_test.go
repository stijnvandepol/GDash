package store

import (
	"context"
	"errors"
	"testing"
)

func twoStepUser(t *testing.T) (*Store, User) {
	t.Helper()
	s, _ := openTestStore(t)
	u, err := s.CreateFirstUser(context.Background(), "stijn", "hash", created)
	if err != nil {
		t.Fatal(err)
	}
	return s, u
}

func TestTwoStepMigrationCreatesItsTables(t *testing.T) {
	s, _ := openTestStore(t)
	for _, table := range []string{"two_step", "recovery_codes"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s: count %d, err %v", table, n, err)
		}
	}
}

func TestTwoStepLifecycle(t *testing.T) {
	ctx := context.Background()
	s, u := twoStepUser(t)
	if _, err := s.TwoStep(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("TwoStep before setup: err = %v, want ErrNotFound", err)
	}
	if err := s.StartTwoStep(ctx, u.ID, "sealed-1", created); err != nil {
		t.Fatal(err)
	}
	// A setup that never got its code may start over with a new secret.
	if err := s.StartTwoStep(ctx, u.ID, "sealed-2", created); err != nil {
		t.Fatal(err)
	}
	got, err := s.TwoStep(ctx, u.ID)
	if err != nil || got.SecretSealed != "sealed-2" || got.Enabled || !got.CreatedAt.Equal(created) {
		t.Fatalf("TwoStep after setup = %+v, %v", got, err)
	}

	if err := s.CreateSession(ctx, "keep", u.ID, created); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, "other", u.ID, created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "sealed-2", 100, []string{"h1", "h2"}, "keep"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByHash(ctx, "other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other session after enabling: err = %v, want ErrNotFound", err)
	}
	if _, err := s.SessionByHash(ctx, "keep"); err != nil {
		t.Fatalf("own session after enabling: %v", err)
	}
	if got, _ := s.TwoStep(ctx, u.ID); !got.Enabled || got.LastStep != 100 {
		t.Fatalf("TwoStep after enabling = %+v", got)
	}
	if err := s.StartTwoStep(ctx, u.ID, "sealed-3", created); !errors.Is(err, ErrConflict) {
		t.Fatalf("StartTwoStep while on: err = %v, want ErrConflict", err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "sealed-2", 101, nil, "keep"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("EnableTwoStep twice: err = %v, want ErrNotFound", err)
	}

	wasOn, err := s.DeleteTwoStep(ctx, u.ID)
	if err != nil || !wasOn {
		t.Fatalf("DeleteTwoStep = %v, %v", wasOn, err)
	}
	if n, _ := s.RecoveryCodesLeft(ctx, u.ID); n != 0 {
		t.Fatalf("%d recovery codes left after turning off", n)
	}
	if wasOn, err := s.DeleteTwoStep(ctx, u.ID); err != nil || wasOn {
		t.Fatalf("second DeleteTwoStep = %v, %v", wasOn, err)
	}
}

// A second setup, in another browser tab, may replace the secret between
// the check of the first code and turning two-step login on; it must not
// turn on with a secret whose code nobody checked.
func TestEnableTwoStepNeedsTheCheckedSecret(t *testing.T) {
	ctx := context.Background()
	s, u := twoStepUser(t)
	if err := s.StartTwoStep(ctx, u.ID, "checked", created); err != nil {
		t.Fatal(err)
	}
	if err := s.StartTwoStep(ctx, u.ID, "replaced", created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "checked", 100, []string{"h1"}, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("EnableTwoStep with a replaced secret: err = %v, want ErrConflict", err)
	}
	got, err := s.TwoStep(ctx, u.ID)
	if err != nil || got.Enabled || got.SecretSealed != "replaced" {
		t.Fatalf("TwoStep after the refused enable = %+v, %v", got, err)
	}
	if n, _ := s.RecoveryCodesLeft(ctx, u.ID); n != 0 {
		t.Fatalf("%d recovery codes stored by the refused enable", n)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "replaced", 100, nil, ""); err != nil {
		t.Fatalf("EnableTwoStep with the current secret: %v", err)
	}
}

func TestTwoStepStepIsUsedOnce(t *testing.T) {
	ctx := context.Background()
	s, u := twoStepUser(t)
	if err := s.StartTwoStep(ctx, u.ID, "sealed", created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "sealed", 100, nil, ""); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		step int64
		ok   bool
	}{{100, false}, {99, false}, {101, true}, {101, false}, {103, true}} {
		if ok, err := s.UseTwoStepStep(ctx, u.ID, c.step); err != nil || ok != c.ok {
			t.Fatalf("UseTwoStepStep(%d) = %v, %v; want %v", c.step, ok, err, c.ok)
		}
	}
}

func TestRecoveryCodesAreSingleUse(t *testing.T) {
	ctx := context.Background()
	s, u := twoStepUser(t)
	if err := s.StartTwoStep(ctx, u.ID, "sealed", created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "sealed", 1, []string{"h1", "h2", "h3"}, ""); err != nil {
		t.Fatal(err)
	}
	if left, ok, err := s.UseRecoveryCode(ctx, u.ID, "h2"); err != nil || !ok || left != 2 {
		t.Fatalf("first use = %d, %v, %v; want 2 left", left, ok, err)
	}
	if _, ok, err := s.UseRecoveryCode(ctx, u.ID, "h2"); err != nil || ok {
		t.Fatalf("second use = %v, %v; want refused", ok, err)
	}
	if _, ok, err := s.UseRecoveryCode(ctx, u.ID, "unknown"); err != nil || ok {
		t.Fatalf("unknown code = %v, %v; want refused", ok, err)
	}
	if n, err := s.RecoveryCodesLeft(ctx, u.ID); err != nil || n != 2 {
		t.Fatalf("RecoveryCodesLeft = %d, %v", n, err)
	}
}

func TestResetPasswordTurnsTwoStepOff(t *testing.T) {
	ctx := context.Background()
	s, u := twoStepUser(t)
	if err := s.StartTwoStep(ctx, u.ID, "sealed", created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoStep(ctx, u.ID, "sealed", 1, []string{"h1"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, "browser", u.ID, created); err != nil {
		t.Fatal(err)
	}
	wasOn, err := s.ResetPassword(ctx, u.ID, "new-hash")
	if err != nil || !wasOn {
		t.Fatalf("ResetPassword = %v, %v", wasOn, err)
	}
	if _, err := s.TwoStep(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("two-step login after reset: err = %v", err)
	}
	if _, err := s.SessionByHash(ctx, "browser"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session after reset: err = %v", err)
	}
	if got, _ := s.UserByName(ctx, "stijn"); got.PasswordHash != "new-hash" {
		t.Fatalf("password hash after reset = %q", got.PasswordHash)
	}
	if wasOn, err := s.ResetPassword(ctx, u.ID, "newer-hash"); err != nil || wasOn {
		t.Fatalf("reset without two-step = %v, %v", wasOn, err)
	}
	if _, err := s.ResetPassword(ctx, 999, "hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reset of an unknown user: err = %v", err)
	}
}
