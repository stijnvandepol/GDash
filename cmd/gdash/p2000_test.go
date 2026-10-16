package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/p2000"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
)

func p2000On(region string) display.Widget {
	return display.Widget{ID: "p2000-" + region, Type: "p2000", Settings: map[string]any{"region": region}}
}

func TestP2000IsFetchedOnlyForWidgetsAndAtMostTwentyFeeds(t *testing.T) {
	body, err := os.ReadFile("../../internal/p2000/testdata/region-brandweer.rss")
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{
		settings: &settings.Service{Box: box},
		p2000:    &p2000.Client{BaseURL: srv.URL, HTTP: srv.Client()},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if got := widgetSources(t, settings.Sources{}); got[display.P2000Source].Key != "" {
		t.Fatal("a P2000 source without a P2000 widget")
	}
	// Six regions with four disciplines each: 24 feeds, 4 too many.
	var widgets []display.Widget
	for _, r := range []string{"twente", "zeeland", "utrecht", "drenthe", "flevoland", "groningen"} {
		widgets = append(widgets, p2000On(r))
	}
	source, ok := d.p2000Source(display.P2000Feeds([]display.Dashboard{{Widgets: widgets}}))
	if !ok {
		t.Fatal("no source")
	}
	value, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := value.(p2000.Value)
	if len(v.Feeds) != 20 || requests.Load() != 20 {
		t.Fatalf("feeds = %d, requests = %d, want 20 of each", len(v.Feeds), requests.Load())
	}
	if want := []string{"zeeland/brandweer", "zeeland/knrm", "zeeland/politie", "zeeland/trauma"}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want the last four by key, %v", v.Skipped, want)
	}
}
