package display

import (
	"slices"

	"github.com/stijnvandepol/gdash/internal/warnings"
)

// WarningsSource is the data key of the weather warnings for the regions
// that Weather warnings widgets show: query key to area
// (warnings.Query.Key).
const WarningsSource = "warnings.default"

// warningsWidget is a Weather warnings widget. Without a region it follows
// the weather location. With announce, a code orange or red warning that
// comes into effect is also put over every screen.
var warningsWidget = WidgetType{Source: WarningsSource, MinW: 6, MinH: 3, Settings: map[string]Setting{
	"region":   {Kind: SettingChoice, Options: warnings.Regions},
	"announce": {Kind: SettingBoolean},
}}

// warningsQuery reads the region a Weather warnings widget shows and
// whether it announces. ok is false for anything but a valid widget.
func warningsQuery(w Widget) (q warnings.Query, announce, ok bool) {
	if w.Type != "warnings" {
		return warnings.Query{}, false, false
	}
	q = warnings.Query{Here: true}
	if value, set := w.Settings["region"]; set {
		name, isText := value.(string)
		if !isText || !warnings.ValidRegion(name) {
			return warnings.Query{}, false, false
		}
		q = warnings.Query{Region: name}
	}
	if value, set := w.Settings["announce"]; set {
		on, isBool := value.(bool)
		if !isBool {
			return warnings.Query{}, false, false
		}
		announce = on
	}
	return q, announce, true
}

// WarningsQueries returns the regions the Weather warnings widgets on the
// dashboards show, sorted and without repeats.
func WarningsQueries(dashboards []Dashboard) []warnings.Query {
	var all []warnings.Query
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if q, _, ok := warningsQuery(w); ok {
				all = append(all, q)
			}
		}
	}
	return uniqueByKey(all)
}

// WarningsKeys returns the keys of WarningsQueries.
func WarningsKeys(dashboards []Dashboard) []string { return keysOf(WarningsQueries(dashboards)) }

// AnnouncedWarningKeys returns the keys of the Weather warnings widgets
// that announce, sorted and without repeats.
func AnnouncedWarningKeys(dashboards []Dashboard) []string {
	var keys []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if q, announce, ok := warningsQuery(w); ok && announce {
				keys = append(keys, q.Key())
			}
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}
