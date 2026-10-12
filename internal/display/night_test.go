package display

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/datahub"
)

// sunDay is one day of the weather report; "" leaves a time out, as the
// report does near the poles.
func sunDay(date, rise, set string) string {
	field := func(day, clock string) string {
		if clock == "" {
			return `""`
		}
		return `"` + day + "T" + clock + `"`
	}
	return fmt.Sprintf(`{"date":%q,"sunrise":%s,"sunset":%s}`, date, field(date, rise), field(date, set))
}

// weatherData is the weather entry for a location two hours ahead of UTC,
// with the sun of 8 and 9 October (Amsterdam: up 07:48 and 07:50, down 18:58
// and 18:56 local time).
func weatherData(offset string, isDay bool, days ...string) map[string]datahub.Entry {
	list := "["
	for i, day := range days {
		if i > 0 {
			list += ","
		}
		list += day
	}
	value := fmt.Sprintf(`{"isDay":%t,%s"sun":%s]}`, isDay, offset, list)
	return map[string]datahub.Entry{WeatherSource: {Value: json.RawMessage(value)}}
}

const plusTwo = `"utcOffsetSeconds":7200,`

func amsterdam() map[string]datahub.Entry {
	return weatherData(plusTwo, true, sunDay("2026-10-08", "07:48", "18:58"), sunDay("2026-10-09", "07:50", "18:56"))
}

func TestIsNight(t *testing.T) {
	utc := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, time.UTC) }
	tests := []struct {
		name string
		at   time.Time
		data map[string]datahub.Entry
		want bool
	}{
		{"before sunset", utc(8, 16, 57), amsterdam(), false},
		{"in the minute of sunset", utc(8, 16, 58), amsterdam(), true},
		{"after sunset", utc(8, 19, 0), amsterdam(), true},
		{"just before midnight local", utc(8, 21, 59), amsterdam(), true},
		// 22:00 UTC is 00:00 on the 9th at the location: yesterday's sunset
		// no longer counts, the next sunrise has not come.
		{"after midnight local", utc(8, 22, 30), amsterdam(), true},
		{"before sunrise", utc(9, 5, 49), amsterdam(), true},
		{"in the minute of sunrise", utc(9, 5, 50), amsterdam(), false},
		{"midday", utc(9, 10, 0), amsterdam(), false},
		// 02:00 UTC on the 9th is still the evening of the 8th five hours behind.
		{"a negative offset", utc(9, 2, 0), weatherData(`"utcOffsetSeconds":-18000,`, true, sunDay("2026-10-08", "07:00", "18:30"), sunDay("2026-10-09", "07:00", "18:30")), true},
		{"no location", utc(8, 19, 0), nil, false},
		{"weather still loading", utc(8, 19, 0), map[string]datahub.Entry{WeatherSource: {}}, false},
		{"report without an offset (cached by an older version)", utc(8, 19, 0), weatherData("", true, sunDay("2026-10-08", "07:48", "18:58")), false},
		{"no sun times for today", utc(12, 19, 0), amsterdam(), false},
		{"malformed sunset", utc(8, 19, 0), weatherData(plusTwo, true, `{"date":"2026-10-08","sunrise":"2026-10-08T07:48","sunset":"soon"}`), false},
		{"not a report", utc(8, 19, 0), map[string]datahub.Entry{WeatherSource: {Value: json.RawMessage(`[1]`)}}, false},
		{"polar night, the report says night", utc(8, 12, 0), weatherData(plusTwo, false, sunDay("2026-10-08", "", "")), true},
		{"midnight sun, the report says day", utc(8, 22, 0), weatherData(plusTwo, true, sunDay("2026-10-08", "", "")), false},
		{"sets but does not rise: up before the sunset", utc(8, 10, 0), weatherData(plusTwo, true, sunDay("2026-10-08", "", "13:00")), false},
		{"sets but does not rise: down after it", utc(8, 12, 0), weatherData(plusTwo, true, sunDay("2026-10-08", "", "13:00")), true},
		{"rises but does not set: down before the sunrise", utc(8, 5, 0), weatherData(plusTwo, false, sunDay("2026-10-08", "09:00", "")), true},
		{"rises but does not set: up after it", utc(8, 12, 0), weatherData(plusTwo, true, sunDay("2026-10-08", "09:00", "")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNight(tt.at, tt.data); got != tt.want {
				t.Fatalf("IsNight(%s) = %t, want %t", tt.at.Format(time.RFC3339), got, tt.want)
			}
		})
	}
}

func TestNightThemeReplacesTheThemeAndKeepsTheFont(t *testing.T) {
	d := Dashboard{ID: "hall", Theme: "frost", NightTheme: "frost-night", Font: "lora"}
	if got := withNightTheme(d, false); got.Theme != "frost" {
		t.Fatalf("by day the theme is %q, want frost", got.Theme)
	}
	got := withNightTheme(d, true)
	if got.Theme != "frost-night" || got.Font != "lora" {
		t.Fatalf("at night theme %q font %q, want frost-night and lora", got.Theme, got.Font)
	}
	plain := Dashboard{ID: "hall", Theme: "frost"}
	if got := withNightTheme(plain, true); got.Theme != "frost" {
		t.Fatalf("without a night theme the dashboard is %q at night, want frost", got.Theme)
	}
	if d.Theme != "frost" {
		t.Fatal("withNightTheme changed the dashboard it was given")
	}
}

func themedView() map[string]View {
	hall := Normalize(Dashboard{ID: "hall", Name: "Hall", Theme: "frost", NightTheme: "frost-night", Font: "lora"})
	other := Normalize(Dashboard{ID: "study", Name: "Study", Theme: "minimal-light", NightTheme: "minimal-dark"})
	return map[string]View{"hall": {Dashboard: hall, Pages: []Dashboard{other}, PageSeconds: 30}}
}

func publishAt(t *testing.T, at time.Time) Snapshot {
	t.Helper()
	b := NewBroadcaster()
	if err := b.Publish(at, themedView(), amsterdam(), "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return decode(t, <-b.Subscribe(ctx, "hall"))
}

func TestSnapshotCarriesTheNightThemeAfterSunset(t *testing.T) {
	day := publishAt(t, time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC))
	if day.Dashboard.Theme != "frost" || day.Pages[1].Theme != "minimal-light" {
		t.Fatalf("by day: %q and page %q, want the dashboards' own themes", day.Dashboard.Theme, day.Pages[1].Theme)
	}
	night := publishAt(t, time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC))
	if night.Dashboard.Theme != "frost-night" || night.Dashboard.Font != "lora" {
		t.Fatalf("at night: theme %q font %q, want frost-night and lora", night.Dashboard.Theme, night.Dashboard.Font)
	}
	if night.Pages[0].Theme != "frost-night" || night.Pages[1].Theme != "minimal-dark" {
		t.Fatalf("at night the pages are %q and %q, want each in its own night theme", night.Pages[0].Theme, night.Pages[1].Theme)
	}
}

// The TV changes theme when a new snapshot arrives. The server sends one
// every minute (cmd/gdash republishEveryMinute), so the switch is at most a
// minute after sunset; this test is the publisher's half of that.
func TestRefreshOnTheMinuteAfterSunsetSwitchesTheTheme(t *testing.T) {
	clock := time.Date(2026, 10, 8, 16, 57, 30, 0, time.UTC)
	b := NewBroadcaster()
	p := &Publisher{
		Broadcaster: b,
		Views:       func(context.Context) (map[string]View, error) { return themedView(), nil },
		Data:        amsterdam,
		Language:    func(context.Context) (string, error) { return "en", nil },
		Announcements: announcement.NewBoard(func() time.Time {
			return clock
		}),
		Now:    func() time.Time { return clock },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx, "hall")

	p.Refresh(ctx)
	if got := decode(t, <-ch).Dashboard.Theme; got != "frost" {
		t.Fatalf("before sunset the theme is %q, want frost", got)
	}
	clock = clock.Add(time.Minute) // 16:58:30 UTC is 18:58:30 locally: sunset was 18:58.
	p.Refresh(ctx)
	if got := decode(t, <-ch).Dashboard.Theme; got != "frost-night" {
		t.Fatalf("a minute later the theme is %q, want frost-night", got)
	}
}

func TestPreviewShowsTheNightThemeAfterSunset(t *testing.T) {
	d := Dashboard{ID: "hall", Name: "Hall", Theme: "frost", NightTheme: "frost-night"}
	after, err := PreviewSnapshot(time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC), d, amsterdam(), "en", 100)
	if err != nil {
		t.Fatal(err)
	}
	before, err := PreviewSnapshot(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC), d, amsterdam(), "en", 100)
	if err != nil {
		t.Fatal(err)
	}
	if after.Dashboard.Theme != "frost-night" || before.Dashboard.Theme != "frost" {
		t.Fatalf("preview themes %q after and %q before sunset, want frost-night and frost", after.Dashboard.Theme, before.Dashboard.Theme)
	}
	if after.Version == before.Version {
		t.Fatal("the preview keeps its version across the switch, so the page would not redraw")
	}
}
