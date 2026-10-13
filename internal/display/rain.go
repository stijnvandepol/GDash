package display

import "github.com/stijnvandepol/gdash/internal/rain"

// RainSource is the data key of the rain forecasts for the places that Rain
// widgets show: query key to forecast (rain.Query.Key).
const RainSource = "rain.default"

// rainWidget is a Rain widget. Without a place it follows the weather
// location.
var rainWidget = WidgetType{Source: RainSource, MinW: 5, MinH: 3, Settings: map[string]Setting{
	"place": {Kind: SettingPlace},
}}

// rainQuery reads the place a Rain widget shows. ok is false for anything
// but a valid Rain widget.
func rainQuery(w Widget) (rain.Query, bool) {
	if w.Type != "rain" {
		return rain.Query{}, false
	}
	value, set := w.Settings["place"]
	if !set {
		return rain.Query{Here: true}, true
	}
	p, ok := readPlace(value)
	if !ok {
		return rain.Query{}, false
	}
	return rain.Query{Latitude: rain.Round(p.latitude), Longitude: rain.Round(p.longitude)}, true
}

// RainQueries returns the places the Rain widgets on the dashboards show,
// sorted and without repeats, so the server asks for each once.
func RainQueries(dashboards []Dashboard) []rain.Query {
	var all []rain.Query
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if q, ok := rainQuery(w); ok {
				all = append(all, q)
			}
		}
	}
	return uniqueByKey(all)
}

// RainKeys returns the keys of RainQueries.
func RainKeys(dashboards []Dashboard) []string { return keysOf(RainQueries(dashboards)) }
