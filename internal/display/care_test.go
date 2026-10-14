package display

import (
	"context"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/schedule"
)

func TestDimmingAt(t *testing.T) {
	evening := time.Date(2026, 10, 8, 23, 0, 0, 0, time.UTC)
	noon := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	night := schedule.Hours{From: "22:00", Until: "07:00"}
	tests := []struct {
		name    string
		dimming Dimming
		at      time.Time
		isNight bool
		want    bool
	}{
		{"off by night", Dimming{Mode: DimOff}, evening, true, false},
		{"after sunset, sun down", Dimming{Mode: DimAfterSunset}, noon, true, true},
		{"after sunset, sun up", Dimming{Mode: DimAfterSunset}, evening, false, false},
		{"during hours, inside", Dimming{Mode: DimDuringHours, Hours: night}, evening, false, true},
		{"during hours, outside", Dimming{Mode: DimDuringHours, Hours: night}, noon, true, false},
		{"during hours without them", Dimming{Mode: DimDuringHours}, evening, true, false},
		{"during invalid hours", Dimming{Mode: DimDuringHours, Hours: schedule.Hours{From: "25:00", Until: "07:00"}}, evening, true, false},
		{"hours kept while off", Dimming{Mode: DimOff, Hours: night}, evening, true, false},
		{"no mode at all", Dimming{}, evening, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dimming.At(tt.at, tt.isNight); got != tt.want {
				t.Fatalf("At = %v, want %v", got, tt.want)
			}
		})
	}
}

func publishView(t *testing.T, at time.Time, view View, data map[string]datahub.Entry) Snapshot {
	t.Helper()
	b := NewBroadcaster()
	if err := b.Publish(at, map[string]View{"bedroom": view}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return decode(t, <-b.Subscribe(ctx, "bedroom"))
}

func TestSnapshotCarriesScreenCare(t *testing.T) {
	view := View{Dashboard: Normalize(DefaultDashboard(false)), ScreenCare: ScreenCareOLED}
	if got := publishView(t, now, view, nil); got.ScreenCare != ScreenCareOLED {
		t.Fatalf("screen care = %q, want oled", got.ScreenCare)
	}
	view.ScreenCare = ScreenCareOff
	if got := publishView(t, now, view, nil); got.ScreenCare != ScreenCareOff {
		t.Fatalf("screen care = %q, want off", got.ScreenCare)
	}
}

// Dimming after sunset follows the same sun as the night theme, so the two
// switch in the same snapshot.
func TestSnapshotIsDimmedAfterSunsetWithTheNightTheme(t *testing.T) {
	board := Normalize(DefaultDashboard(false))
	board.Theme, board.NightTheme = "minimal-light", "starlight"
	view := View{Dashboard: board, Dimming: Dimming{Mode: DimAfterSunset}}
	day := publishView(t, time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC), view, amsterdam())
	if day.Dimmed || day.Dashboard.Theme != "minimal-light" {
		t.Fatalf("by day: dimmed %v in %q, want full brightness in the day theme", day.Dimmed, day.Dashboard.Theme)
	}
	night := publishView(t, time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC), view, amsterdam())
	if !night.Dimmed || night.Dashboard.Theme != "starlight" {
		t.Fatalf("after sunset: dimmed %v in %q, want dimmed in the night theme", night.Dimmed, night.Dashboard.Theme)
	}
	if got := publishView(t, time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC), view, nil); got.Dimmed {
		t.Fatal("dimmed without a weather location, which can never tell sunset")
	}
}

func TestSnapshotIsDimmedDuringItsHoursOnlyWhileActive(t *testing.T) {
	view := View{
		Dashboard: Normalize(DefaultDashboard(false)),
		Dimming:   Dimming{Mode: DimDuringHours, Hours: schedule.Hours{From: "07:00", Until: "08:00"}},
	}
	// now is 07:30.
	if got := publishView(t, now, view, nil); !got.Dimmed {
		t.Fatal("not dimmed inside its hours")
	}
	if got := publishView(t, now.Add(time.Hour), view, nil); got.Dimmed {
		t.Fatal("dimmed outside its hours")
	}
	view.Hours = schedule.Hours{From: "09:00", Until: "22:00"}
	if got := publishView(t, now, view, nil); got.Active || got.Dimmed {
		t.Fatalf("outside active hours: active %v, dimmed %v; want a dark screen, not a dimmed one", got.Active, got.Dimmed)
	}
}
