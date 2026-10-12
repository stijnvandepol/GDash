package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// departuresWidgetAt returns a Departures widget below the default layout.
func departuresWidgetAt(settings map[string]any) Widget {
	return Widget{ID: "bus", Type: "departures", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func TestDeparturesSettingsAreValidated(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"stop only", map[string]any{"stop": "utrcsw"}, ""},
		{"everything", map[string]any{"stop": "utrcsw", "stopName": "Utrecht, CS Jaarbeurszijde", "maxItems": float64(6)}, ""},
		{"digits are fine", map[string]any{"stop": "30267"}, ""},
		{"no stop", map[string]any{"maxItems": float64(6)}, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"no settings", nil, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"empty stop", map[string]any{"stop": ""}, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"stop with a slash", map[string]any{"stop": "a/b"}, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"several stops", map[string]any{"stop": "utrcsw,utrbus"}, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"stop is a number", map[string]any{"stop": float64(30267)}, "widgets[2]: stop must be a stop area code; choose one with the stop search"},
		{"too many items", map[string]any{"stop": "utrcsw", "maxItems": float64(21)}, "widgets[2]: maxItems must be a whole number from 1 to 20"},
		{"empty name", map[string]any{"stop": "utrcsw", "stopName": ""}, "widgets[2]: stopName must be 1 to 100 characters of text"},
		{"unknown setting", map[string]any{"stop": "utrcsw", "line": "28"}, `widgets[2]: unknown setting "line"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, departuresWidgetAt(tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeparturesWidgetHasItsOwnSource(t *testing.T) {
	d := Normalize(Dashboard{Widgets: []Widget{departuresWidgetAt(map[string]any{"stop": "utrcsw"})}})
	if d.Widgets[0].Source != DeparturesSource {
		t.Fatalf("source = %q, want %q", d.Widgets[0].Source, DeparturesSource)
	}
}

func TestStopCodesAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{
		departuresWidgetAt(map[string]any{"stop": "utrcsw"}),
		{Type: "text", Settings: map[string]any{"stop": "ignored"}},
	}}
	b := Dashboard{Widgets: []Widget{
		departuresWidgetAt(map[string]any{"stop": "05003"}),
		departuresWidgetAt(map[string]any{"stop": "utrcsw"}),
		departuresWidgetAt(map[string]any{"stop": "not valid"}),
		departuresWidgetAt(nil),
	}}
	if got := strings.Join(StopCodes([]Dashboard{a, b}), ","); got != "05003,utrcsw" {
		t.Fatalf("StopCodes = %s, want the stops of valid Departures widgets, sorted once", got)
	}
}

func TestEachDisplayGetsOnlyItsOwnStops(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{departuresWidgetAt(map[string]any{"stop": "utrcsw"})}})
	office := Normalize(Dashboard{ID: "office", Widgets: []Widget{departuresWidgetAt(map[string]any{"stop": "utrbus"})}})
	data := map[string]datahub.Entry{DeparturesSource: {Value: []byte(
		`{"utrcsw":[{"line":"28"}],"utrbus":[{"line":"22"}],"asdcs":[]}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "office": {Dashboard: office}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[DeparturesSource].Value); got != `{"utrcsw":[{"line":"28"}]}` {
		t.Fatalf("hall received %s, want only its own stop", got)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "office")).Data[DeparturesSource].Value); got != `{"utrbus":[{"line":"22"}]}` {
		t.Fatalf("office received %s, want only its own stop", got)
	}
}
