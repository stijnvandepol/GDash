// Package traffic reads traffic jams, roadworks and mobile speed checks on
// Dutch roads from the traffic information of the ANWB, the service behind
// anwb.nl/verkeer.
//
// The service is undocumented and unofficial: the ANWB publishes no API and
// every answer says that its content may not be reused without written
// permission. GDash therefore only asks for it after the admin turned it on
// for their own household (see internal/settings), at most every few
// minutes, with one request shared by every screen, and stops asking when
// the ANWB keeps refusing (see Guard).
package traffic

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the list of current incidents that anwb.nl/verkeer reads.
const DefaultBaseURL = "https://api.anwb.nl/traffic/traffic-info/v1/incidents/incidents-desktop"

const (
	// MaxRoads is how many roads one widget watches.
	MaxRoads = 10
	// A widget without roads shows what is within a radius of the weather
	// location: the whole of the Netherlands fits in 300 km, but a TV in the
	// hall cares about the way to work.
	MinRadiusKm     = 1
	MaxRadiusKm     = 100
	DefaultRadiusKm = 25
	// MaxItems is how many items of each kind the server keeps for a widget.
	// Widgets show a part of it, and a Cast snapshot has to stay below 64 KB.
	MaxItems = 10
)

// The kinds of information a widget can show.
const (
	ShowJams        = "jams"
	ShowRoadworks   = "roadworks"
	ShowSpeedChecks = "speedchecks"
	ShowCameras     = "cameras"
	ShowTotals      = "totals"
)

// Kinds are the values of a widget's show setting.
var Kinds = []string{ShowJams, ShowRoadworks, ShowSpeedChecks, ShowCameras, ShowTotals}

// DefaultShow is what a widget without a show setting shows.
var DefaultShow = []string{ShowJams, ShowRoadworks, ShowSpeedChecks}

// validRoad matches the motorways (A) and provincial roads (N) that the ANWB
// lists, such as A2 and N279.
var validRoad = regexp.MustCompile(`^[AN][1-9][0-9]{0,2}$`)

// ValidRoad reports whether road is the name of a Dutch A or N road, in
// capitals.
func ValidRoad(road string) bool { return validRoad.MatchString(road) }

// Point is a place on the map.
type Point struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// Incident is one jam, roadwork or speed check as a screen shows it. Texts
// are the ANWB's own, in Dutch.
type Incident struct {
	Road string `json:"road"`
	// From and To are where it starts and ends, such as "Born" and "Urmond".
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// LengthKm is how long a jam is, with one decimal.
	LengthKm float64 `json:"lengthKm,omitempty"`
	// DelayMinutes is the extra travel time through a jam.
	DelayMinutes int `json:"delayMinutes,omitempty"`
	// Reason is the ANWB's description, such as "Stilstaand verkeer.".
	Reason string `json:"reason,omitempty"`
	// Hectometre is the marker post of a speed check, such as 236.1.
	Hectometre float64 `json:"hectometre,omitempty"`
	// Closed is true for a closed road, carriageway or slip road.
	Closed bool `json:"closed,omitempty"`
	// Start and Stop bound roadworks; either can be missing.
	Start *time.Time `json:"start,omitempty"`
	Stop  *time.Time `json:"stop,omitempty"`
	// points are where it is, for the radius filter; screens do not need them.
	points []Point
}

// The kinds of fixed camera.
const (
	// CameraFixed is a fixed speed camera (flitspaal).
	CameraFixed = "fixed"
	// CameraSection is an average speed check over a stretch of road
	// (trajectcontrole).
	CameraSection = "section"
)

// Camera is one fixed speed camera or average speed check, as
// internal/speedcameras reads them from OpenStreetMap.
type Camera struct {
	Kind string `json:"kind"`
	// Roads are the A and N roads it is on, such as ["A2"]; empty for a
	// camera in town.
	Roads []string `json:"roads,omitempty"`
	// Name is the name mappers gave it, if any.
	Name string `json:"name,omitempty"`
	// MaxSpeed is the speed limit it checks in km/h, when tagged.
	MaxSpeed int `json:"maxSpeed,omitempty"`
	// Hectometre is the marker post, when its name holds one ("hm 123.4").
	Hectometre float64 `json:"hectometre,omitempty"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lon"`
}

// Totals are the national figures the ANWB reports for its jams.
type Totals struct {
	Jams         int `json:"jams"`
	LengthKm     int `json:"lengthKm"`
	DelayMinutes int `json:"delayMinutes"`
}

// Report is everything the ANWB reports at one moment, for the whole country.
type Report struct {
	Jams        []Incident
	Roadworks   []Incident
	SpeedChecks []Incident
	Totals      Totals
}

// Query is what one Traffic widget watches: its roads, or without roads
// everything within RadiusKm of the weather location.
type Query struct {
	// Roads are distinct valid road names, sorted.
	Roads    []string
	RadiusKm int
}

// Key identifies a query in the data the server sends. The widget on the TV
// builds the same text from its settings (web/src/widgets/traffic.ts).
func (q Query) Key() string {
	if len(q.Roads) > 0 {
		return "roads:" + strings.Join(q.Roads, ",")
	}
	return "near:" + strconv.Itoa(q.RadiusKm)
}

// NearHome reports whether the query needs the weather location.
func (q Query) NearHome() bool { return len(q.Roads) == 0 }

// NormalizeRoads reads road names as people type them ("a2", " N279 ") into
// distinct capitals, sorted. ok is false when one is not an A or N road or
// there are more than MaxRoads.
func NormalizeRoads(names []string) (roads []string, ok bool) {
	for _, name := range names {
		road := strings.ToUpper(strings.TrimSpace(name))
		if !ValidRoad(road) {
			return nil, false
		}
		roads = append(roads, road)
	}
	slices.Sort(roads)
	roads = slices.Compact(roads)
	return roads, len(roads) <= MaxRoads
}
