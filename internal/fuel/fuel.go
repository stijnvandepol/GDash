// Package fuel reads petrol station prices in the Netherlands from the
// public points-of-interest service of the ANWB, which needs no API key.
// A widget shows the cheapest stations within a radius of a place for one
// kind of fuel. The prices are the ones the ANWB publishes and can lag the
// sign at the station.
package fuel

import (
	"fmt"
	"math"
	"slices"
	"strconv"
)

const (
	// MaxStations is how many stations the server keeps for a query.
	// Widgets show a part of it, and a Cast snapshot has to stay below
	// 64 KB.
	MaxStations = 10

	MinRadiusKm     = 1
	MaxRadiusKm     = 25
	DefaultRadiusKm = 5

	// DefaultFuel is what a widget without a fuel setting shows.
	DefaultFuel = "EURO95"

	// coordinateScale rounds a place to three decimals, about 100 m. It
	// makes two widgets at the same town share one request, and keeps the
	// key a widget builds from its settings equal to the server's.
	coordinateScale = 1000
)

// Types are the fuels a widget can show, in the ANWB's own codes. DIESEL_SPECIAL
// is left out: few stations sell it and it is not a fuel people compare.
var Types = []string{"EURO95", "EURO98", "DIESEL", "AUTOGAS", "CNG"}

// ValidType reports whether fuel is one of Types.
func ValidType(fuel string) bool { return slices.Contains(Types, fuel) }

// Station is one petrol station that sells the fuel asked for.
type Station struct {
	// Name is the brand and the station's own name, with the town unless
	// the name already holds it.
	Name string `json:"name"`
	// Price is euro per litre, or per kilogram for CNG, with three
	// decimals as pumps show it.
	Price float64 `json:"price"`
	// DistanceKm is the straight-line distance from the place, with one
	// decimal.
	DistanceKm float64 `json:"distanceKm"`
}

// Area is a circle on the map: the place and how far to look.
type Area struct {
	Latitude  float64
	Longitude float64
	RadiusKm  int
}

// Query is what one Fuel widget shows.
type Query struct {
	// Here means the widget follows the weather location under Integrations.
	// The caller fills in Area from it; Key ignores the place then, so the
	// key is the same before and after the location changes.
	Here bool
	Area Area
	Fuel string
}

// Key identifies a query in the data the server sends. The widget on the
// TV builds the same text from its settings (web/src/widgets/fuel.ts).
func (q Query) Key() string {
	if q.Here {
		return fmt.Sprintf("here,%d,%s", q.Area.RadiusKm, q.Fuel)
	}
	return fmt.Sprintf("%s,%s,%d,%s", Coordinate(q.Area.Latitude), Coordinate(q.Area.Longitude), q.Area.RadiusKm, q.Fuel)
}

// Round rounds a latitude or longitude to three decimals. Places are stored
// and fetched rounded, so every widget at the same town shares one request.
func Round(degrees float64) float64 {
	rounded := math.Round(degrees*coordinateScale) / coordinateScale
	if rounded == 0 {
		// Go writes a negative zero as "-0", JavaScript as "0".
		return 0
	}
	return rounded
}

// Coordinate writes a rounded latitude or longitude without trailing zeros,
// as JavaScript writes the same number.
func Coordinate(degrees float64) string {
	return strconv.FormatFloat(Round(degrees), 'f', -1, 64)
}
