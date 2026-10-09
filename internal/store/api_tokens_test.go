package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestAPITokenLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	made, err := s.CreateAPIToken(ctx, NewAPIToken{Name: "Home Assistant", Hash: "hash-1", Created: created})
	if err != nil || made.ID == 0 || made.Name != "Home Assistant" || made.LastUsedAt != nil {
		t.Fatalf("CreateAPIToken = %+v, %v", made, err)
	}

	found, err := s.APITokenByHash(ctx, "hash-1")
	if err != nil || found.ID != made.ID || !found.CreatedAt.Equal(created) || found.LastUsedAt != nil {
		t.Fatalf("APITokenByHash = %+v, %v", found, err)
	}
	if _, err := s.APITokenByHash(ctx, "other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown hash: err = %v, want ErrNotFound", err)
	}

	later := created.Add(time.Hour)
	if err := s.TouchAPIToken(ctx, made.ID, later); err != nil {
		t.Fatal(err)
	}
	list, err := s.APITokens(ctx)
	if err != nil || len(list) != 1 || list[0].LastUsedAt == nil || !list[0].LastUsedAt.Equal(later) {
		t.Fatalf("APITokens after touch = %+v, %v", list, err)
	}

	name, err := s.DeleteAPIToken(ctx, made.ID)
	if err != nil || name != "Home Assistant" {
		t.Fatalf("DeleteAPIToken = %q, %v", name, err)
	}
	if _, err := s.APITokenByHash(ctx, "hash-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token survived revoking: err = %v", err)
	}
	if _, err := s.DeleteAPIToken(ctx, made.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second revoke: err = %v, want ErrNotFound", err)
	}
}

func TestAPITokenLimit(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	for i := range MaxAPITokens {
		if _, err := s.CreateAPIToken(ctx, NewAPIToken{Name: fmt.Sprintf("token %d", i), Hash: fmt.Sprintf("hash-%d", i), Created: created}); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
	if _, err := s.CreateAPIToken(ctx, NewAPIToken{Name: "one too many", Hash: "hash-extra", Created: created}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
