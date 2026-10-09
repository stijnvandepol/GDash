package display

import (
	"slices"
	"testing"
	"time"
)

var templatesNow = time.Date(2026, time.October, 9, 12, 0, 0, 0, time.UTC)

func TestTemplatesAreValidDashboards(t *testing.T) {
	templates := Templates(templatesNow)
	if len(templates) < 5 {
		t.Fatalf("got %d templates, want at least 5", len(templates))
	}
	ids := map[string]bool{}
	for _, tmpl := range templates {
		if ids[tmpl.ID] || !ValidDashboardID(tmpl.ID) {
			t.Errorf("template id %q is repeated or not a valid dashboard id", tmpl.ID)
		}
		ids[tmpl.ID] = true
		if tmpl.Description == "" {
			t.Errorf("%s: no description", tmpl.ID)
		}
		d := Normalize(Dashboard{ID: tmpl.ID, Name: tmpl.Name, Theme: tmpl.Theme, Widgets: tmpl.Widgets})
		// ValidateDashboard covers the grid, the minimum sizes, overlaps and settings.
		if err := ValidateDashboard(d); err != nil {
			t.Errorf("%s: %s", tmpl.ID, fields(err))
		}
		for _, w := range tmpl.Widgets {
			if w.Type == "homeassistant" {
				t.Errorf("%s: widget %s needs Home Assistant entities", tmpl.ID, w.ID)
			}
			if want := WidgetTypes[w.Type].Source; w.Source != want {
				t.Errorf("%s: widget %s has source %q, want %q", tmpl.ID, w.ID, w.Source, want)
			}
		}
	}
}

func TestTemplateThemes(t *testing.T) {
	for _, tmpl := range Templates(templatesNow) {
		if !slices.Contains(Themes, tmpl.Theme) {
			t.Errorf("%s: unknown theme %q", tmpl.ID, tmpl.Theme)
		}
		if tmpl.ID == "bedroom" && tmpl.Theme != "night" {
			t.Errorf("bedroom theme = %q, want night", tmpl.Theme)
		}
	}
}

func TestTemplateCountdownIsInTheFuture(t *testing.T) {
	// On the last day of a year the next New Year is a day away; on the
	// first day it is a full year away.
	times := []time.Time{templatesNow, time.Date(2026, time.December, 31, 23, 59, 0, 0, time.UTC), time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)}
	for _, now := range times {
		found := false
		for _, tmpl := range Templates(now) {
			for _, w := range tmpl.Widgets {
				if w.Type != "countdown" {
					continue
				}
				found = true
				date, err := time.Parse(dateLayout, w.Settings["date"].(string))
				if err != nil || !date.After(now) {
					t.Errorf("now %s: countdown date %v (%v) is not after now", now.Format(dateLayout), w.Settings["date"], err)
				}
			}
		}
		if !found {
			t.Error("no template has a countdown")
		}
	}
}

func TestTemplatesDoNotShareState(t *testing.T) {
	first := Templates(templatesNow)
	first[0].Widgets[0].X = 99
	first[0].Widgets[2].Settings["days"] = 99
	if second := Templates(templatesNow); second[0].Widgets[0].X == 99 || second[0].Widgets[2].Settings["days"] == 99 {
		t.Error("a change to one result leaked into the next call")
	}
}
