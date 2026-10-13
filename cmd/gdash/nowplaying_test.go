package main

import (
	"slices"
	"testing"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
)

const (
	speakerID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tvID      = "cccccccccccccccccccccccccccccccc"
)

func nowPlayingDashboard(settings map[string]any) display.Dashboard {
	return display.Dashboard{Widgets: []display.Widget{{ID: "music", Type: display.NowPlayingWidget, Settings: settings}}}
}

func TestNowPlayingIsFollowedOnlyWhileADashboardShowsIt(t *testing.T) {
	tvs := []displays.Display{{ID: "living", CastDeviceID: tvID}, {ID: "tablet"}}
	if want := nowPlayingWant([]display.Dashboard{display.DefaultDashboard(false)}, tvs); want.Any || want.IDs != nil || want.Exclude != nil {
		t.Fatalf("without the widget: %+v, want nothing followed", want)
	}
	if sources := nowPlayingSource([]display.Dashboard{display.DefaultDashboard(false)}); sources != nil {
		t.Fatalf("without the widget the hub gets %+v", sources)
	}

	want := nowPlayingWant([]display.Dashboard{nowPlayingDashboard(map[string]any{"devices": []any{speakerID, tvID}})}, tvs)
	if want.Any || !slices.Equal(want.IDs, []string{speakerID, tvID}) || !slices.Equal(want.Exclude, []string{tvID}) {
		t.Fatalf("want = %+v; the speaker, and the TV only as one to leave alone", want)
	}
	if want := nowPlayingWant([]display.Dashboard{nowPlayingDashboard(nil)}, tvs); !want.Any || !slices.Equal(want.Exclude, []string{tvID}) {
		t.Fatalf("a widget for any device: %+v", want)
	}

	sources := nowPlayingSource([]display.Dashboard{nowPlayingDashboard(nil)})
	if len(sources) != 1 || sources[0].Key != display.NowPlayingSource || sources[0].Fetch != nil {
		t.Fatalf("sources = %+v, want one pushed source", sources)
	}
}
