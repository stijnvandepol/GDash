package main

import (
	"fmt"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
)

func homeySources(t *testing.T, cfg settings.Sources, widgets ...display.Widget) map[string]datahub.Source {
	t.Helper()
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]datahub.Source{}
	for _, src := range (&dataSources{settings: &settings.Service{Box: box}}).homeySources(cfg, []display.Dashboard{{ID: "a", Widgets: widgets}}) {
		out[src.Key] = src
	}
	return out
}

func homeyValues(keys ...any) display.Widget {
	return display.Widget{ID: "values", Type: "homey", Settings: map[string]any{"capabilities": keys}}
}

func whoIsHome(ids ...string) display.Widget {
	var users []any
	for _, id := range ids {
		users = append(users, map[string]any{"id": id, "label": "X"})
	}
	return display.Widget{ID: "home", Type: display.HomeyPresenceWidget, Settings: map[string]any{"users": users}}
}

func TestHomeyIsFetchedOnlyWhenShown(t *testing.T) {
	cfg := settings.Sources{Homey: &homey.Connection{URL: "http://192.168.1.20", Key: "made-up"}}
	if got := homeySources(t, cfg); len(got) != 0 {
		t.Fatalf("sources without Homey widgets = %v, want none", got)
	}
	if got := homeySources(t, settings.Sources{}, homeyValues("d1/onoff"), whoIsHome("u1")); len(got) != 0 {
		t.Fatalf("sources without a connection = %v, want none", got)
	}
	values := homeySources(t, cfg, homeyValues("d1/onoff"))
	if _, ok := values[display.HomeySource]; !ok || len(values) != 1 {
		t.Fatalf("sources for a Homey widget = %v, want the values only", values)
	}
	people := homeySources(t, cfg, whoIsHome("u1"))
	if _, ok := people[display.HomeyUsersSource]; !ok || len(people) != 1 {
		t.Fatalf("sources for a Who's home widget = %v, want the users only", people)
	}
	other := homeySources(t, cfg, homeyValues("d1/onoff", "d2/onoff"), whoIsHome("u1", "u2"))
	if other[display.HomeySource].Version == values[display.HomeySource].Version ||
		other[display.HomeyUsersSource].Version == people[display.HomeyUsersSource].Version {
		t.Fatal("the sources did not restart when the widgets changed")
	}
}

func TestHomeyKeysAreCapped(t *testing.T) {
	cfg := settings.Sources{Homey: &homey.Connection{URL: "http://192.168.1.20", Key: "made-up"}}
	// Six widgets of eight values each: more than the cap.
	var widgets []display.Widget
	for w := range 6 {
		var keys []any
		for k := range display.MaxHomeyCapabilities {
			keys = append(keys, fmt.Sprintf("d%d/c%d", w, k))
		}
		widget := homeyValues(keys...)
		widget.ID = fmt.Sprintf("w%d", w)
		widgets = append(widgets, widget)
	}
	all := homeySources(t, cfg, widgets...)[display.HomeySource]
	fewer := homeySources(t, cfg, widgets[:5]...)[display.HomeySource]
	if all.Version != fewer.Version {
		t.Fatal("values beyond the cap changed the source; what is kept would grow without bound")
	}
}
