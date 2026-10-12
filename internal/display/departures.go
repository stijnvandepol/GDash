package display

import (
	"fmt"
	"slices"

	"github.com/stijnvandepol/gdash/internal/departures"
)

// DeparturesSource is the data key of the departures at the stops that
// Departures widgets show: stop area code to the next departures.
const DeparturesSource = "departures.default"

// departuresWidget is a Departures widget: one stop chosen with the stop
// search, which stores its code in stop and its name in stopName.
var departuresWidget = WidgetType{Source: DeparturesSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"stop":     {Kind: SettingStop, Required: true},
	"stopName": {Kind: SettingText, Min: 1, Max: 100},
	"maxItems": {Kind: SettingInteger, Min: 1, Max: departures.MaxPerStop},
}}

// stopCode reads a stop setting, or reports false for anything that is not
// a valid stop area code.
func stopCode(value any) (string, bool) {
	code, ok := value.(string)
	return code, ok && departures.ValidCode(code)
}

func stopProblem(key string, value any) string {
	if _, ok := stopCode(value); !ok {
		return fmt.Sprintf("%s must be a stop area code; choose one with the stop search", key)
	}
	return ""
}

// StopCodes returns every stop area the dashboards show departures for,
// sorted and without repeats, so the server asks OVapi for each once.
func StopCodes(dashboards []Dashboard) []string {
	var all []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if code, ok := stopCode(w.Settings["stop"]); ok && w.Type == "departures" {
				all = append(all, code)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}
