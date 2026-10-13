package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/traffic"
)

func trafficWidgetAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "traffic", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func TestTrafficSettingsAreValidated(t *testing.T) {
	const roadsMsg = "widgets[2]: roads must be 1 to 10 roads such as A2 or N279, each once"
	const showMsg = "widgets[2]: show must be at least 1 of: jams, roadworks, speedchecks, cameras, totals, each once"
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"nothing set watches around the weather location", nil, ""},
		{"roads", map[string]any{"roads": []any{"A2", "A50", "N279"}, "show": []any{"jams", "cameras"}, "maxItems": float64(10)}, ""},
		{"a radius", map[string]any{"radius": float64(100)}, ""},
		{"roads and a radius", map[string]any{"roads": []any{"A2"}, "radius": float64(10)},
			"widgets[2]: a traffic widget watches roads or a radius around the weather location, not both"},
		{"no roads", map[string]any{"roads": []any{}}, roadsMsg},
		{"a road in lower case", map[string]any{"roads": []any{"a2"}}, roadsMsg},
		{"a European road", map[string]any{"roads": []any{"E25"}}, roadsMsg},
		{"a road twice", map[string]any{"roads": []any{"A2", "A2"}}, roadsMsg},
		{"eleven roads", map[string]any{"roads": []any{"A1", "A2", "A4", "A5", "A6", "A7", "A8", "A9", "A10", "A12", "A13"}}, roadsMsg},
		{"roads as text", map[string]any{"roads": "A2"}, roadsMsg},
		{"radius 101", map[string]any{"radius": float64(101)}, "widgets[2]: radius must be a whole number from 1 to 100"},
		{"nothing to show", map[string]any{"show": []any{}}, showMsg},
		{"an unknown kind", map[string]any{"show": []any{"weather"}}, showMsg},
		{"a kind twice", map[string]any{"show": []any{"jams", "jams"}}, showMsg},
		{"eleven items", map[string]any{"maxItems": float64(11)}, "widgets[2]: maxItems must be a whole number from 1 to 10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, trafficWidgetAt("traffic", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrafficQueriesAreCollectedOnce(t *testing.T) {
	d := Dashboard{Widgets: []Widget{
		trafficWidgetAt("a", map[string]any{"roads": []any{"A50", "A2"}}),
		trafficWidgetAt("b", map[string]any{"roads": []any{"A2", "A50"}, "show": []any{"cameras"}}),
		trafficWidgetAt("c", nil),
		trafficWidgetAt("d", map[string]any{"radius": float64(25)}),
		trafficWidgetAt("e", map[string]any{"radius": float64(10)}),
		trafficWidgetAt("f", map[string]any{"roads": "nonsense"}),
	}}
	if got := strings.Join(TrafficKeys([]Dashboard{d}), " "); got != "near:10 near:25 roads:A2,A50" {
		t.Fatalf("TrafficKeys = %q", got)
	}
	if !TrafficShows([]Dashboard{d}, traffic.ShowCameras) || !TrafficShows([]Dashboard{d}, traffic.ShowJams) {
		t.Error("cameras and the default jams are shown")
	}
	only := Dashboard{Widgets: []Widget{trafficWidgetAt("a", map[string]any{"show": []any{"jams"}})}}
	if TrafficShows([]Dashboard{only}, traffic.ShowCameras) {
		t.Error("no widget shows cameras")
	}
}

func TestEachDisplayGetsOnlyItsOwnTraffic(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{trafficWidgetAt("t", map[string]any{"roads": []any{"A2"}})}})
	office := Normalize(Dashboard{ID: "office", Widgets: []Widget{trafficWidgetAt("t", nil)}})
	data := map[string]datahub.Entry{TrafficSource: {Value: []byte(`{"roads:A2":{"jams":[]},"near:25":{"jams":[]},"roads:A50":{"jams":[]}}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "office": {Dashboard: office}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[TrafficSource].Value); got != `{"roads:A2":{"jams":[]}}` {
		t.Fatalf("hall received %s", got)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "office")).Data[TrafficSource].Value); got != `{"near:25":{"jams":[]}}` {
		t.Fatalf("office received %s", got)
	}
}

func TestMeasuredWeatherOnlyGoesWhereItIsShown(t *testing.T) {
	weatherAt := func(settings map[string]any) Widget {
		return Widget{ID: "w", Type: "weather", X: 0, Y: 5, W: 4, H: 4, Settings: settings}
	}
	measured := Normalize(Dashboard{ID: "measured", Widgets: []Widget{weatherAt(map[string]any{"current": "knmi"})}})
	model := Normalize(Dashboard{ID: "model", Widgets: []Widget{weatherAt(nil)}})
	data := map[string]datahub.Entry{
		WeatherSource: {Value: []byte(`{"temperatureC":12}`)},
		KNMISource:    {Value: []byte(`{"station":"Volkel","temperatureC":12.6}`)},
	}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"measured": {Dashboard: measured}, "model": {Dashboard: model}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, ok := decode(t, <-b.Subscribe(ctx, "measured")).Data[KNMISource]; !ok {
		t.Error("the measured dashboard has no KNMI observation")
	}
	if _, ok := decode(t, <-b.Subscribe(ctx, "model")).Data[KNMISource]; ok {
		t.Error("the model dashboard got the KNMI observation")
	}

	d := valid()
	d.Widgets = append(d.Widgets, weatherAt(map[string]any{"current": "measured"}))
	if got := fields(ValidateDashboard(d)); got != "widgets[2]: current must be one of: model, knmi" {
		t.Errorf("errors = %q", got)
	}
}
