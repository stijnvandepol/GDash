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

	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const secretURL = "https://calendar.example.org/private-0123456789/basic.ics"

func newService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "gdash.db"))
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

func TestFeedAddressMayNotHoldAPassword(t *testing.T) {
	s := newService(t)
	_, err := s.AddNewsFeed(context.Background(), "Private", "https://user:secret@feeds.example.org/rss")
	if fieldNames(err) != "url" {
		t.Fatalf("feed with a password: fields = %q, want url", fieldNames(err))
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("the error repeats the password: %v", err)
	}
}

func TestFeedCannotBeAddedTwice(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	first, err := s.AddNewsFeed(ctx, "NOS", "https://feeds.nos.nl/nosnieuwsalgemeen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNewsFeed(ctx, "NOS again", "https://feeds.nos.nl/nosnieuwsalgemeen"); fieldNames(err) != "url" {
		t.Fatalf("the same address twice: fields = %q, want url", fieldNames(err))
	}
	// The data hub keys each feed's headlines by its ID.
	src, err := s.Sources(ctx)
	if err != nil || len(src.NewsFeeds) != 1 || src.NewsFeeds[0].ID != first.ID || src.NewsFeeds[0].Name != "NOS" {
		t.Fatalf("Sources().NewsFeeds = %+v, %v; want the one feed with its ID", src.NewsFeeds, err)
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
	if err := s.SetLanguage(ctx, "es"); fieldNames(err) != "language" {
		t.Fatalf("unknown language: fields = %q", fieldNames(err))
	}
	for _, language := range []string{"nl", "de", "fr", "en"} {
		if err := s.SetLanguage(ctx, language); err != nil {
			t.Fatalf("set %s: %v", language, err)
		}
		if got, _ := s.Get(ctx); got.Language != language {
			t.Fatalf("Get language = %q, want %s", got.Language, language)
		}
	}
}

func TestUpdateCheckIsOnUntilTurnedOff(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if got, err := s.Get(ctx); err != nil || !got.UpdateCheck {
		t.Fatalf("fresh install: updateCheck = %v, %v; want on", got.UpdateCheck, err)
	}
	for _, on := range []bool{false, true, false} {
		if err := s.SetUpdateCheck(ctx, on); err != nil {
			t.Fatal(err)
		}
		if got, err := s.UpdateCheck(ctx); err != nil || got != on {
			t.Fatalf("UpdateCheck = %v, %v; want %v", got, err, on)
		}
	}
}

func TestSeedWritesNothingWhenAnyPartIsInvalid(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	seed := Seed{
		Location:  &Location{Name: "Home", Latitude: 51.65, Longitude: 5.95},
		Calendars: []SeedCalendar{{Name: "Calendar 1", URL: secretURL}, {Name: "Calendar 2", URL: "ftp://secret-token@host/x"}},
	}
	if _, err := s.SeedOnce(ctx, seed); err == nil {
		t.Fatal("SeedOnce accepted an invalid calendar")
	}
	if got, _ := s.Get(ctx); got.Location != nil || len(got.Calendars) != 0 {
		t.Fatalf("settings after a failed seed = %+v; want nothing written, so the next start tries again", got)
	}
	seed.Calendars = seed.Calendars[:1]
	if seeded, err := s.SeedOnce(ctx, seed); err != nil || !seeded {
		t.Fatalf("SeedOnce after fixing the environment = %v, %v", seeded, err)
	}
}

func TestSeedKeepsAtMostTheLimit(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	var seed Seed
	for i := range MaxCalendars + 2 {
		seed.Calendars = append(seed.Calendars, SeedCalendar{Name: "Calendar", URL: secretURL + "?n=" + string(rune('a'+i))})
	}
	if _, err := s.SeedOnce(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); len(got.Calendars) != MaxCalendars {
		t.Fatalf("calendars = %d, want %d", len(got.Calendars), MaxCalendars)
	}
}

func TestCalendarColors(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	for _, name := range []string{"Family", "Work"} {
		if _, err := s.AddCalendar(ctx, name, secretURL+"?"+name); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.Get(ctx)
	if got.Calendars[0].Color != "blue" || got.Calendars[1].Color != "green" {
		t.Fatalf("default colours = %s, %s; want them to follow the list", got.Calendars[0].Color, got.Calendars[1].Color)
	}
	if err := s.SetCalendarColor(ctx, got.Calendars[0].ID, "magenta"); fieldNames(err) != "color" {
		t.Fatalf("unknown colour: fields = %q", fieldNames(err))
	}
	if err := s.SetCalendarColor(ctx, got.Calendars[0].ID, "orange"); err != nil {
		t.Fatal(err)
	}
	src, _ := s.Sources(ctx)
	if src.Calendars[0].Color != "orange" || src.Calendars[1].Color != "green" {
		t.Fatalf("source colours = %s, %s", src.Calendars[0].Color, src.Calendars[1].Color)
	}
}
