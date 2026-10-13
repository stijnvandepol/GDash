package traffic

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/geo"
)

// roadworksAhead is how far ahead roadworks are shown: a closure tonight
// matters this morning, one next week does not yet.
const roadworksAhead = 24 * time.Hour

// distanceScale rounds a camera's distance to one decimal.
const distanceScale = 10

// Sources is what the widgets are built from.
type Sources struct {
	// Report is the ANWB's report, or nil while traffic info is off.
	Report *Report
	// Cameras is the camera list; CamerasLoaded is false until there is one.
	Cameras       []Camera
	CamerasLoaded bool
}

// CameraItem is a fixed camera as a screen shows it.
type CameraItem struct {
	Kind string `json:"kind"`
	// Road is the road the widget watches that the camera is on, or the
	// first road it is on.
	Road       string  `json:"road,omitempty"`
	Name       string  `json:"name,omitempty"`
	MaxSpeed   int     `json:"maxSpeed,omitempty"`
	Hectometre float64 `json:"hectometre,omitempty"`
	// DistanceKm is the distance from the weather location, for a widget
	// that watches a radius.
	DistanceKm float64 `json:"distanceKm,omitempty"`
}

// Result is what one widget's query gets: at most MaxItems of each kind.
type Result struct {
	// Incidents is false while ANWB traffic info is off; the lists of
	// jams, roadworks and speed checks are then empty.
	Incidents   bool       `json:"incidents"`
	Jams        []Incident `json:"jams"`
	Roadworks   []Incident `json:"roadworks"`
	SpeedChecks []Incident `json:"speedChecks"`
	// Cameras is nil until the camera list is loaded.
	Cameras []CameraItem `json:"cameras"`
	// CameraCount is how many cameras match, of which Cameras lists the
	// first MaxItems.
	CameraCount int `json:"cameraCount"`
	// Totals are the national figures, while traffic info is on.
	Totals *Totals `json:"totals,omitempty"`
}

// Select picks what a query shows. home is the weather location; a query
// without roads needs it, and matches nothing without it.
func Select(src Sources, q Query, home *Point, now time.Time) Result {
	r := Result{Jams: []Incident{}, Roadworks: []Incident{}, SpeedChecks: []Incident{}}
	near := func(points []Point) bool { return withinRadius(points, q.RadiusKm, home) }
	if src.Report != nil {
		r.Incidents = true
		totals := src.Report.Totals
		r.Totals = &totals
		r.Jams = pickIncidents(src.Report.Jams, q, near, byDelay)
		current := slices.DeleteFunc(slices.Clone(src.Report.Roadworks), func(in Incident) bool { return !soon(in, now) })
		r.Roadworks = pickIncidents(current, q, near, byStart(now))
		r.SpeedChecks = pickIncidents(src.Report.SpeedChecks, q, near, byRoadAndPost)
	}
	if src.CamerasLoaded {
		r.Cameras, r.CameraCount = pickCameras(src.Cameras, q, home)
	}
	return r
}

func pickIncidents(all []Incident, q Query, near func([]Point) bool, order func(a, b Incident) int) []Incident {
	found := []Incident{}
	for _, in := range all {
		if q.NearHome() && near(in.points) || !q.NearHome() && slices.Contains(q.Roads, in.Road) {
			found = append(found, in)
		}
	}
	slices.SortStableFunc(found, order)
	return found[:min(len(found), MaxItems)]
}

func pickCameras(all []Camera, q Query, home *Point) ([]CameraItem, int) {
	found := []CameraItem{}
	for _, c := range all {
		item := CameraItem{Kind: c.Kind, Name: c.Name, MaxSpeed: c.MaxSpeed, Hectometre: c.Hectometre}
		switch {
		case q.NearHome():
			if !withinRadius([]Point{{c.Latitude, c.Longitude}}, q.RadiusKm, home) {
				continue
			}
			item.DistanceKm = math.Round(geo.DistanceKm(home.Latitude, home.Longitude, c.Latitude, c.Longitude)*distanceScale) / distanceScale
			if len(c.Roads) > 0 {
				item.Road = c.Roads[0]
			}
		default:
			i := slices.IndexFunc(c.Roads, func(road string) bool { return slices.Contains(q.Roads, road) })
			if i < 0 {
				continue
			}
			item.Road = c.Roads[i]
		}
		found = append(found, item)
	}
	slices.SortStableFunc(found, func(a, b CameraItem) int {
		return cmp.Or(cmp.Compare(a.DistanceKm, b.DistanceKm), strings.Compare(a.Road, b.Road),
			cmp.Compare(a.Hectometre, b.Hectometre), strings.Compare(a.Name, b.Name))
	})
	return found[:min(len(found), MaxItems)], len(found)
}

// withinRadius reports whether any of the points is within radiusKm of
// home. Without home nothing is.
func withinRadius(points []Point, radiusKm int, home *Point) bool {
	if home == nil {
		return false
	}
	return slices.ContainsFunc(points, func(p Point) bool {
		return geo.DistanceKm(home.Latitude, home.Longitude, p.Latitude, p.Longitude) <= float64(radiusKm)
	})
}

// soon reports whether roadworks are on now or start within roadworksAhead.
func soon(in Incident, now time.Time) bool {
	if in.Stop != nil && !in.Stop.After(now) {
		return false
	}
	return in.Start == nil || in.Start.Before(now.Add(roadworksAhead))
}

// byDelay puts the longest delay first, then the longest jam: what costs
// the most time is what a household wants to know.
func byDelay(a, b Incident) int {
	return cmp.Or(cmp.Compare(b.DelayMinutes, a.DelayMinutes), cmp.Compare(b.LengthKm, a.LengthKm), strings.Compare(a.Road, b.Road))
}

// byStart puts what is on now first, then what starts soonest.
func byStart(now time.Time) func(a, b Incident) int {
	start := func(in Incident) time.Time {
		if in.Start == nil || in.Start.Before(now) {
			return now
		}
		return *in.Start
	}
	return func(a, b Incident) int {
		return cmp.Or(start(a).Compare(start(b)), strings.Compare(a.Road, b.Road), strings.Compare(a.From, b.From))
	}
}

func byRoadAndPost(a, b Incident) int {
	return cmp.Or(strings.Compare(a.Road, b.Road), cmp.Compare(a.Hectometre, b.Hectometre))
}
