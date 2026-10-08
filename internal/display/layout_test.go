package display

import (
	"strings"
	"testing"

	"github.com/stijnvandepol/daycast/internal/validation"
)

func valid() Dashboard {
	return Dashboard{ID: "kitchen", Name: "Kitchen", Theme: "glass", Widgets: []Widget{
		{ID: "clock", Type: "clock", X: 0, Y: 0, W: 10, H: 5},
		{ID: "news", Type: "news", X: 10, Y: 0, W: 14, H: 5, Settings: map[string]any{"maxItems": float64(4)}},
	}}
}

func fields(err error) string {
	var names []string
	for _, f := range validation.Fields(err) {
		names = append(names, f.Field+": "+f.Message)
	}
	return strings.Join(names, " | ")
}

func TestValidateDashboard(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Dashboard)
		want   string
	}{
		{"valid", func(*Dashboard) {}, ""},
		{"empty dashboard", func(d *Dashboard) { d.Widgets = nil }, ""},
		{"bad id", func(d *Dashboard) { d.ID = "Kitchen!" }, "id:"},
		{"blank name", func(d *Dashboard) { d.Name = "  " }, "name:"},
		{"unknown theme", func(d *Dashboard) { d.Theme = "comic-sans" }, "theme: choose one of"},
		{"unknown type", func(d *Dashboard) { d.Widgets[0].Type = "iframe" }, `widgets[0]: unknown widget type "iframe"`},
		{"outside grid", func(d *Dashboard) { d.Widgets[1].X = 12 }, "widgets[1]: must fit the 24x14 grid"},
		{"negative position", func(d *Dashboard) { d.Widgets[0].Y = -1 }, "widgets[0]: must fit"},
		{"too small", func(d *Dashboard) { d.Widgets[1].W = 3 }, "widgets[1]: a news widget needs at least 6x3 cells"},
		{"overlap", func(d *Dashboard) { d.Widgets[1].X = 9; d.Widgets[1].W = 15 }, "widgets[1]: overlaps another widget"},
		{"duplicate id", func(d *Dashboard) { d.Widgets[1].ID = "clock" }, "widgets[1]: id is used by another widget"},
		{"setting on clock", func(d *Dashboard) { d.Widgets[0].Settings = map[string]any{"maxItems": float64(3)} }, `widgets[0]: unknown setting "maxItems"`},
		{"maxItems too big", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = float64(50) }, "widgets[1]: maxItems must be a whole number"},
		{"maxItems fraction", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = 2.5 }, "widgets[1]: maxItems must be a whole number"},
		{"maxItems text", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = "5" }, "widgets[1]: maxItems must be a whole number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append([]Widget(nil), d.Widgets...)
			d.Widgets[1].Settings = map[string]any{"maxItems": float64(4)}
			tt.change(&d)
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

func TestValidateDashboardLimitsWidgetCount(t *testing.T) {
	d := valid()
	d.Widgets = make([]Widget, maxWidgets+1)
	if got := fields(ValidateDashboard(d)); !strings.Contains(got, "use at most 30 widgets") {
		t.Fatalf("errors = %s", got)
	}
}

func TestNormalizeSetsSourceFromType(t *testing.T) {
	d := valid()
	d.Columns, d.Rows = 99, 99
	d.Widgets[1].Source = "secrets.everything"
	n := Normalize(d)
	if n.Columns != GridColumns || n.Rows != GridRows || n.Theme != "glass" {
		t.Fatalf("grid/theme = %d %d %s", n.Columns, n.Rows, n.Theme)
	}
	if empty := Normalize(Dashboard{}); empty.Theme != defaultTheme {
		t.Fatalf("empty theme became %q, want the default", empty.Theme)
	}
	if n.Widgets[1].Source != "news.default" || n.Widgets[0].Source != "" {
		t.Fatalf("sources = %q %q, want them derived from the type", n.Widgets[0].Source, n.Widgets[1].Source)
	}
	if d.Widgets[1].Source != "secrets.everything" {
		t.Fatal("Normalize changed its input")
	}
}

func TestDefaultDashboardsAreValid(t *testing.T) {
	for _, withCalendar := range []bool{false, true} {
		if err := ValidateDashboard(DefaultDashboard(withCalendar)); err != nil {
			t.Errorf("DefaultDashboard(%v): %s", withCalendar, fields(err))
		}
	}
}
