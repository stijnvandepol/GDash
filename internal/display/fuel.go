package display

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/fuel"
)

// FuelSource is the data key of the cheapest petrol stations for the places
// that Fuel widgets show: query key to stations (fuel.Query.Key).
const FuelSource = "fuel.default"

// maxPlaceNameLength keeps a long name from filling the title of a tile.
const maxPlaceNameLength = 100

// fuelWidget is a Fuel widget. Without a place it follows the weather
// location; the radius and fuel have defaults in fuel too.
var fuelWidget = WidgetType{Source: FuelSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"place":    {Kind: SettingPlace},
	"radius":   {Kind: SettingInteger, Min: fuel.MinRadiusKm, Max: fuel.MaxRadiusKm},
	"fuel":     {Kind: SettingChoice, Options: fuel.Types},
	"maxItems": {Kind: SettingInteger, Min: 1, Max: fuel.MaxStations},
}}

// place is a place setting once it has been checked.
type place struct {
	name                string
	latitude, longitude float64
}

// readPlace reads a place setting: exactly a name of 1 to 100 characters, a
// latitude and a longitude on earth. Anything else is not a place, so a
// hand-made value cannot smuggle more into a stored layout.
func readPlace(value any) (place, bool) {
	fields, ok := value.(map[string]any)
	if !ok || len(fields) != 3 {
		return place{}, false
	}
	name, isText := fields["name"].(string)
	lat, hasLat := number(fields["latitude"])
	lon, hasLon := number(fields["longitude"])
	n := utf8.RuneCountInString(name)
	if !isText || n < 1 || n > maxPlaceNameLength || strings.TrimSpace(name) == "" || hasControl(name) ||
		!hasLat || !hasLon || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return place{}, false
	}
	return place{name: name, latitude: lat, longitude: lon}, true
}

func placeProblem(key string, value any) string {
	if _, ok := readPlace(value); !ok {
		return fmt.Sprintf("%s must be a name of 1 to %d characters with a latitude from -90 to 90 and a longitude from -180 to 180; choose one with the place search", key, maxPlaceNameLength)
	}
	return ""
}

func choiceProblem(key string, value any, rule Setting) string {
	if choice, ok := value.(string); !ok || !slices.Contains(rule.Options, choice) {
		return fmt.Sprintf("%s must be one of: %s", key, strings.Join(rule.Options, ", "))
	}
	return ""
}

// fuelQuery reads what a Fuel widget shows, with the defaults for what it
// leaves out. ok is false for anything but a valid Fuel widget.
func fuelQuery(w Widget) (fuel.Query, bool) {
	if w.Type != "fuel" {
		return fuel.Query{}, false
	}
	q := fuel.Query{Here: true, Area: fuel.Area{RadiusKm: fuel.DefaultRadiusKm}, Fuel: fuel.DefaultFuel}
	if value, set := w.Settings["place"]; set {
		p, ok := readPlace(value)
		if !ok {
			return fuel.Query{}, false
		}
		q.Here = false
		q.Area.Latitude, q.Area.Longitude = fuel.Round(p.latitude), fuel.Round(p.longitude)
	}
	if value, set := w.Settings["radius"]; set {
		km, ok := number(value)
		if !ok || km != math.Trunc(km) || km < fuel.MinRadiusKm || km > fuel.MaxRadiusKm {
			return fuel.Query{}, false
		}
		q.Area.RadiusKm = int(km)
	}
	if value, set := w.Settings["fuel"]; set {
		kind, ok := value.(string)
		if !ok || !fuel.ValidType(kind) {
			return fuel.Query{}, false
		}
		q.Fuel = kind
	}
	return q, true
}

// FuelQueries returns what the Fuel widgets on the dashboards show, sorted
// and without repeats, so the server asks the ANWB for each once.
func FuelQueries(dashboards []Dashboard) []fuel.Query {
	var all []fuel.Query
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if q, ok := fuelQuery(w); ok {
				all = append(all, q)
			}
		}
	}
	return uniqueByKey(all)
}

// FuelKeys returns the keys of FuelQueries.
func FuelKeys(dashboards []Dashboard) []string { return keysOf(FuelQueries(dashboards)) }
