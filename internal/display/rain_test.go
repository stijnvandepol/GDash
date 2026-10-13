package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func rainWidgetAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "rain", X: 0, Y: 5, W: 5, H: 3, Settings: settings}
}

func TestRainSettingsAreValidated(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"nothing set follows the weather location", nil, ""},
		{"a place", map[string]any{"place": osPlace()}, ""},
		{"place as text", map[string]any{"place": "Oss"}, "widgets[2]: place must be a name of 1 to 100 characters with a latitude from -90 to 90 and a longitude from -180 to 180; choose one with the place search"},
		{"unknown setting", map[string]any{"radius": float64(5)}, `widgets[2]: unknown setting "radius"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, rainWidgetAt("rain", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
	small := valid()
	small.Widgets = append(small.Widgets, Widget{ID: "rain", Type: "rain", X: 0, Y: 5, W: 4, H: 3})
	if got := fields(ValidateDashboard(small)); got != "widgets[2]: a rain widget needs at least 5x3 cells" {
		t.Fatalf("errors = %q", got)
	}
	if d := Normalize(Dashboard{Widgets: []Widget{rainWidgetAt("rain", nil)}}); d.Widgets[0].Source != RainSource {
		t.Fatalf("source = %q, want %q", d.Widgets[0].Source, RainSource)
	}
}

func TestRainQueriesAreCollectedOncePerRoundedPlace(t *testing.T) {
	a := Dashboard{Widgets: []Widget{
		rainWidgetAt("a", map[string]any{"place": osPlace()}),
		rainWidgetAt("b", nil),
		// Oss again, a few hundred metres off: the same request.
		rainWidgetAt("c", map[string]any{"place": map[string]any{"name": "Oss west", "latitude": 51.769, "longitude": 5.515}}),
		rainWidgetAt("d", map[string]any{"place": "nonsense"}),
		fuelWidgetAt("e", map[string]any{"place": osPlace()}),
	}}
	if got := strings.Join(RainKeys([]Dashboard{a}), " "); got != "51.77,5.52 here" {
		t.Fatalf("RainKeys = %q, want Oss once and the weather location", got)
	}
}

func TestEachDisplayGetsOnlyItsOwnRain(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{rainWidgetAt("rain", map[string]any{"place": osPlace()})}})
	data := map[string]datahub.Entry{RainSource: {Value: []byte(`{"51.77,5.52":{"source":"buienradar"},"here":{"source":"open-meteo"}}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[RainSource].Value); got != `{"51.77,5.52":{"source":"buienradar"}}` {
		t.Fatalf("hall received %s, want only its own place", got)
	}
}
