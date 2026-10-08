package store

import (
	"context"
	"errors"
	"testing"
)

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if _, ok, err := s.Setting(ctx, "location"); ok || err != nil {
		t.Fatalf("unset setting: ok %v, err %v", ok, err)
	}
	for _, v := range []string{`{"name":"Oss"}`, `{"name":"Uden"}`} {
		if err := s.SetSetting(ctx, "location", v); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok, err := s.Setting(ctx, "location"); !ok || err != nil || v != `{"name":"Uden"}` {
		t.Fatalf("Setting = %q %v %v, want the second value", v, ok, err)
	}
	if err := s.DeleteSetting(ctx, "location"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Setting(ctx, "location"); ok {
		t.Fatal("setting still there after delete")
	}
}

func TestSources(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	add := func(kind, name string) (int64, error) {
		return s.AddSource(ctx, Source{Kind: kind, Name: name, Address: "sealed", Host: "example.org", CreatedAt: created}, 2)
	}
	first, err := add(SourceCalendar, "Family")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add(SourceCalendar, "Work"); err != nil {
		t.Fatal(err)
	}
	if _, err := add(SourceCalendar, "Third"); !errors.Is(err, ErrConflict) {
		t.Fatalf("third calendar: err = %v, want ErrConflict", err)
	}
	if _, err := add(SourceNews, "NOS"); err != nil {
		t.Fatalf("the limit is per kind: %v", err)
	}
	list, err := s.Sources(ctx, SourceCalendar)
	if err != nil || len(list) != 2 || list[0].Name != "Family" || list[0].Host != "example.org" || !list[0].CreatedAt.Equal(created) {
		t.Fatalf("Sources = %+v, %v", list, err)
	}
	if err := s.DeleteSource(ctx, SourceNews, first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete with the wrong kind: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteSource(ctx, SourceCalendar, first); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Sources(ctx, SourceCalendar); len(list) != 1 || list[0].Name != "Work" {
		t.Fatalf("after delete: %+v", list)
	}
}
