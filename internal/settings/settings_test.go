package settings

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/secret"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

const secretURL = "https://calendar.example.org/private-0123456789/basic.ics"

func newService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "daycast.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	box, err := secret.LoadOrCreateKey(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC) }
	return &Service{Store: db, Box: box, Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func fieldNames(err error) string {
	var names []string
	for _, f := range validation.Fields(err) {
		names = append(names, f.Field)
	}
	return strings.Join(names, ",")
}

func TestLocation(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if got, err := s.Get(ctx); err != nil || got.Location != nil || got.NewsFeeds == nil || got.Calendars == nil {
		t.Fatalf("fresh settings = %+v, %v; want no location and empty lists", got, err)
	}
	if err := s.SetLocation(ctx, Location{Name: " ", Latitude: 91, Longitude: -181}); fieldNames(err) != "name,latitude,longitude" {
		t.Fatalf("invalid location: fields = %q", fieldNames(err))
	}
	if err := s.SetLocation(ctx, Location{Name: "Oss", Latitude: 51.765, Longitude: 5.518}); err != nil {
		t.Fatal(err)
	}
	src, err := s.Sources(ctx)
	if err != nil || src.Location == nil || src.Location.Latitude != 51.77 {
		t.Fatalf("Sources location = %+v, %v; want it rounded for the weather service", src.Location, err)
	}
	if err := s.ClearLocation(ctx); err != nil {
		t.Fatal(err)
	}
	if src, _ := s.Sources(ctx); src.Location != nil {
		t.Fatal("location still set after clearing it")
	}
}

func TestCalendarAddressIsSecret(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	cal, err := s.AddCalendar(ctx, "Family", "webcal://calendar.example.org/private-0123456789/basic.ics")
	if err != nil {
		t.Fatal(err)
	}
	if cal.Host != "calendar.example.org" {
		t.Fatalf("host = %q", cal.Host)
	}
	stored, _ := s.Store.Sources(ctx, store.SourceCalendar)
	if strings.Contains(stored[0].Address, "private") {
		t.Fatal("the calendar address is stored in plain text")
	}
	got, _ := s.Get(ctx)
	if len(got.Calendars) != 1 || got.Calendars[0].Unreadable {
		t.Fatalf("Get calendars = %+v", got.Calendars)
	}
	src, _ := s.Sources(ctx)
	if len(src.Calendars) != 1 || src.Calendars[0].URL != secretURL {
		t.Fatalf("Sources calendars = %+v; want the https address", src.Calendars)
	}

	// With another key the calendar is reported, not fetched.
	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if got, _ := s.Get(ctx); !got.Calendars[0].Unreadable {
		t.Fatal("a calendar sealed with another key is not marked unreadable")
	}
	if src, err := s.Sources(ctx); err != nil || len(src.Calendars) != 0 {
		t.Fatalf("Sources with another key = %+v, %v; want the calendar left out", src.Calendars, err)
	}
}

func TestSourcesValidationAndLimits(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	for _, tt := range []struct{ name, url, fields string }{
		{"", "https://example.org/rss", "name"},
		{"News", "ftp://example.org/rss", "url"},
		{"News", "example.org/rss", "url"},
		{"News", "webcal://example.org/feed", "url"}, // webcal is for calendars
		{"News", "https://example.org/" + strings.Repeat("a", maxURLLength), "url"},
	} {
		if _, err := s.AddNewsFeed(ctx, tt.name, tt.url); fieldNames(err) != tt.fields {
			t.Errorf("AddNewsFeed(%q, %.40q): fields = %q, want %q", tt.name, tt.url, fieldNames(err), tt.fields)
		}
	}
	for i := range MaxNewsFeeds {
		if _, err := s.AddNewsFeed(ctx, "Feed", "https://example.org/"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddNewsFeed(ctx, "One too many", "https://example.org/z"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("over the limit: err = %v, want ErrConflict", err)
	}
	got, _ := s.Get(ctx)
	if err := s.DeleteNewsFeed(ctx, got.NewsFeeds[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCalendar(ctx, got.NewsFeeds[1].ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleting a feed as a calendar: err = %v, want ErrNotFound", err)
	}
}

func TestSeedOnce(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	seed := Seed{
		Location:  &Location{Name: "Home", Latitude: 51.65, Longitude: 5.95},
		NewsFeeds: []NewsFeed{{Name: "NOS", URL: "https://feeds.nos.nl/nosnieuwsalgemeen"}},
		Calendars: []SeedCalendar{{Name: "Calendar 1", URL: secretURL}},
	}
	if seeded, err := s.SeedOnce(ctx, seed); err != nil || !seeded {
		t.Fatalf("first SeedOnce = %v, %v", seeded, err)
	}
	got, _ := s.Get(ctx)
	if got.Location == nil || len(got.NewsFeeds) != 1 || len(got.Calendars) != 1 {
		t.Fatalf("after seeding: %+v", got)
	}

	// After the admin removes the feed, a restart does not bring it back.
	if err := s.DeleteNewsFeed(ctx, got.NewsFeeds[0].ID); err != nil {
		t.Fatal(err)
	}
	if seeded, err := s.SeedOnce(ctx, seed); err != nil || seeded {
		t.Fatalf("second SeedOnce = %v, %v; want nothing done", seeded, err)
	}
	if got, _ := s.Get(ctx); len(got.NewsFeeds) != 0 {
		t.Fatal("a removed feed came back on restart")
	}
}

func TestSeedOnceKeepsExistingSettings(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.AddNewsFeed(ctx, "Mine", "https://example.org/rss"); err != nil {
		t.Fatal(err)
	}
	seeded, err := s.SeedOnce(ctx, Seed{NewsFeeds: []NewsFeed{{Name: "NOS", URL: "https://feeds.nos.nl/x"}}})
	if err != nil || seeded {
		t.Fatalf("SeedOnce = %v, %v; want existing settings left alone", seeded, err)
	}
	if got, _ := s.Get(ctx); len(got.NewsFeeds) != 1 || got.NewsFeeds[0].Name != "Mine" {
		t.Fatalf("feeds = %+v", got.NewsFeeds)
	}
}

func TestSeedErrorHidesCalendarAddress(t *testing.T) {
	s := newService(t)
	_, err := s.SeedOnce(context.Background(), Seed{Calendars: []SeedCalendar{{Name: "", URL: "ftp://secret-token@host/x"}}})
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("err = %v; want an error without the address", err)
	}
}

func TestLanguage(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if got, err := s.Language(ctx); err != nil || got != DefaultLanguage {
		t.Fatalf("default language = %q, %v", got, err)
	}
	if err := s.SetLanguage(ctx, "fr"); fieldNames(err) != "language" {
		t.Fatalf("unknown language: fields = %q", fieldNames(err))
	}
	if err := s.SetLanguage(ctx, "nl"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); got.Language != "nl" {
		t.Fatalf("Get language = %q, want nl", got.Language)
	}
}
