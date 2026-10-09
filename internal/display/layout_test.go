package display

import (
	"math"
	"strings"
	"testing"

	"github.com/stijnvandepol/nookboard/internal/validation"
)

func valid() Dashboard {
	return Dashboard{ID: "kitchen", Name: "Kitchen", Theme: "glass", Widgets: []Widget{
		{ID: "clock", Type: "clock", X: 0, Y: 0, W: 10, H: 5},
		{ID: "news", Type: "news", X: 10, Y: 0, W: 14, H: 5, Settings: map[string]any{"maxItems": float64(4)}},
	}}
}

func note(text string) Widget {
	return Widget{ID: "note", Type: "text", X: 0, Y: 5, W: 4, H: 3, Settings: map[string]any{"text": text}}
}

func countdown(label, date string) Widget {
	return Widget{ID: "holiday", Type: "countdown", X: 0, Y: 5, W: 4, H: 4, Settings: map[string]any{"label": label, "date": date}}
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
		{"position that wraps around", func(d *Dashboard) { d.Widgets[0].X = math.MaxInt - 1 }, "widgets[0]: must fit"},
		{"too small", func(d *Dashboard) { d.Widgets[1].W = 3 }, "widgets[1]: a news widget needs at least 6x4 cells"},
		{"overlap", func(d *Dashboard) { d.Widgets[1].X = 9; d.Widgets[1].W = 15 }, "widgets[1]: overlaps another widget"},
		{"duplicate id", func(d *Dashboard) { d.Widgets[1].ID = "clock" }, "widgets[1]: id is used by another widget"},
		{"setting on clock", func(d *Dashboard) { d.Widgets[0].Settings = map[string]any{"maxItems": float64(3)} }, `widgets[0]: unknown setting "maxItems"`},
		{"maxItems too big", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = float64(50) }, "widgets[1]: maxItems must be a whole number"},
		{"maxItems fraction", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = 2.5 }, "widgets[1]: maxItems must be a whole number"},
		{"maxItems text", func(d *Dashboard) { d.Widgets[1].Settings["maxItems"] = "5" }, "widgets[1]: maxItems must be a whole number"},
		{"note", func(d *Dashboard) { d.Widgets = append(d.Widgets, note("Bins out tonight\nBring a coat")) }, ""},
		{"empty note", func(d *Dashboard) { d.Widgets = append(d.Widgets, note("   ")) }, "widgets[2]: text must be 1 to 300 characters"},
		{"note with control character", func(d *Dashboard) { d.Widgets = append(d.Widgets, note("bell\a")) }, "widgets[2]: text must be"},
		{"note too long", func(d *Dashboard) { d.Widgets = append(d.Widgets, note(strings.Repeat("é", 301))) }, "widgets[2]: text must be"},
		{"note without text", func(d *Dashboard) {
			d.Widgets = append(d.Widgets, Widget{ID: "note", Type: "text", X: 0, Y: 5, W: 4, H: 3})
		}, "widgets[2]: a text widget needs a text"},
		{"countdown", func(d *Dashboard) { d.Widgets = append(d.Widgets, countdown("Holiday", "2026-12-24")) }, ""},
		{"countdown bad date", func(d *Dashboard) { d.Widgets = append(d.Widgets, countdown("Holiday", "24-12-2026")) }, "widgets[2]: date must be a date as YYYY-MM-DD"},
		{"countdown date as number", func(d *Dashboard) {
			w := countdown("Holiday", "")
			w.Settings["date"] = float64(20261224)
			d.Widgets = append(d.Widgets, w)
		}, "widgets[2]: date must be a date"},
		{"home assistant", func(d *Dashboard) { d.Widgets = append(d.Widgets, entities("sensor.temp", "binary_sensor.door")) }, ""},
		{"home assistant without entities", func(d *Dashboard) {
			w := entities()
			w.Settings = nil
			d.Widgets = append(d.Widgets, w)
		}, "widgets[2]: choose 1 to 8 Home Assistant entities"},
		{"home assistant empty list", func(d *Dashboard) { d.Widgets = append(d.Widgets, entities()) }, "widgets[2]: choose 1 to 8"},
		{"home assistant bad id", func(d *Dashboard) { d.Widgets = append(d.Widgets, entities("../api/config")) }, "widgets[2]: choose"},
		{"home assistant repeated id", func(d *Dashboard) { d.Widgets = append(d.Widgets, entities("sensor.a", "sensor.a")) }, "widgets[2]: choose"},
		{"home assistant too many", func(d *Dashboard) {
			d.Widgets = append(d.Widgets, entities("a.a", "a.b", "a.c", "a.d", "a.e", "a.f", "a.g", "a.h", "a.i"))
		}, "widgets[2]: choose"},
		{"home assistant id as number", func(d *Dashboard) { d.Widgets = append(d.Widgets, entities(float64(1))) }, "widgets[2]: choose"},
		{"forecast days", func(d *Dashboard) {
			d.Widgets = append(d.Widgets, Widget{ID: "fc", Type: "forecast", X: 0, Y: 5, W: 6, H: 4, Settings: map[string]any{"days": float64(8)}})
		}, "widgets[2]: days must be a whole number from 1 to 7"},
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

// entities returns a Home Assistant widget below the default layout.
func entities(ids ...any) Widget {
	return Widget{ID: "home", Type: "homeassistant", X: 0, Y: 5, W: 4, H: 3, Settings: map[string]any{"entities": ids}}
}

func TestEntityIDsAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{entities("sensor.b", "sensor.a"), {Type: "text", Settings: map[string]any{"entities": []any{"sensor.c"}}}}}
	b := Dashboard{Widgets: []Widget{entities("sensor.a", "light.kitchen"), entities("bad id")}}
	got := strings.Join(EntityIDs([]Dashboard{a, b}), ",")
	if got != "light.kitchen,sensor.a,sensor.b" {
		t.Fatalf("EntityIDs = %s, want the entities of valid Home Assistant widgets, sorted once", got)
	}
}
