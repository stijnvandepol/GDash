package display

import (
	"math"
	"slices"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// SettingCalendar is the ID of one of the waste calendars under
// Integrations.
const SettingCalendar = "calendar"

// WasteSource is the data key of the pickups of the calendars that waste
// collection widgets show.
const WasteSource = "waste.default"

// WasteDefaultKey holds, in the waste data, the pickups of the only waste
// calendar, for widgets that name no calendar. It is absent with no waste
// calendar or several, and those widgets then ask for a choice.
const WasteDefaultKey = "default"

// EnergyPriceSource is the data key of the dynamic electricity price in
// the Netherlands.
const EnergyPriceSource = "energyprice.nl"

// SolarSource is the data key of the solar forecast for the household's
// panels.
const SolarSource = "solar.default"

// maxRecordID is the largest whole number that survives the trip through
// JSON, where every number is a float64.
const maxRecordID = 1<<53 - 1

// A widget without a calendar shows the only waste calendar.
var wasteSettings = map[string]Setting{"calendar": {Kind: SettingCalendar}}

// number reads a JSON number, which decodes as float64, or an int.
func number(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// settingID reads a setting that names a calendar or a camera under
// Settings by its ID: a whole number from 1 up.
func recordID(value any) (int64, bool) {
	n, ok := number(value)
	if !ok || n != math.Trunc(n) || n < 1 || n > maxRecordID {
		return 0, false
	}
	return int64(n), true
}

// WasteCalendarIDs returns the calendars that waste collection widgets on
// the dashboards name, sorted and without repeats, so the server fetches
// each one once, and whether a widget names none and shows the only waste
// calendar.
func WasteCalendarIDs(dashboards []Dashboard) (ids []int64, usesDefault bool) {
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != "waste" {
				continue
			}
			if _, named := w.Settings["calendar"]; !named {
				usesDefault = true
			} else if id, ok := recordID(w.Settings["calendar"]); ok {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), usesDefault
}

// wasteKeys returns the parts of the waste data that the dashboards show:
// the calendars by ID, and WasteDefaultKey for a widget without one.
func wasteKeys(dashboards []Dashboard) []string {
	ids, usesDefault := WasteCalendarIDs(dashboards)
	keys := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	if usesDefault {
		keys = append(keys, WasteDefaultKey)
	}
	return keys
}

// UsesWidget reports whether any dashboard has a widget of the type, so the
// server only asks a service for data that a screen shows.
func UsesWidget(dashboards []Dashboard, widgetType string) bool {
	return slices.ContainsFunc(dashboards, func(d Dashboard) bool {
		return slices.ContainsFunc(d.Widgets, func(w Widget) bool { return w.Type == widgetType })
	})
}

// onlyIDs narrows an entry keyed by ID, such as the web pages, to ids, for
// the same reason onlyKeys narrows Home Assistant: a screen receives what
// its own widgets show and not what another dashboard's widgets name.
func onlyIDs(entry datahub.Entry, ids []int64) datahub.Entry {
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = strconv.FormatInt(id, 10)
	}
	return onlyKeys(entry, keys)
}
