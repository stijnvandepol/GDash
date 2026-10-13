// Package rain reads the expected rain for the next two hours at a place.
// Buienradar's short-term radar forecast comes first: it is made for this,
// in steps of five minutes, but only covers the Netherlands and Belgium.
// Elsewhere, or when Buienradar fails, Open-Meteo's quarter-hourly forecast
// takes over. Neither needs an API key. Buienradar's data is free for
// non-commercial use with a credit to buienradar.nl, which the widget shows.
package rain

import (
	"math"
	"strconv"
	"time"
)

const (
	// Sources name where a forecast came from, for the credit on the screen.
	SourceBuienradar = "buienradar"
	SourceOpenMeteo  = "open-meteo"

	// coordinateScale rounds a place to two decimals, about 1 km: what
	// Buienradar takes, and enough for rain. Widgets at the same town share
	// one request.
	coordinateScale = 100
	// mmScale rounds an intensity to a hundredth of a millimetre per hour.
	mmScale = 100
)

// Point is the expected rain from Time for the forecast's step.
type Point struct {
	Time      time.Time `json:"time"`
	MMPerHour float64   `json:"mmPerHour"`
}

// Forecast is the expected rain at a place, soonest first.
type Forecast struct {
	// Source is SourceBuienradar or SourceOpenMeteo.
	Source string `json:"source"`
	// StepMinutes is how long each point lasts: 5 for Buienradar, 15 for
	// Open-Meteo.
	StepMinutes int     `json:"stepMinutes"`
	Points      []Point `json:"points"`
}

// Query is the place one Rain widget shows.
type Query struct {
	// Here means the widget follows the weather location under Settings.
	// The caller fills in the coordinates; Key ignores them, so the key is
	// the same before and after the location changes.
	Here      bool
	Latitude  float64
	Longitude float64
}

// HereKey is the key of a widget without a place of its own.
const HereKey = "here"

// Key identifies a query in the data the server sends. The widget on the
// TV builds the same text from its settings (web/src/widgets/rain.ts).
func (q Query) Key() string {
	if q.Here {
		return HereKey
	}
	return Coordinate(q.Latitude) + "," + Coordinate(q.Longitude)
}

// valid reports whether the query is a point on earth.
func (q Query) valid() bool {
	return math.Abs(q.Latitude) <= 90 && math.Abs(q.Longitude) <= 180
}

// Round rounds a latitude or longitude to two decimals.
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

// roundIntensity drops what no one would call rain and the noise of the
// conversion below a hundredth of a millimetre.
func roundIntensity(mmPerHour float64) float64 {
	return math.Round(mmPerHour*mmScale) / mmScale
}
