package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDisplayTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: "hash-1", DisplayID: "keuken", Kind: TokenScreen, Created: created}); err != nil {
		t.Fatalf("CreateDisplayToken: %v", err)
	}
	tok, err := s.DisplayToken(ctx, "hash-1")
	if err != nil || tok.DisplayID != "keuken" || !tok.LastSeenAt.Equal(created) {
		t.Fatalf("DisplayToken = %+v, %v", tok, err)
	}

	later := created.Add(time.Hour)
	if err := s.TouchDisplayToken(ctx, "hash-1", later); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.DisplayToken(ctx, "hash-1"); !tok.LastSeenAt.Equal(later) {
		t.Fatalf("LastSeenAt = %v after touch, want %v", tok.LastSeenAt, later)
	}

	n, err := s.DeleteDisplayTokens(ctx, "keuken")
	if err != nil || n != 1 {
		t.Fatalf("DeleteDisplayTokens = %d, %v; want 1", n, err)
	}
	if _, err := s.DisplayToken(ctx, "hash-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token after unpairing: err = %v, want ErrNotFound", err)
	}
}

func TestDisplayTokenForMissingDisplay(t *testing.T) {
	s, _ := openTestStore(t)
	if err := s.CreateDisplayToken(context.Background(), NewDisplayToken{Hash: "hash", DisplayID: "missing", Kind: TokenScreen, Created: created}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeletingDisplayUnpairsScreens(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: "hash", DisplayID: "keuken", Kind: TokenScreen, Created: created}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDisplay(ctx, "keuken"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DisplayToken(ctx, "hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token survived deleting its display: err = %v", err)
	}
}

func TestDisplayTokenLimit(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	for i := range MaxScreensPerDisplay {
		if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: fmt.Sprintf("hash-%d", i), DisplayID: "keuken", Kind: TokenScreen, Created: created}); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
	if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: "one-too-many", DisplayID: "keuken", Kind: TokenScreen, Created: created}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestCastTokenReplacesPreviousCastToken(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: "screen", DisplayID: "keuken", Kind: TokenScreen, Created: created}); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if err := s.CreateDisplayToken(ctx, NewDisplayToken{Hash: fmt.Sprintf("cast-%d", i), DisplayID: "keuken", Kind: TokenCast, Created: created}); err != nil {
			t.Fatalf("cast token %d: %v", i, err)
		}
	}
	for _, gone := range []string{"cast-0", "cast-1"} {
		if _, err := s.DisplayToken(ctx, gone); !errors.Is(err, ErrNotFound) {
			t.Errorf("old cast token %s still valid: err = %v", gone, err)
		}
	}
	for _, kept := range []string{"cast-2", "screen"} {
		if _, err := s.DisplayToken(ctx, kept); err != nil {
			t.Errorf("token %s lost: %v", kept, err)
		}
	}
	counts, err := s.PairedScreenCounts(ctx)
	if err != nil || counts["keuken"] != 1 {
		t.Fatalf("PairedScreenCounts = %v, %v; want 1 screen, cast tokens not counted", counts, err)
	}
}
