package display

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// wasteWidget returns a waste collection widget below the default layout.
func wasteWidget(id string, calendar any) Widget {
	w := Widget{ID: id, Type: "waste", X: 0, Y: 5, W: 6, H: 4}
	if calendar != nil {
		w.Settings = map[string]any{"calendar": calendar}
	}
	return w
}

func TestValidateWasteAndEnergyPriceWidgets(t *testing.T) {
	tests := []struct {
		name   string
		widget Widget
		want   string
	}{
		{"waste", wasteWidget("waste", float64(3)), ""},
		{"waste with an int from Go code", wasteWidget("waste", 3), ""},
		{"waste without a calendar", wasteWidget("waste", nil), "widgets[2]: a waste widget needs a calendar"},
		{"waste calendar zero", wasteWidget("waste", float64(0)), "widgets[2]: calendar must be the number of a calendar added under Settings"},
		{"waste calendar negative", wasteWidget("waste", float64(-1)), "widgets[2]: calendar must be the number of a calendar"},
		{"waste calendar fraction", wasteWidget("waste", 1.5), "widgets[2]: calendar must be the number of a calendar"},
		{"waste calendar as text", wasteWidget("waste", "3"), "widgets[2]: calendar must be the number of a calendar"},
		{"waste calendar too large", wasteWidget("waste", float64(1<<60)), "widgets[2]: calendar must be the number of a calendar"},
		{"waste too small", Widget{ID: "waste", Type: "waste", X: 0, Y: 5, W: 5, H: 4, Settings: map[string]any{"calendar": float64(1)}}, "a waste widget needs at least 6x4 cells"},
		{"waste with another setting", Widget{ID: "waste", Type: "waste", X: 0, Y: 5, W: 6, H: 4, Settings: map[string]any{"calendar": float64(1), "maxItems": float64(3)}}, `unknown setting "maxItems"`},
		{"energy price", Widget{ID: "price", Type: "energyprice", X: 0, Y: 5, W: 6, H: 4}, ""},
		{"energy price too small", Widget{ID: "price", Type: "energyprice", X: 0, Y: 5, W: 5, H: 4}, "a energyprice widget needs at least 6x4 cells"},
		{"energy price takes no settings", Widget{ID: "price", Type: "energyprice", X: 0, Y: 5, W: 6, H: 4, Settings: map[string]any{"calendar": float64(1)}}, `unknown setting "calendar"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets[:2:2], tt.widget)
			got := fields(ValidateDashboard(d))
			if tt.want == "" && got != "" {
				t.Fatalf("errors = %s, want none", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeGivesTheNewWidgetsTheirSource(t *testing.T) {
	d := Normalize(Dashboard{Widgets: []Widget{wasteWidget("waste", float64(1)), {ID: "price", Type: "energyprice", Source: "other"}}})
	if d.Widgets[0].Source != WasteSource || d.Widgets[1].Source != EnergyPriceSource {
		t.Fatalf("sources = %q and %q", d.Widgets[0].Source, d.Widgets[1].Source)
	}
}

func TestWasteCalendarIDsAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{wasteWidget("a", float64(7)), wasteWidget("b", float64(2))}}
	b := Dashboard{Widgets: []Widget{
		wasteWidget("c", float64(7)),
		wasteWidget("d", float64(0)),
		wasteWidget("e", nil),
		{Type: "text", Settings: map[string]any{"calendar": float64(9)}},
	}}
	if got := WasteCalendarIDs([]Dashboard{a, b}); !slices.Equal(got, []int64{2, 7}) {
		t.Fatalf("WasteCalendarIDs = %v, want the calendars of valid waste widgets, sorted once", got)
	}
}

func TestUsesWidget(t *testing.T) {
	d := Dashboard{Widgets: []Widget{{Type: "clock"}, {Type: "energyprice"}}}
	if !UsesWidget([]Dashboard{{}, d}, "energyprice") || UsesWidget([]Dashboard{d}, "waste") || UsesWidget(nil, "clock") {
		t.Fatal("UsesWidget does not follow the widgets of the dashboards")
	}
}

func TestEachDisplayGetsOnlyItsOwnWasteCalendars(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{wasteWidget("waste", float64(1))}})
	living := Normalize(Dashboard{ID: "living", Widgets: []Widget{wasteWidget("a", float64(1))}})
	living.Widgets = append(living.Widgets, Normalize(Dashboard{Widgets: []Widget{wasteWidget("b", float64(2))}}).Widgets[0])
	living.Widgets[1].Y = 9
	data := map[string]datahub.Entry{WasteSource: {Value: []byte(
		`{"1":[{"kind":"gft","title":"GFT","date":"2026-10-13"}],"2":[{"kind":"glass","title":"Glas","date":"2026-10-14"}]}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "living": {Dashboard: living}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calendars := func(displayID string) []string {
		var all map[string]json.RawMessage
		if err := json.Unmarshal(decode(t, <-b.Subscribe(ctx, displayID)).Data[WasteSource].Value, &all); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for id := range all {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		return ids
	}
	if got := calendars("hall"); !slices.Equal(got, []string{"1"}) {
		t.Fatalf("hall received calendars %v, want only 1", got)
	}
	if got := calendars("living"); !slices.Equal(got, []string{"1", "2"}) {
		t.Fatalf("living room received calendars %v, want 1 and 2", got)
	}
}
