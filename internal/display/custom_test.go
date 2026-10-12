package display

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/validation"
)

func customTile(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: CustomWidget, X: 0, Y: 0, W: 4, H: 3, Settings: settings}
}

func valueSettings(source int, path string) map[string]any {
	return map[string]any{"source": float64(source), "mode": "value", "path": path}
}

func TestCustomWidgetValidation(t *testing.T) {
	ok := Dashboard{ID: "d", Name: "D", Theme: defaultTheme, Widgets: []Widget{customTile("c", valueSettings(1, "data.temp"))}}
	if err := ValidateDashboard(Normalize(ok)); err != nil {
		t.Fatalf("a valid Custom widget: %v", err)
	}
	for settings, want := range map[string]map[string]any{
		"source: choose an API": {"mode": "value", "path": "a"},
		"path: ":                {"source": 1.0, "mode": "value", "path": "a["},
		"rules[0].when: ":       {"source": 1.0, "mode": "value", "path": "a", "rules": []any{map[string]any{"when": "value >", "tone": "ok"}}},
	} {
		d := Dashboard{ID: "d", Name: "D", Theme: defaultTheme, Widgets: []Widget{customTile("c", want)}}
		err := ValidateDashboard(Normalize(d))
		fields := validation.Fields(err)
		if len(fields) != 1 || fields[0].Field != "widgets[0]" || !strings.HasPrefix(fields[0].Message, settings) {
			t.Errorf("%v: %v, want a message starting %q", want, fields, settings)
		}
	}
}

func TestCustomWidgetsGetOnlyTheirOwnValues(t *testing.T) {
	answer := `{"temp": 21.5, "door": "open", "code": "1234"}`
	data := map[string]datahub.Entry{
		custom.SourceKey(1): {Value: []byte(answer), FetchedAt: now, StaleAfter: now.Add(time.Hour)},
	}
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{customTile("c", valueSettings(1, "temp"))}})
	kitchen := Normalize(Dashboard{ID: "kitchen", Widgets: []Widget{customTile("c", valueSettings(1, "door"))}})
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "kitchen": {Dashboard: kitchen}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snapshot := decode(t, <-b.Subscribe(ctx, "hall"))
	if _, raw := snapshot.Data[custom.SourceKey(1)]; raw {
		t.Fatal("a screen received the raw answer of a custom source")
	}
	var shown map[string]custom.Shown
	if err := json.Unmarshal(snapshot.Data[CustomSource].Value, &shown); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || shown["hall/c"].Value != 21.5 || shown["hall/c"].State != custom.StateOK {
		t.Fatalf("hall received %+v, want only its temperature", shown)
	}
	all, _ := json.Marshal(snapshot)
	if strings.Contains(string(all), "open") || strings.Contains(string(all), "1234") {
		t.Fatalf("hall received values it does not show: %s", all)
	}
}

func TestCustomWidgetStates(t *testing.T) {
	data := map[string]datahub.Entry{
		custom.SourceKey(1): {},
		custom.SourceKey(2): {Failing: true},
		custom.SourceKey(3): {Value: []byte(`{"a":1}`), FetchedAt: now, StaleAfter: now.Add(-time.Minute), Failing: true},
	}
	d := Normalize(Dashboard{ID: "d", Widgets: []Widget{
		customTile("loading", valueSettings(1, "a")), customTile("failing", valueSettings(2, "a")),
		customTile("stale", valueSettings(3, "a")), customTile("gone", valueSettings(4, "a")),
	}})
	entry, ok := customEntry([]Dashboard{d}, data, now)
	if !ok {
		t.Fatal("no entry for Custom widgets")
	}
	var shown map[string]custom.Shown
	if err := json.Unmarshal(entry.Value, &shown); err != nil {
		t.Fatal(err)
	}
	if shown["d/loading"].State != custom.StateLoading || shown["d/failing"].State != custom.StateFailing ||
		shown["d/gone"].State != custom.StateMissing || !shown["d/stale"].Stale || shown["d/stale"].Value != 1.0 {
		t.Fatalf("shown = %+v", shown)
	}
	if _, ok := customEntry([]Dashboard{{ID: "empty"}}, data, now); ok {
		t.Fatal("an entry for dashboards without Custom widgets")
	}
}

func TestCustomSourceIDsAndTargets(t *testing.T) {
	announcing := valueSettings(2, "a")
	announcing["rules"] = []any{map[string]any{"when": "value > 1", "message": "High", "announce": true}}
	invalid := map[string]any{"source": 9.0}
	dashboards := []Dashboard{
		{ID: "one", Widgets: []Widget{customTile("x", valueSettings(3, "a")), customTile("y", announcing)}},
		{ID: "two", Widgets: []Widget{customTile("x", valueSettings(2, "a")), customTile("z", invalid)}},
	}
	if got := CustomSourceIDs(dashboards); !slices.Equal(got, []int64{2, 3}) {
		t.Fatalf("CustomSourceIDs = %v, want [2 3]", got)
	}
	data := map[string]datahub.Entry{custom.SourceKey(2): {Value: []byte(`{"a":5}`), FetchedAt: now}}
	targets := CustomTargets(dashboards, data, now)
	if len(targets) != 1 || targets[0].Key != "one/y" {
		t.Fatalf("CustomTargets = %+v, want only the announcing widget", targets)
	}
	if got := CustomTargets(dashboards, nil, now); len(got) != 0 {
		t.Fatalf("targets without data = %+v", got)
	}
}
