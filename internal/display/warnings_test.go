package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func warningsWidgetAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "warnings", X: 0, Y: 5, W: 6, H: 3, Settings: settings}
}

func TestWarningsSettingsAreValidated(t *testing.T) {
	const regionMsg = "widgets[2]: region must be one of: Groningen, Friesland, Drenthe, Overijssel, Flevoland, Gelderland, Utrecht, Noord-Holland, Zuid-Holland, Zeeland, Noord-Brabant, Limburg, Waddeneilanden"
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"nothing set follows the weather location", nil, ""},
		{"a province that announces", map[string]any{"region": "Noord-Brabant", "announce": true}, ""},
		{"announce off", map[string]any{"announce": false}, ""},
		{"unknown province", map[string]any{"region": "Brabant"}, regionMsg},
		{"province in lower case", map[string]any{"region": "utrecht"}, regionMsg},
		{"announce as text", map[string]any{"announce": "yes"}, "widgets[2]: announce must be true or false"},
		{"announce as number", map[string]any{"announce": float64(1)}, "widgets[2]: announce must be true or false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, warningsWidgetAt("warnings", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
	small := valid()
	small.Widgets = append(small.Widgets, Widget{ID: "warnings", Type: "warnings", X: 0, Y: 5, W: 5, H: 3})
	if got := fields(ValidateDashboard(small)); got != "widgets[2]: a warnings widget needs at least 6x3 cells" {
		t.Fatalf("errors = %q", got)
	}
}

func TestWarningsQueriesAndAnnouncers(t *testing.T) {
	dashboards := []Dashboard{{Widgets: []Widget{
		warningsWidgetAt("a", map[string]any{"region": "Zeeland", "announce": true}),
		warningsWidgetAt("b", nil),
		warningsWidgetAt("c", map[string]any{"region": "Zeeland"}),
		warningsWidgetAt("d", map[string]any{"announce": true}),
		warningsWidgetAt("e", map[string]any{"region": "Atlantis", "announce": true}),
	}}}
	if got := strings.Join(WarningsKeys(dashboards), " "); got != "Zeeland here" {
		t.Fatalf("WarningsKeys = %q", got)
	}
	if got := strings.Join(AnnouncedWarningKeys(dashboards), " "); got != "Zeeland here" {
		t.Fatalf("AnnouncedWarningKeys = %q, want the two widgets that announce", got)
	}
	quiet := []Dashboard{{Widgets: []Widget{warningsWidgetAt("b", map[string]any{"announce": false})}}}
	if got := AnnouncedWarningKeys(quiet); len(got) != 0 {
		t.Fatalf("AnnouncedWarningKeys = %v, want none", got)
	}
}

func TestEachDisplayGetsOnlyItsOwnWarnings(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{warningsWidgetAt("w", map[string]any{"region": "Zeeland"})}})
	data := map[string]datahub.Entry{WarningsSource: {Value: []byte(`{"Zeeland":{"region":"Zeeland","warnings":[]},"here":{"region":"Utrecht","warnings":[]}}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[WarningsSource].Value); got != `{"Zeeland":{"region":"Zeeland","warnings":[]}}` {
		t.Fatalf("hall received %s, want only Zeeland", got)
	}
}
