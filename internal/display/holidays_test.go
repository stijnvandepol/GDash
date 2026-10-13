package display

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// holidaysOn returns a Holidays widget below the default layout.
func holidaysOn(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "holidays", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func TestValidateHolidaysAndMoonWidgets(t *testing.T) {
	tests := []struct {
		name   string
		widget Widget
		want   string
	}{
		{"holidays without settings", holidaysOn("h", nil), ""},
		{"holidays with every setting", holidaysOn("h", map[string]any{"region": "south", "show": "school", "maxItems": float64(3)}), ""},
		{"unknown region", holidaysOn("h", map[string]any{"region": "zuid"}), "widgets[2]: region must be one of: north, middle, south"},
		{"unknown show", holidaysOn("h", map[string]any{"show": "all"}), "widgets[2]: show must be one of: both, school, public"},
		{"too many items", holidaysOn("h", map[string]any{"maxItems": float64(21)}), "widgets[2]: maxItems must be a whole number from 1 to 20"},
		{"holidays too small", Widget{ID: "h", Type: "holidays", X: 0, Y: 5, W: 5, H: 4}, "a holidays widget needs at least 6x4 cells"},
		{"moon", Widget{ID: "moon", Type: "moon", X: 0, Y: 5, W: 4, H: 4}, ""},
		{"moon too small", Widget{ID: "moon", Type: "moon", X: 0, Y: 5, W: 4, H: 3}, "a moon widget needs at least 4x4 cells"},
		{"moon takes no settings", Widget{ID: "moon", Type: "moon", X: 0, Y: 5, W: 4, H: 4, Settings: map[string]any{"region": "north"}}, `unknown setting "region"`},
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

func TestHolidayKeysFollowWhatTheWidgetsShow(t *testing.T) {
	tests := []struct {
		name    string
		widgets []Widget
		want    []string
	}{
		{"none", []Widget{{Type: "clock"}}, nil},
		{"both in the middle by default", []Widget{holidaysOn("a", nil)}, []string{"middle", "public"}},
		{"school in the north", []Widget{holidaysOn("a", map[string]any{"show": "school", "region": "north"})}, []string{"north"}},
		{"public only", []Widget{holidaysOn("a", map[string]any{"show": "public", "region": "south"})}, []string{"public"}},
		{"two widgets", []Widget{
			holidaysOn("a", map[string]any{"show": "public"}),
			holidaysOn("b", map[string]any{"region": "south"}),
			holidaysOn("c", map[string]any{"region": "south", "show": "school"}),
		}, []string{"public", "south"}},
	}
	for _, tt := range tests {
		if got := HolidayKeys([]Dashboard{{Widgets: tt.widgets}}); !slices.Equal(got, tt.want) {
			t.Errorf("%s: HolidayKeys = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEachDisplayGetsOnlyTheHolidaysItShows(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{holidaysOn("h", map[string]any{"show": "public"})}})
	living := Normalize(Dashboard{ID: "living", Widgets: []Widget{holidaysOn("h", map[string]any{"region": "north"})}})
	data := map[string]datahub.Entry{HolidaysSource: {Value: []byte(`{"public":[{"kind":"christmas","date":"2026-12-25"}],` +
		`"north":[],"middle":[{"kind":"autumn","name":"Herfstvakantie","start":"2026-10-17","end":"2026-10-25"}],"south":[]}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "living": {Dashboard: living}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	parts := func(displayID string) []string {
		var all map[string]json.RawMessage
		if err := json.Unmarshal(decode(t, <-b.Subscribe(ctx, displayID)).Data[HolidaysSource].Value, &all); err != nil {
			t.Fatal(err)
		}
		return slices.Sorted(maps.Keys(all))
	}
	if got := parts("hall"); !slices.Equal(got, []string{"public"}) {
		t.Fatalf("hall received %v, want only the public holidays", got)
	}
	// An empty region stays: the north has no holidays left, which the
	// screen must not take for data that has not arrived.
	if got := parts("living"); !slices.Equal(got, []string{"north", "public"}) {
		t.Fatalf("living room received %v, want the public holidays and the north", got)
	}
}
