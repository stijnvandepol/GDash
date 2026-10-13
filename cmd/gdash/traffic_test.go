package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/speedcameras"
	"github.com/stijnvandepol/gdash/internal/traffic"
	"github.com/stijnvandepol/gdash/internal/weather"
)

func trafficOn(settings map[string]any) display.Widget {
	return display.Widget{ID: "traffic", Type: "traffic", Settings: settings}
}

// trafficStandIns serves the recorded ANWB and Overpass answers, counts the
// requests to each, and returns data sources that read them.
func trafficStandIns(t *testing.T) (*dataSources, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	incidents, err := os.ReadFile("../../internal/traffic/testdata/incidents.json")
	if err != nil {
		t.Fatal(err)
	}
	cameras, err := os.ReadFile("../../internal/speedcameras/testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	var anwbCalls, overpassCalls atomic.Int32
	anwb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		anwbCalls.Add(1)
		_, _ = w.Write(incidents)
	}))
	overpass := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		overpassCalls.Add(1)
		_, _ = w.Write(cameras)
	}))
	t.Cleanup(anwb.Close)
	t.Cleanup(overpass.Close)
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{
		settings: &settings.Service{Box: box},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		traffic:  &traffic.Guard{Client: traffic.Client{BaseURL: anwb.URL, HTTP: anwb.Client()}, Now: time.Now},
		cameras: &speedcameras.Store{
			Client: speedcameras.Client{BaseURL: overpass.URL, HTTP: overpass.Client()},
			Path:   filepath.Join(t.TempDir(), "speedcameras.json"), Now: time.Now,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}
	return d, &anwbCalls, &overpassCalls
}

// fetchAll runs every source once, as the hub would.
func fetchAll(t *testing.T, d *dataSources, cfg settings.Sources, widgets ...display.Widget) map[string]any {
	t.Helper()
	values := map[string]any{}
	for _, src := range d.trafficSources(cfg, []display.Dashboard{{Widgets: widgets}}) {
		value, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatalf("fetch %s: %v", src.Key, err)
		}
		values[src.Key] = value
	}
	return values
}

func TestTrafficIsNotAskedBeforeTheAcknowledgement(t *testing.T) {
	d, anwbCalls, overpassCalls := trafficStandIns(t)
	roads := trafficOn(map[string]any{"roads": []any{"A2"}})
	if got := fetchAll(t, d, settings.Sources{}, roads); len(got) != 0 {
		t.Fatalf("sources while traffic info is off = %v, want none", got)
	}
	// A widget that also shows cameras gets them, still without the ANWB.
	withCameras := trafficOn(map[string]any{"roads": []any{"A2"}, "show": []any{"jams", "cameras"}})
	values := fetchAll(t, d, settings.Sources{}, withCameras)
	if anwbCalls.Load() != 0 {
		t.Fatalf("%d requests to the ANWB while traffic info is off", anwbCalls.Load())
	}
	if _, ok := values[display.CamerasSource]; !ok {
		t.Fatalf("values = %v, want a camera list", values)
	}
	// The first refresh fetched the camera list itself, once.
	result := values[display.TrafficSource].(map[string]traffic.Result)["roads:A2"]
	if result.Incidents || result.CameraCount != 2 || overpassCalls.Load() != 1 {
		t.Fatalf("result while traffic info is off = %+v after %d queries, want two A2 cameras, no incidents, one query", result, overpassCalls.Load())
	}

	on := fetchAll(t, d, settings.Sources{TrafficInfo: true}, roads)[display.TrafficSource].(map[string]traffic.Result)["roads:A2"]
	if anwbCalls.Load() != 1 || !on.Incidents || len(on.Jams) != 2 {
		t.Fatalf("after turning it on: %d requests, %+v", anwbCalls.Load(), on)
	}
}

func TestTrafficWidgetsShareOneRequest(t *testing.T) {
	d, anwbCalls, overpassCalls := trafficStandIns(t)
	cfg := settings.Sources{TrafficInfo: true, Location: &weather.Location{Latitude: 51.03, Longitude: 5.81}}
	widgets := []display.Widget{
		trafficOn(map[string]any{"roads": []any{"A2", "A50"}}),
		trafficOn(nil),
		trafficOn(map[string]any{"radius": float64(10)}),
	}
	for range 3 {
		got := fetchAll(t, d, cfg, widgets...)[display.TrafficSource].(map[string]traffic.Result)
		if len(got) != 3 {
			t.Fatalf("results = %v, want one per query", got)
		}
	}
	if anwbCalls.Load() != 1 || overpassCalls.Load() != 0 {
		t.Fatalf("%d ANWB and %d Overpass requests, want 1 and 0", anwbCalls.Load(), overpassCalls.Load())
	}
}

func TestTrafficNearHomeNeedsALocation(t *testing.T) {
	d, _, _ := trafficStandIns(t)
	if got := fetchAll(t, d, settings.Sources{TrafficInfo: true}, trafficOn(nil)); len(got) != 0 {
		t.Fatalf("sources = %v, want none without a weather location", got)
	}
}

func TestKNMIIsFetchedOnlyWhenShownWithAKey(t *testing.T) {
	measured := display.Widget{ID: "w", Type: "weather", Settings: map[string]any{"current": "knmi"}}
	here := &weather.Location{Latitude: 51.66, Longitude: 5.62}
	tests := []struct {
		name    string
		cfg     settings.Sources
		widgets []display.Widget
		want    bool
	}{
		{"everything", settings.Sources{Location: here, KNMIKey: "made-up"}, []display.Widget{measured}, true},
		{"no key", settings.Sources{Location: here}, []display.Widget{measured}, false},
		{"no location", settings.Sources{KNMIKey: "made-up"}, []display.Widget{measured}, false},
		{"model values", settings.Sources{Location: here, KNMIKey: "made-up"}, []display.Widget{{ID: "w", Type: "weather"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := widgetSources(t, tt.cfg, tt.widgets...)[display.KNMISource]
			if ok != tt.want {
				t.Fatalf("source = %v, want %v", ok, tt.want)
			}
		})
	}
	a := widgetSources(t, settings.Sources{Location: here, KNMIKey: "made-up-one"}, measured)[display.KNMISource]
	b := widgetSources(t, settings.Sources{Location: here, KNMIKey: "made-up-two"}, measured)[display.KNMISource]
	if a.Version == b.Version || a.Interval != 10*time.Minute {
		t.Fatalf("versions %q and %q; want a source that restarts with a new key", a.Version, b.Version)
	}
}
