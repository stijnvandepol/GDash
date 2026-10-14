package display

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func homeyValues(keys ...string) Widget {
	list := make([]any, len(keys))
	for i, k := range keys {
		list[i] = k
	}
	return Widget{ID: "values", Type: "homey", X: 0, Y: 0, W: 4, H: 3, Settings: map[string]any{"capabilities": list}}
}

// whoIsHome is a Who's home widget that looks for users, given as id and
// label in turn.
func whoIsHome(idsAndLabels ...string) Widget {
	var users []any
	for i := 0; i+1 < len(idsAndLabels); i += 2 {
		users = append(users, map[string]any{"id": idsAndLabels[i], "label": idsAndLabels[i+1]})
	}
	return Widget{ID: "home", Type: HomeyPresenceWidget, X: 4, Y: 0, W: 4, H: 3, Settings: map[string]any{"users": users}}
}

func TestEachDisplayGetsOnlyItsOwnHomeyValues(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{homeyValues("d1/measure_temperature")}})
	living := Normalize(Dashboard{ID: "living", Widgets: []Widget{homeyValues("d1/measure_temperature", "d2/locked")}})
	data := map[string]datahub.Entry{HomeySource: {Value: []byte(
		`{"d1/measure_temperature":{"device":"Thermostat","value":21},"d2/locked":{"device":"Front door","value":false}}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "living": {Dashboard: living}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[HomeySource].Value)
	if got != `{"d1/measure_temperature":{"device":"Thermostat","value":21}}` {
		t.Fatalf("hall received %s, want only its temperature", got)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "living")).Data[HomeySource].Value); !strings.Contains(got, "Front door") {
		t.Fatalf("living room received %s, want both values", got)
	}
}

func TestScreensOnlyLearnWhoIsHome(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{whoIsHome("u1", "AN", "u2", "BRAM", "u3", "CS")}})
	data := map[string]datahub.Entry{HomeyUsersSource: {Value: []byte(`{"u1":true,"u2":false,"u4":true}`), FetchedAt: now, StaleAfter: now.Add(5 * time.Minute)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall, Pages: []Dashboard{hall}, PageSeconds: 30}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	msg := <-b.Subscribe(ctx, "hall")
	s := decode(t, msg)

	if got := string(s.Data[HomeyPresenceSource].Value); got != `{"hall/home":["AN"]}` {
		t.Fatalf("presence = %s, want only Anna's label", got)
	}
	if _, sent := s.Data[HomeyUsersSource]; sent {
		t.Fatal("the screen received everyone's presence")
	}
	// Not in the data, not in the layout, not in the pages.
	for _, away := range []string{"BRAM", "CS", "u2", "u3"} {
		if strings.Contains(string(msg), away) {
			t.Fatalf("the snapshot names %q, who is not home: %s", away, msg)
		}
	}
	if !s.Data[HomeyPresenceSource].StaleAfter.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("presence stale after %v, want the users' own", s.Data[HomeyPresenceSource].StaleAfter)
	}
}

func TestPresenceWhileHomeyIsNotReady(t *testing.T) {
	dashboards := []Dashboard{{ID: "hall", Widgets: []Widget{whoIsHome("u1", "AN")}}}
	if _, ok := presenceEntry(dashboards, map[string]datahub.Entry{}); ok {
		t.Fatal("an entry without a Homey connection; the widget should say it is not set up")
	}
	loading, ok := presenceEntry(dashboards, map[string]datahub.Entry{HomeyUsersSource: {Failing: true}})
	if !ok || loading.Value != nil || !loading.Failing {
		t.Fatalf("entry = %+v, %v; want a failing entry without a value", loading, ok)
	}
	broken, _ := presenceEntry(dashboards, map[string]datahub.Entry{HomeyUsersSource: {Value: json.RawMessage(`[1]`)}})
	if broken.Value != nil || !broken.Failing {
		t.Fatalf("entry = %+v; want an unreadable answer to show as failing", broken)
	}
	if _, ok := presenceEntry(nil, map[string]datahub.Entry{HomeyUsersSource: {Value: json.RawMessage(`{}`)}}); ok {
		t.Fatal("an entry without Who's home widgets")
	}
}

func TestPreviewLeavesOutTheUsersToo(t *testing.T) {
	d := Dashboard{ID: "hall", Name: "Hall", Theme: "glass", Widgets: []Widget{whoIsHome("u1", "AN", "u2", "BR")}}
	s, err := PreviewSnapshot(now, d, map[string]datahub.Entry{HomeyUsersSource: {Value: json.RawMessage(`{"u1":false,"u2":true}`)}}, "en", 100)
	if err != nil {
		t.Fatal(err)
	}
	if s.Dashboard.Widgets[0].Settings != nil || string(s.Data[HomeyPresenceSource].Value) != `{"hall/home":["BR"]}` {
		t.Fatalf("preview = %+v, data %s", s.Dashboard.Widgets[0], s.Data[HomeyPresenceSource].Value)
	}
	if d.Widgets[0].Settings == nil {
		t.Fatal("the preview changed the dashboard it was given")
	}
}

func TestScreensKeepTheTitleOfWhoIsHome(t *testing.T) {
	w := whoIsHome("u1", "AN")
	w.Settings["title"] = "Thuis"
	w.Settings["hideTitle"] = false
	got := screenDashboard(Dashboard{Widgets: []Widget{w}}).Widgets[0].Settings
	if len(got) != 2 || got["title"] != "Thuis" || got["hideTitle"] != false {
		t.Fatalf("settings on the screen = %v, want the title settings without the users", got)
	}
}

func TestHomeyWidgetSettings(t *testing.T) {
	valid := []Widget{
		homeyValues("d1/measure_temperature", "d1/alarm_motion.hall"),
		whoIsHome("u1", "AN", "u2", "Grandma"),
	}
	for _, w := range valid {
		if err := ValidateDashboard(Normalize(Dashboard{ID: "x", Name: "X", Widgets: []Widget{w}})); err != nil {
			t.Errorf("%s: %s", w.Type, fields(err))
		}
	}
	invalid := map[string]Widget{
		"no values":         homeyValues(),
		"a bad key":         homeyValues("d1/../x"),
		"twice the same":    homeyValues("d1/onoff", "d1/onoff"),
		"nine values":       homeyValues("a/1", "a/2", "a/3", "a/4", "a/5", "a/6", "a/7", "a/8", "a/9"),
		"no users":          whoIsHome(),
		"an empty label":    whoIsHome("u1", ""),
		"a long label":      whoIsHome("u1", "Thirteen char"),
		"a label in spaces": whoIsHome("u1", " AN"),
		"the same user":     whoIsHome("u1", "AN", "u1", "AB"),
		"a bad user id":     whoIsHome("u 1", "AN"),
		"settings missing":  {ID: "home", Type: HomeyPresenceWidget, W: 4, H: 3},
	}
	extra := whoIsHome("u1", "AN")
	extra.Settings["users"].([]any)[0].(map[string]any)["name"] = "Anna"
	invalid["a user with more fields"] = extra
	for name, w := range invalid {
		if err := ValidateDashboard(Normalize(Dashboard{ID: "x", Name: "X", Widgets: []Widget{w}})); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestHomeyKeysAndUsersOfDashboards(t *testing.T) {
	dashboards := []Dashboard{
		{ID: "a", Widgets: []Widget{homeyValues("d2/onoff", "d1/onoff"), whoIsHome("u2", "B", "u1", "A")}},
		{ID: "b", Widgets: []Widget{homeyValues("d1/onoff"), whoIsHome("u1", "A")}},
	}
	if got := strings.Join(HomeyCapabilityKeys(dashboards), ","); got != "d1/onoff,d2/onoff" {
		t.Errorf("HomeyCapabilityKeys = %s", got)
	}
	if got := strings.Join(HomeyUserIDs(dashboards), ","); got != "u1,u2" {
		t.Errorf("HomeyUserIDs = %s", got)
	}
}
