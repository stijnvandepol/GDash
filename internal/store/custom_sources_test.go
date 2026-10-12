package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCustomSourceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	src := CustomSource{
		Name: "Prices", URL: "https://api.example.com/prices?currency=EUR", Method: "POST", Body: `{"q":1}`,
		Auth: "header", AuthName: "X-API-Key", Headers: `[{"name":"Accept-Language","value":"nl"}]`,
		SecretsSealed: "sealed", IntervalSeconds: 300, Sample: `{"price":1}`, CreatedAt: created, UpdatedAt: created,
	}
	id, err := s.AddCustomSource(ctx, src, 20)
	if err != nil || id == 0 {
		t.Fatalf("AddCustomSource = %d, %v", id, err)
	}
	src.ID = id
	got, err := s.CustomSource(ctx, id)
	if err != nil || got != src {
		t.Fatalf("CustomSource = %+v, %v; want %+v", got, err, src)
	}

	src.Name, src.Auth, src.SecretsSealed, src.UpdatedAt = "Renamed", "none", "", created.Add(time.Hour)
	if err := s.UpdateCustomSource(ctx, src); err != nil {
		t.Fatal(err)
	}
	list, err := s.CustomSources(ctx)
	if err != nil || len(list) != 1 || list[0] != src {
		t.Fatalf("CustomSources = %+v, %v; want [%+v]", list, err, src)
	}

	if err := s.DeleteCustomSource(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CustomSource(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted source: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteCustomSource(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
	if err := s.UpdateCustomSource(ctx, src); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of a deleted source: err = %v, want ErrNotFound", err)
	}
}

func TestCustomSourceLimitAndChecks(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	base := CustomSource{URL: "https://x", Method: "GET", Auth: "none", Headers: "[]", IntervalSeconds: 60, CreatedAt: created, UpdatedAt: created}
	for i := range 3 {
		base.Name = fmt.Sprintf("api %d", i)
		if _, err := s.AddCustomSource(ctx, base, 3); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddCustomSource(ctx, base, 3); !errors.Is(err, ErrConflict) {
		t.Fatalf("over the limit: err = %v, want ErrConflict", err)
	}
	// The schema refuses what the web interface never sends.
	for _, bad := range []CustomSource{
		{Method: "DELETE", Auth: "none", IntervalSeconds: 60},
		{Method: "GET", Auth: "cookie", IntervalSeconds: 60},
		{Method: "GET", Auth: "none", IntervalSeconds: 5},
	} {
		bad.CreatedAt, bad.UpdatedAt = created, created
		if _, err := s.AddCustomSource(ctx, bad, 50); err == nil {
			t.Errorf("stored %+v", bad)
		}
	}
}
