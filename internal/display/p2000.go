package display

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/p2000"
)

// P2000Source is the data key of the P2000 alerts that P2000 widgets show:
// a p2000.Value with the alerts by region and discipline.
const P2000Source = "p2000.default"

// SettingP2000Places is a list of places a P2000 widget shows, each found
// with the place test: an object with name, slug and region.
const SettingP2000Places = "p2000Places"

// The bounds of a P2000 widget.
const (
	maxP2000Places    = 5
	maxP2000PlaceName = 100
	maxP2000Hours     = 48
	maxP2000Items     = 20
)

// p2000Widget is a P2000 widget: the alerts of one safety region, or of one
// to five places, for the chosen disciplines. Two rows high it is a strip
// with the latest alert or two.
var p2000Widget = WidgetType{Source: P2000Source, MinW: 6, MinH: 2, Settings: map[string]Setting{
	"region":      {Kind: SettingChoice, Options: p2000.RegionSlugs},
	"places":      {Kind: SettingP2000Places, Min: 1, Max: maxP2000Places},
	"disciplines": {Kind: SettingChoices, Min: 1, Options: p2000.Disciplines},
	"hours":       {Kind: SettingInteger, Min: 1, Max: maxP2000Hours},
	"maxItems":    {Kind: SettingInteger, Min: 1, Max: maxP2000Items},
}}

// readP2000Places reads a places setting: 1 to maxP2000Places places with
// distinct slugs, each with a name, a valid slug and a known region. ok is
// false for anything else.
func readP2000Places(value any) ([]p2000.Place, bool) {
	list, ok := value.([]any)
	if !ok || len(list) == 0 || len(list) > maxP2000Places {
		return nil, false
	}
	places := make([]p2000.Place, 0, len(list))
	for _, item := range list {
		fields, ok := item.(map[string]any)
		if !ok || len(fields) != 3 {
			return nil, false
		}
		name, _ := fields["name"].(string)
		slug, _ := fields["slug"].(string)
		region, _ := fields["region"].(string)
		n := utf8.RuneCountInString(name)
		if n < 1 || n > maxP2000PlaceName || strings.TrimSpace(name) == "" || hasControl(name) ||
			!p2000.ValidSlug(slug) || !p2000.ValidRegion(region) {
			return nil, false
		}
		if slices.ContainsFunc(places, func(p p2000.Place) bool { return p.Slug == slug }) {
			return nil, false
		}
		places = append(places, p2000.Place{Name: name, Slug: slug, Region: region})
	}
	return places, true
}

func p2000PlacesProblem(key string, value any) string {
	if _, ok := readP2000Places(value); !ok {
		return fmt.Sprintf("%s must be 1 to %d different places, each found with Test", key, maxP2000Places)
	}
	return ""
}

// p2000Problem returns why a P2000 widget's settings do not fit together,
// or "": it shows a region or places, one of the two.
func p2000Problem(settings map[string]any) string {
	_, hasRegion := settings["region"]
	_, hasPlaces := settings["places"]
	if hasRegion == hasPlaces {
		return "a p2000 widget shows a safety region or places, one of the two"
	}
	return ""
}

// p2000Feeds returns the feeds a P2000 widget needs. ok is false for
// anything but a valid P2000 widget.
func p2000Feeds(w Widget) ([]p2000.Feed, bool) {
	if w.Type != "p2000" || p2000Problem(w.Settings) != "" {
		return nil, false
	}
	var regions []string
	if value, set := w.Settings["region"]; set {
		region, isText := value.(string)
		if !isText || !p2000.ValidRegion(region) {
			return nil, false
		}
		regions = []string{region}
	} else {
		places, ok := readP2000Places(w.Settings["places"])
		if !ok {
			return nil, false
		}
		for _, p := range places {
			regions = append(regions, p.Region)
		}
	}
	disciplines := p2000.DefaultDisciplines
	if value, set := w.Settings["disciplines"]; set {
		chosen, ok := idList(value, len(p2000.Disciplines), p2000.ValidDiscipline)
		if !ok || len(chosen) == 0 {
			return nil, false
		}
		disciplines = chosen
	}
	var feeds []p2000.Feed
	for _, r := range regions {
		for _, d := range disciplines {
			feeds = append(feeds, p2000.Feed{Region: r, Discipline: d})
		}
	}
	return feeds, true
}

// P2000Feeds returns the feeds the P2000 widgets on the dashboards show,
// sorted by key and without repeats.
func P2000Feeds(dashboards []Dashboard) []p2000.Feed {
	var all []p2000.Feed
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if feeds, ok := p2000Feeds(w); ok {
				all = append(all, feeds...)
			}
		}
	}
	return uniqueByKey(all)
}

// onlyP2000 cuts the P2000 alerts down to the feeds that the dashboards of
// one screen show.
func onlyP2000(entry datahub.Entry, dashboards []Dashboard) datahub.Entry {
	if len(entry.Value) == 0 {
		return entry
	}
	var all p2000.Value
	if err := json.Unmarshal(entry.Value, &all); err != nil {
		entry.Value = nil
		return entry
	}
	kept := p2000.Value{Feeds: map[string][]p2000.Alert{}, Skipped: []string{}}
	for _, key := range keysOf(P2000Feeds(dashboards)) {
		if alerts, ok := all.Feeds[key]; ok {
			kept.Feeds[key] = alerts
		}
		if slices.Contains(all.Skipped, key) {
			kept.Skipped = append(kept.Skipped, key)
		}
	}
	value, err := json.Marshal(kept)
	if err != nil {
		entry.Value = nil
		return entry
	}
	entry.Value = value
	return entry
}
