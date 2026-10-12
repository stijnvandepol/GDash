package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
)

func widgetSources(t *testing.T, cfg settings.Sources, widgets ...display.Widget) map[string]datahub.Source {
	t.Helper()
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{settings: &settings.Service{Box: box}}
	out := map[string]datahub.Source{}
	for _, src := range d.dashboardSources(cfg, []display.Dashboard{{Widgets: widgets}}) {
		out[src.Key] = src
	}
	return out
}

func wasteOn(id float64) display.Widget {
	return display.Widget{ID: "waste", Type: "waste", Settings: map[string]any{"calendar": id}}
}

func TestWasteIsFetchedOnlyForCalendarsThatWidgetsShow(t *testing.T) {
	cfg := settings.Sources{Calendars: []calendar.Source{{ID: 1, URL: "https://a.example/a.ics"}, {ID: 2, URL: "https://a.example/b.ics"}}}
	if got := widgetSources(t, cfg); len(got) != 0 {
		t.Fatalf("sources without widgets = %v, want none", got)
	}
	if got := widgetSources(t, settings.Sources{}, wasteOn(1)); len(got) != 0 {
		t.Fatalf("sources without calendars = %v, want none, so the widget says to add one", got)
	}
	one := widgetSources(t, cfg, wasteOn(1))[display.WasteSource]
	two := widgetSources(t, cfg, wasteOn(2))[display.WasteSource]
	if one.Key == "" || one.Version == "" || one.Version == two.Version {
		t.Fatalf("versions = %q and %q; want a source whose version follows the chosen calendar", one.Version, two.Version)
	}
	changed := settings.Sources{Calendars: []calendar.Source{{ID: 1, URL: "https://a.example/new.ics"}}}
	if now := widgetSources(t, changed, wasteOn(1))[display.WasteSource]; now.Version == one.Version {
		t.Fatal("the source did not restart when the calendar's address changed")
	}
}

func TestEnergyPriceIsFetchedOnlyWhenShown(t *testing.T) {
	if got := widgetSources(t, settings.Sources{}); len(got) != 0 {
		t.Fatalf("sources without widgets = %v, want none", got)
	}
	got := widgetSources(t, settings.Sources{}, display.Widget{Type: "energyprice"})[display.EnergyPriceSource]
	if got.Interval != time.Hour || got.Fetch == nil {
		t.Fatalf("source = %+v, want one that refreshes hourly", got)
	}
}

// wasteServer serves the recorded pickup calendar, or an error, and returns
// the data sources that read it and its address.
func wasteServer(t *testing.T, status int) (*dataSources, string) {
	t.Helper()
	body, err := os.ReadFile("../../internal/waste/testdata/afvalkalender.ics")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "no", status)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	loc := time.FixedZone("CEST", 2*60*60)
	now := func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, loc) }
	return &dataSources{calendar: &calendar.Client{HTTP: srv.Client(), Location: loc, Now: now}}, srv.URL
}

func TestWastePickupsAreKeyedByCalendar(t *testing.T) {
	d, address := wasteServer(t, http.StatusOK)
	report, err := d.wastePickups(context.Background(), []calendar.Source{{ID: 7, Name: "Afval", URL: address}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string][]struct{ Kind, Date string }
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	pickups := got["7"]
	if len(got) != 1 || len(pickups) != 5 || pickups[0].Kind != "gft" || pickups[0].Date != "2026-10-13" {
		t.Fatalf("report = %s, want the five pickups of calendar 7 starting with GFT on 13 October", raw)
	}
}

func TestWastePickupsFailWhenACalendarFails(t *testing.T) {
	d, address := wasteServer(t, http.StatusInternalServerError)
	if _, err := d.wastePickups(context.Background(), []calendar.Source{{ID: 7, URL: address}}); err == nil {
		t.Fatal("wastePickups succeeded; the widget would show an empty list instead of its last good one")
	}
}
