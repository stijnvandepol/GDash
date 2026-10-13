package display

import (
	"slices"

	"github.com/stijnvandepol/gdash/internal/holidays"
)

// HolidaysSource is the data key of the coming school and public holidays
// in the Netherlands.
const HolidaysSource = "holidays.nl"

// What a Holidays widget shows.
const (
	showBoth   = "both"
	showSchool = "school"
	showPublic = "public"
)

// holidaysWidget lists the coming holidays. Without settings it shows both
// kinds, with the school holidays of the middle region.
var holidaysWidget = WidgetType{Source: HolidaysSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"region":   {Kind: SettingChoice, Options: holidays.Regions},
	"show":     {Kind: SettingChoice, Options: []string{showBoth, showSchool, showPublic}},
	"maxItems": {Kind: SettingInteger, Min: 1, Max: 20},
}}

// HolidayKeys returns the parts of the holidays report that the Holidays
// widgets on the dashboards show: holidays.PublicKey and school holiday
// regions, sorted and without repeats.
func HolidayKeys(dashboards []Dashboard) []string {
	var keys []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != "holidays" {
				continue
			}
			show, _ := w.Settings["show"].(string)
			if show != showSchool {
				keys = append(keys, holidays.PublicKey)
			}
			if show != showPublic {
				keys = append(keys, holidayRegion(w))
			}
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// holidayRegion is the school holiday region a widget shows.
func holidayRegion(w Widget) string {
	if region, ok := w.Settings["region"].(string); ok && slices.Contains(holidays.Regions, region) {
		return region
	}
	return holidays.Middle
}
