package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

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

func TestHomeyThatSendsItsDataIsWaitedFor(t *testing.T) {
	cfg := settings.Sources{HomeyPushed: true}
	// Without widgets too: the editor lists values from the snapshot.
	for _, got := range []map[string]datahub.Source{homeySources(t, cfg), homeySources(t, cfg, homeyValues("d1/onoff"), whoIsHome("u1"))} {
		if len(got) != 3 {
			t.Fatalf("sources = %v, want the snapshot, values and users", got)
		}
		for key, src := range got {
			if src.Fetch != nil || src.FreshFor != 3*time.Minute {
				t.Errorf("%s = %+v, want it pushed and fresh for three minutes", key, src)
			}
		}
	}
	polled := homeySources(t, settings.Sources{Homey: &homey.Connection{URL: "http://192.168.1.20", Key: "made-up"}}, homeyValues("d1/onoff"))
	if polled[display.HomeySource].Version == homeySources(t, cfg)[display.HomeySource].Version {
		t.Fatal("switching to push mode keeps the polled values")
	}
}

func TestHomeySnapshotSurvivesARestart(t *testing.T) {
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cache, err := datahub.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := func() *dataSources {
		d := &dataSources{hub: datahub.New(cache, logger, func() {}), settings: &settings.Service{Box: box}}
		d.hub.Configure(ctx, d.homeySources(settings.Sources{HomeyPushed: true}, nil))
		return d
	}
	snapshot, err := homey.ParseSnapshot([]byte(`{"devices": [{"id": "d1", "name": "Lamp", "caps": {"onoff": {"value": true}}}],
		"users": [{"id": "u1", "name": "Anna", "present": true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := start().receiveHomey(snapshot); err != nil {
		t.Fatal(err)
	}
	entries := start().hub.Entries()
	if got := string(entries[display.HomeySource].Value); !strings.Contains(got, `"d1/onoff":{"device":"Lamp"`) {
		t.Errorf("values after a restart = %s", got)
	}
	if got := string(entries[display.HomeyUsersSource].Value); got != `{"u1":true}` {
		t.Errorf("presence after a restart = %s", got)
	}
	if len(entries[display.HomeySnapshotSource].Value) == 0 {
		t.Error("the snapshot was not kept on disk")
	}
}
