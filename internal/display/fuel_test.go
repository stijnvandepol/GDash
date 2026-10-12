package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func fuelWidgetAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "fuel", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func osPlace() map[string]any {
	return map[string]any{"name": "Oss", "latitude": 51.765, "longitude": 5.518}
}

func TestFuelSettingsAreValidated(t *testing.T) {
	const placeMsg = "widgets[2]: place must be a name of 1 to 100 characters with a latitude from -90 to 90 and a longitude from -180 to 180; choose one with the place search"
	with := func(key string, value any) map[string]any { return map[string]any{key: value} }
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"nothing set follows the weather location", nil, ""},
		{"everything", map[string]any{"place": osPlace(), "radius": float64(10), "fuel": "DIESEL", "maxItems": float64(8)}, ""},
		{"place on the edge of the earth", with("place", map[string]any{"name": "Pole", "latitude": float64(-90), "longitude": float64(180)}), ""},
		{"latitude too high", with("place", map[string]any{"name": "Oss", "latitude": 90.5, "longitude": 5.5}), placeMsg},
		{"latitude too low", with("place", map[string]any{"name": "Oss", "latitude": float64(-91), "longitude": 5.5}), placeMsg},
		{"longitude too high", with("place", map[string]any{"name": "Oss", "latitude": 51.7, "longitude": 180.1}), placeMsg},
		{"longitude too low", with("place", map[string]any{"name": "Oss", "latitude": 51.7, "longitude": float64(-181)}), placeMsg},
		{"no name", with("place", map[string]any{"name": "", "latitude": 51.7, "longitude": 5.5}), placeMsg},
		{"blank name", with("place", map[string]any{"name": "   ", "latitude": 51.7, "longitude": 5.5}), placeMsg},
		{"name of 101 characters", with("place", map[string]any{"name": strings.Repeat("x", 101), "latitude": 51.7, "longitude": 5.5}), placeMsg},
		{"name of 100 characters", with("place", map[string]any{"name": strings.Repeat("x", 100), "latitude": 51.7, "longitude": 5.5}), ""},
		{"name with a control character", with("place", map[string]any{"name": "Os\x00s", "latitude": 51.7, "longitude": 5.5}), placeMsg},
		{"no longitude", with("place", map[string]any{"name": "Oss", "latitude": 51.7}), placeMsg},
		{"latitude as text", with("place", map[string]any{"name": "Oss", "latitude": "51.7", "longitude": 5.5}), placeMsg},
		{"an extra field", with("place", map[string]any{"name": "Oss", "latitude": 51.7, "longitude": 5.5, "url": "http://x"}), placeMsg},
		{"place as text", with("place", "Oss"), placeMsg},
		{"radius 1", with("radius", float64(1)), ""},
		{"radius 25", with("radius", float64(25)), ""},
		{"radius 0", with("radius", float64(0)), "widgets[2]: radius must be a whole number from 1 to 25"},
		{"radius 26", with("radius", float64(26)), "widgets[2]: radius must be a whole number from 1 to 25"},
		{"radius fraction", with("radius", 2.5), "widgets[2]: radius must be a whole number from 1 to 25"},
		{"every fuel", with("fuel", "AUTOGAS"), ""},
		{"unknown fuel", with("fuel", "EURO100"), "widgets[2]: fuel must be one of: EURO95, EURO98, DIESEL, AUTOGAS, CNG"},
		{"fuel in lower case", with("fuel", "diesel"), "widgets[2]: fuel must be one of: EURO95, EURO98, DIESEL, AUTOGAS, CNG"},
		{"fuel as number", with("fuel", float64(95)), "widgets[2]: fuel must be one of: EURO95, EURO98, DIESEL, AUTOGAS, CNG"},
		{"ten stations", with("maxItems", float64(10)), ""},
		{"eleven stations", with("maxItems", float64(11)), "widgets[2]: maxItems must be a whole number from 1 to 10"},
		{"unknown setting", with("brand", "Shell"), `widgets[2]: unknown setting "brand"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, fuelWidgetAt("fuel", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFuelWidgetHasItsOwnSourceAndSize(t *testing.T) {
	d := Normalize(Dashboard{Widgets: []Widget{fuelWidgetAt("fuel", nil)}})
	if d.Widgets[0].Source != FuelSource {
		t.Fatalf("source = %q, want %q", d.Widgets[0].Source, FuelSource)
	}
	small := valid()
	small.Widgets = append(small.Widgets, Widget{ID: "fuel", Type: "fuel", X: 0, Y: 5, W: 5, H: 4})
	if got := fields(ValidateDashboard(small)); got != "widgets[2]: a fuel widget needs at least 6x4 cells" {
		t.Fatalf("errors = %q", got)
	}
}

func TestFuelQueriesAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{
		fuelWidgetAt("a", map[string]any{"place": osPlace()}),
		fuelWidgetAt("b", nil),
		{Type: "text", Settings: map[string]any{"fuel": "ignored"}},
	}}
	b := Dashboard{Widgets: []Widget{
		// The same town to the metre as the first, and a different fuel.
		fuelWidgetAt("c", map[string]any{"place": map[string]any{"name": "Oss centre", "latitude": 51.7651, "longitude": 5.5179}}),
		fuelWidgetAt("d", map[string]any{"place": osPlace(), "fuel": "DIESEL", "radius": float64(10)}),
		fuelWidgetAt("e", map[string]any{"fuel": "DIESEL"}),
		fuelWidgetAt("f", map[string]any{"fuel": "nonsense"}),
		fuelWidgetAt("g", map[string]any{"radius": float64(99)}),
	}}

	got := strings.Join(FuelKeys([]Dashboard{a, b}), " ")
	want := "51.765,5.518,10,DIESEL 51.765,5.518,5,EURO95 here,5,DIESEL here,5,EURO95"
	if got != want {
		t.Fatalf("FuelKeys = %q, want %q", got, want)
	}
	for _, q := range FuelQueries([]Dashboard{a}) {
		if !q.Here && (q.Area.Latitude != 51.765 || q.Area.Longitude != 5.518 || q.Area.RadiusKm != 5) {
			t.Errorf("query %+v: want the place rounded to about 100 m and the default radius", q)
		}
	}
}

func TestEachDisplayGetsOnlyItsOwnFuelLists(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{fuelWidgetAt("fuel", map[string]any{"place": osPlace()})}})
	office := Normalize(Dashboard{ID: "office", Widgets: []Widget{fuelWidgetAt("fuel", map[string]any{"fuel": "DIESEL"})}})
	data := map[string]datahub.Entry{FuelSource: {Value: []byte(
		`{"51.765,5.518,5,EURO95":[{"name":"A"}],"here,5,DIESEL":[{"name":"B"}],"here,5,EURO95":[{"name":"C"}]}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "office": {Dashboard: office}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[FuelSource].Value); got != `{"51.765,5.518,5,EURO95":[{"name":"A"}]}` {
		t.Fatalf("hall received %s, want only its own list", got)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "office")).Data[FuelSource].Value); got != `{"here,5,DIESEL":[{"name":"B"}]}` {
		t.Fatalf("office received %s, want only its own list", got)
	}
}
