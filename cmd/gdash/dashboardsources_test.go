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
	"github.com/stijnvandepol/gdash/internal/fuel"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/weather"
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

func TestHolidaysReadTheSchoolFeedOnlyWhenShown(t *testing.T) {
	if got := widgetSources(t, settings.Sources{}); got[display.HolidaysSource].Key != "" {
		t.Fatalf("sources without Holidays widgets = %v, want none", got)
	}
	public := widgetSources(t, settings.Sources{}, display.Widget{Type: "holidays", Settings: map[string]any{"show": "public"}})[display.HolidaysSource]
	both := widgetSources(t, settings.Sources{}, display.Widget{Type: "holidays"})[display.HolidaysSource]
	south := widgetSources(t, settings.Sources{}, display.Widget{Type: "holidays", Settings: map[string]any{"region": "south"}})[display.HolidaysSource]
	if public.Interval != 24*time.Hour || public.Fetch == nil || both.Interval != 24*time.Hour {
		t.Fatalf("sources = %+v and %+v, want daily ones", public, both)
	}
	// The feed holds every region: another region needs no new fetch, but
	// the school holidays do.
	if public.Version == both.Version || both.Version != south.Version {
		t.Fatalf("versions = %q, %q and %q", public.Version, both.Version, south.Version)
	}
}

func fuelOn(settings map[string]any) display.Widget {
	return display.Widget{ID: "fuel", Type: "fuel", Settings: settings}
}

func TestFuelIsFetchedOnlyForPlacesThatWidgetsShow(t *testing.T) {
	oss := map[string]any{"name": "Oss", "latitude": 51.77, "longitude": 5.52}
	here := settings.Sources{Location: &weather.Location{Latitude: 51.77, Longitude: 5.52}}

	if got := widgetSources(t, here); got[display.FuelSource].Key != "" {
		t.Fatalf("sources without Fuel widgets = %v, want no fuel source", got)
	}
	if got := widgetSources(t, settings.Sources{}, fuelOn(nil)); got[display.FuelSource].Key != "" {
		t.Fatal("a Fuel widget without a place and without a weather location got a source; it has nothing to fetch")
	}
	own := widgetSources(t, settings.Sources{}, fuelOn(map[string]any{"place": oss}))[display.FuelSource]
	if own.Key == "" || own.Interval != 15*time.Minute {
		t.Fatalf("source for a chosen place = %+v, want one that refreshes every 15 minutes", own)
	}
	followed := widgetSources(t, here, fuelOn(nil))[display.FuelSource]
	moved := widgetSources(t, settings.Sources{Location: &weather.Location{Latitude: 52.37, Longitude: 4.9}}, fuelOn(nil))[display.FuelSource]
	if followed.Key == "" || followed.Version == moved.Version {
		t.Fatalf("versions = %q and %q; want a source that restarts when the weather location moves", followed.Version, moved.Version)
	}
}

func TestFuelRequestsAreCapped(t *testing.T) {
	var widgets []display.Widget
	for i := range maxFuelQueries + 5 {
		place := map[string]any{"name": "Town", "latitude": 50.0 + float64(i)/10, "longitude": 5.0}
		widgets = append(widgets, display.Widget{ID: "fuel", Type: "fuel", Settings: map[string]any{"place": place}})
	}
	capped := widgetSources(t, settings.Sources{}, widgets[:maxFuelQueries]...)[display.FuelSource]
	over := widgetSources(t, settings.Sources{}, widgets...)[display.FuelSource]
	if capped.Version == "" || capped.Version != over.Version {
		t.Fatal("places beyond the cap changed the source; the requests would grow without bound")
	}
}

func TestFuelSourceFetchesTheWeatherLocationForWidgetsWithoutAPlace(t *testing.T) {
	body, err := os.ReadFile("../../internal/fuel/testdata/oss.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{settings: &settings.Service{Box: box}, fuel: &fuel.Client{BaseURL: srv.URL, HTTP: srv.Client()}}
	cfg := settings.Sources{Location: &weather.Location{Latitude: 51.76, Longitude: 5.52}}
	source, ok := d.fuelSource(cfg.Location, display.FuelQueries([]display.Dashboard{{Widgets: []display.Widget{fuelOn(map[string]any{"radius": float64(3)})}}}))
	if !ok {
		t.Fatal("no source")
	}

	value, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	lists, ok := value.(map[string][]fuel.Station)
	if !ok || len(lists["here,3,EURO95"]) != 10 || lists["here,3,EURO95"][0].Price != 2.364 {
		t.Fatalf("value = %+v, want the ten cheapest stations around the weather location under the key of a widget without a place", value)
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
