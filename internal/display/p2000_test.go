package display

import (
	"context"
	"slices"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func p2000WidgetAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "p2000", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func p2000Place(name, slug, region string) map[string]any {
	return map[string]any{"name": name, "slug": slug, "region": region}
}

func TestP2000SettingsAreValidated(t *testing.T) {
	const placesMsg = "widgets[2]: places must be 1 to 5 different places, each found with Test"
	const eitherMsg = "widgets[2]: a p2000 widget shows a safety region or places, one of the two"
	nuenen := p2000Place("Nuenen", "nuenen", "brabant-zuidoost")
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"a region", map[string]any{"region": "brabant-zuidoost", "disciplines": []any{"brandweer"}, "hours": float64(12), "maxItems": float64(8)}, ""},
		{"places", map[string]any{"places": []any{nuenen, p2000Place("Den Haag", "den-haag", "haaglanden")}}, ""},
		{"neither", nil, eitherMsg},
		{"both", map[string]any{"region": "twente", "places": []any{nuenen}}, eitherMsg},
		{"unknown region", map[string]any{"region": "brabant"}, "widgets[2]: region must be one of: groningen, friesland, drenthe, ijsselland, twente, noord-en-oost-gelderland, gelderland-midden, gelderland-zuid, utrecht, noord-holland-noord, zaanstreek-waterland, kennemerland, amsterdam-amstelland, gooi-en-vechtstreek, haaglanden, hollands-midden, rotterdam-rijnmond, zuid-holland-zuid, zeeland, midden-en-west-brabant, brabant-noord, brabant-zuidoost, limburg-noord, limburg-zuid, flevoland"},
		{"no places", map[string]any{"places": []any{}}, placesMsg},
		{"a place twice", map[string]any{"places": []any{nuenen, nuenen}}, placesMsg},
		{"a place in an unknown region", map[string]any{"places": []any{p2000Place("Nuenen", "nuenen", "brabant")}}, placesMsg},
		{"a place with a bad slug", map[string]any{"places": []any{p2000Place("Nuenen", "../x", "twente")}}, placesMsg},
		{"a place with more fields", map[string]any{"places": []any{map[string]any{"name": "x", "slug": "x", "region": "twente", "id": "1"}}}, placesMsg},
		{"six places", map[string]any{"places": []any{
			p2000Place("a", "a", "twente"), p2000Place("b", "b", "twente"), p2000Place("c", "c", "twente"),
			p2000Place("d", "d", "twente"), p2000Place("e", "e", "twente"), p2000Place("f", "f", "twente"),
		}}, placesMsg},
		{"no disciplines", map[string]any{"region": "twente", "disciplines": []any{}}, "widgets[2]: disciplines must be at least 1 of: brandweer, politie, ambulance, knrm, trauma, each once"},
		{"too many hours", map[string]any{"region": "twente", "hours": float64(49)}, "widgets[2]: hours must be a whole number from 1 to 48"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, p2000WidgetAt("p2000", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestP2000Feeds(t *testing.T) {
	dashboards := []Dashboard{
		{Widgets: []Widget{p2000WidgetAt("a", map[string]any{"region": "twente", "disciplines": []any{"politie", "brandweer"}})}},
		{Widgets: []Widget{
			// Without disciplines: all but ambulance.
			p2000WidgetAt("b", map[string]any{"places": []any{p2000Place("Hengelo", "hengelo-ov", "twente")}}),
			p2000WidgetAt("c", map[string]any{"region": "nowhere"}),
		}},
	}
	want := []string{"twente/brandweer", "twente/knrm", "twente/politie", "twente/trauma"}
	if got := keysOf(P2000Feeds(dashboards)); !slices.Equal(got, want) {
		t.Fatalf("feeds = %v, want %v", got, want)
	}
}

func TestEachDisplayGetsOnlyItsOwnP2000Feeds(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{p2000WidgetAt("p", map[string]any{"region": "zeeland", "disciplines": []any{"brandweer"}})}})
	data := map[string]datahub.Entry{P2000Source: {Value: []byte(`{"feeds":{"zeeland/brandweer":[],"twente/brandweer":[]},"skipped":["twente/politie"]}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[P2000Source].Value); got != `{"feeds":{"zeeland/brandweer":[]},"skipped":[]}` {
		t.Fatalf("hall received %s, want only Zeeland's fire alerts", got)
	}
}
