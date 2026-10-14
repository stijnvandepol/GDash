package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewsFeedNameMigrationNamesOnlyTheSeededFeed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gdash.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// A database from before the migration: the feed of the first start,
	// another feed named after the same host, and a calendar with the
	// feed's address and name.
	old := &Store{db: db}
	if err := old.migrate(ctx, path, scripts[:24]); err != nil {
		t.Fatal(err)
	}
	for _, f := range [][2]string{
		{"feeds.nos.nl", "https://feeds.nos.nl/nosnieuwsalgemeen"},
		{"feeds.nos.nl", "https://feeds.nos.nl/nossportalgemeen"},
	} {
		if _, err := db.ExecContext(ctx, `INSERT INTO sources (kind, name, address, host, color, created_at) VALUES ('news', ?, ?, 'feeds.nos.nl', '', '2026-01-01T00:00:00Z')`, f[0], f[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sources (kind, name, address, host, color, created_at) VALUES ('calendar', 'feeds.nos.nl', 'https://feeds.nos.nl/nosnieuwsalgemeen', 'feeds.nos.nl', '', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	feeds, err := s.Sources(ctx, SourceNews)
	if err != nil || len(feeds) != 2 || feeds[0].Name != "NOS Nieuws" || feeds[1].Name != "feeds.nos.nl" {
		t.Fatalf("news feeds after the migration = %+v, %v; want only the seeded feed renamed", feeds, err)
	}
	if calendars, err := s.Sources(ctx, SourceCalendar); err != nil || calendars[0].Name != "feeds.nos.nl" {
		t.Fatalf("calendars after the migration = %+v, %v; want them left alone", calendars, err)
	}
}

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

func TestSourceLimitIsPerPurpose(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	add := func(name, purpose string) (int64, error) {
		return s.AddSource(ctx, Source{Kind: SourceCalendar, Name: name, Address: "sealed", Host: "example.org", Purpose: purpose, CreatedAt: created}, 1)
	}
	if _, err := add("Family", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := add("Home", PurposeWaste); err != nil {
		t.Fatalf("a waste calendar beside a full list of calendars: %v", err)
	}
	if _, err := add("Holiday house", PurposeWaste); !errors.Is(err, ErrConflict) {
		t.Fatalf("second waste calendar: err = %v, want ErrConflict", err)
	}
	list, err := s.Sources(ctx, SourceCalendar)
	if err != nil || len(list) != 2 || list[0].Purpose != "" || list[1].Purpose != PurposeWaste {
		t.Fatalf("Sources = %+v, %v; want each with its purpose", list, err)
	}
}

func TestMigrationMakesTheCalendarsOfWasteWidgetsWasteCalendars(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gdash.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// A database from before waste calendars: three calendars, the second
	// shown by a Waste collection widget, the third by a widget on another
	// dashboard that names it, and the first by none.
	old := &Store{db: db}
	if err := old.migrate(ctx, path, scripts[:len(scripts)-1]); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO sources (kind, name, address, host, created_at) VALUES
			('calendar', 'Family', 'sealed', 'example.org', '2026-01-01T00:00:00Z'),
			('calendar', 'Afval', 'sealed', 'example.org', '2026-01-01T00:00:00Z'),
			('calendar', 'Holiday house', 'sealed', 'example.org', '2026-01-01T00:00:00Z'),
			('news', 'NOS', 'https://feeds.nos.nl', 'feeds.nos.nl', '2026-01-01T00:00:00Z')`,
		`INSERT INTO dashboards (id, name, widgets, created_at, updated_at) VALUES
			('default', 'Home', '[{"id":"c","type":"calendar","settings":{"maxItems":5}},{"id":"w","type":"waste","settings":{"calendar":2}}]', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			('hall', 'Hall', '[{"id":"w","type":"waste","settings":{"calendar":3}},{"id":"t","type":"text","settings":{"calendar":1}}]', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	list, err := s.Sources(ctx, SourceCalendar)
	if err != nil || len(list) != 3 {
		t.Fatalf("calendars after the migration = %+v, %v", list, err)
	}
	if list[0].Purpose != "" || list[1].Purpose != PurposeWaste || list[2].Purpose != PurposeWaste {
		t.Fatalf("purposes = %q, %q, %q; want Afval and Holiday house waste calendars and Family as it was", list[0].Purpose, list[1].Purpose, list[2].Purpose)
	}
	if news, _ := s.Sources(ctx, SourceNews); len(news) != 1 || news[0].Purpose != "" {
		t.Fatalf("news after the migration = %+v", news)
	}
	if _, err := os.Stat(fmt.Sprintf("%s.bak-v%d", path, len(scripts)-1)); err != nil {
		t.Fatalf("no backup before the migration: %v", err)
	}
}
