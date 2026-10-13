// Package warnings reads the weather warnings of the KNMI, the Dutch
// weather service, from the public MeteoAlarm feed of the Netherlands,
// which needs no API key. The feed holds CAP alerts per province; GDash
// keeps the yellow, orange and red ones that have not expired, per region.
// Other countries publish their own MeteoAlarm feed in the same shape, so
// another country is a matter of its feed address and its regions.
package warnings

import (
	"slices"
	"time"
)

// Levels of a warning, from MeteoAlarm's awareness level. Green, "no
// warning", is never kept.
const (
	LevelYellow = "yellow"
	LevelOrange = "orange"
	LevelRed    = "red"
)

// Warning is one weather warning for a region.
type Warning struct {
	// ID stays the same when the KNMI updates the warning (a new end time
	// or a higher level), so a screen announces it once.
	ID    string `json:"id"`
	Level string `json:"level"`
	// Kind is MeteoAlarm's awareness type, such as "wind" or "rain", or
	// empty for a type GDash does not know; the screen names it in its
	// own language and falls back on Event.
	Kind string `json:"kind"`
	// Event and Headline are the KNMI's own texts by language: "nl" and
	// "en".
	Event    map[string]string `json:"event"`
	Headline map[string]string `json:"headline"`
	Onset    time.Time         `json:"onset"`
	Expires  time.Time         `json:"expires"`
}

// InEffect reports whether the warning applies at now.
func (w Warning) InEffect(now time.Time) bool {
	return !now.Before(w.Onset) && now.Before(w.Expires)
}

// Severe reports whether the warning is code orange or red, the levels
// that may be announced on every screen.
func (w Warning) Severe() bool {
	return w.Level == LevelOrange || w.Level == LevelRed
}

// Area is the warnings of one region, soonest first.
type Area struct {
	Region   string    `json:"region"`
	Warnings []Warning `json:"warnings"`
}

// Query is the region one Warnings widget shows.
type Query struct {
	// Here means the widget follows the weather location under Settings.
	// The caller fills in Region from it; Key ignores it, so the key is
	// the same before and after the location changes.
	Here   bool
	Region string
}

// HereKey is the key of a widget without a region of its own.
const HereKey = "here"

// Key identifies a query in the data the server sends. The widget on the
// TV builds the same text from its settings (web/src/widgets/warnings.ts).
func (q Query) Key() string {
	if q.Here {
		return HereKey
	}
	return q.Region
}

// region is a warning region of the KNMI with MeteoAlarm's code for it.
type region struct {
	name string
	// emma is the EMMA_ID geocode MeteoAlarm gives the area.
	emma string
}

// regions are the land regions the KNMI warns for: the twelve provinces
// and the Wadden Islands. The coastal and water areas (IJsselmeer, the
// Waddenzee, the harbours) are for shipping and are left out.
var regions = []region{
	{"Groningen", "NL007"},
	{"Friesland", "NL017"},
	{"Drenthe", "NL018"},
	{"Overijssel", "NL016"},
	{"Flevoland", "NL008"},
	{"Gelderland", "NL014"},
	{"Utrecht", "NL015"},
	{"Noord-Holland", "NL011"},
	{"Zuid-Holland", "NL009"},
	{"Zeeland", "NL010"},
	{"Noord-Brabant", "NL013"},
	{"Limburg", "NL012"},
	{"Waddeneilanden", "NL019"},
}

// Regions are the names a widget's region setting takes.
var Regions = func() []string {
	names := make([]string, len(regions))
	for i, r := range regions {
		names[i] = r.name
	}
	return names
}()

// ValidRegion reports whether name is one of Regions.
func ValidRegion(name string) bool { return slices.Contains(Regions, name) }
