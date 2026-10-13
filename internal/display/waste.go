package display

import (
	"math"
	"slices"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// SettingCalendar is the ID of one of the calendars under Integrations.
const SettingCalendar = "calendar"

// WasteSource is the data key of the pickups of the calendars that waste
// collection widgets show.
const WasteSource = "waste.default"

// EnergyPriceSource is the data key of the dynamic electricity price in
// the Netherlands.
const EnergyPriceSource = "energyprice.nl"

// maxCalendarID is the largest whole number that survives the trip through
// JSON, where every number is a float64.
const maxCalendarID = 1<<53 - 1

var wasteSettings = map[string]Setting{"calendar": {Kind: SettingCalendar, Required: true}}

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

// calendarID reads a calendar setting: a whole number from 1 up.
func calendarID(value any) (int64, bool) {
	n, ok := number(value)
	if !ok || n != math.Trunc(n) || n < 1 || n > maxCalendarID {
		return 0, false
	}
	return int64(n), true
}

// WasteCalendarIDs returns the calendars that waste collection widgets on
// the dashboards show, sorted and without repeats, so the server fetches
// each one once.
func WasteCalendarIDs(dashboards []Dashboard) []int64 {
	var ids []int64
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if id, ok := calendarID(w.Settings["calendar"]); ok && w.Type == "waste" {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// UsesWidget reports whether any dashboard has a widget of the type, so the
// server only asks a service for data that a screen shows.
func UsesWidget(dashboards []Dashboard, widgetType string) bool {
	return slices.ContainsFunc(dashboards, func(d Dashboard) bool {
		return slices.ContainsFunc(d.Widgets, func(w Widget) bool { return w.Type == widgetType })
	})
}

// onlyCalendars narrows the waste entry to the calendars in ids, for the
// same reason onlyKeys narrows Home Assistant: a screen receives what its
// own widgets show and not the pickups of another dashboard's calendar.
func onlyCalendars(entry datahub.Entry, ids []int64) datahub.Entry {
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = strconv.FormatInt(id, 10)
	}
	return onlyKeys(entry, keys)
}
