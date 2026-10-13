package traffic

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// The ANWB gives the length of a jam in metres and its delay in
	// seconds; its national totals are in kilometres and minutes already.
	metresPerKm      = 1000
	secondsPerMinute = 60
	lengthScale      = 10
	// maxPlaceRunes and maxReasonRunes keep one odd text from filling a
	// tile or the snapshot.
	maxPlaceRunes  = 80
	maxReasonRunes = 200
	// Sane bounds: anything outside is a broken answer, not traffic.
	maxJamMetres    = 500 * metresPerKm
	maxDelaySeconds = 24 * 60 * 60
	maxHectometre   = 1000
)

// answer is the part of the ANWB's answer that GDash reads. Unknown fields
// are ignored, so the service can add some without breaking the widget.
type answer struct {
	Roads *[]struct {
		Road     string `json:"road"`
		Segments []struct {
			// Each item is read on its own, so one odd item is left out
			// instead of failing the whole answer.
			Jams      []json.RawMessage `json:"jams"`
			Roadworks []json.RawMessage `json:"roadworks"`
			Radars    []json.RawMessage `json:"radars"`
		} `json:"segments"`
	} `json:"roads"`
	Totals struct {
		All struct {
			Count    float64 `json:"count"`
			Distance float64 `json:"distance"`
			Delay    float64 `json:"delay"`
		} `json:"all"`
	} `json:"totals"`
}

type rawIncident struct {
	Road         string    `json:"road"`
	IncidentType string    `json:"incidentType"`
	Label        string    `json:"label"`
	From         string    `json:"from"`
	To           string    `json:"to"`
	FromLoc      *rawPoint `json:"fromLoc"`
	ToLoc        *rawPoint `json:"toLoc"`
	Loc          *rawPoint `json:"loc"`
	Reason       string    `json:"reason"`
	// Distance is metres and Delay seconds (checked against the national
	// totals on 2026-10-13: 15 jams, 40500 m and 3240 s against totals of
	// 41 km and 54 minutes).
	Distance float64 `json:"distance"`
	Delay    float64 `json:"delay"`
	HM       float64 `json:"HM"`
	Start    string  `json:"start"`
	Stop     string  `json:"stop"`
}

type rawPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// decode reads an ANWB answer. An answer without a road list is an error,
// so a changed or broken service keeps the last good report on screen
// instead of showing "no jams". An empty list is real: a quiet night.
func decode(body []byte) (Report, error) {
	var a answer
	if err := json.Unmarshal(body, &a); err != nil {
		return Report{}, fmt.Errorf("decode anwb traffic answer: %w", err)
	}
	if a.Roads == nil {
		return Report{}, errors.New("anwb traffic answer has no road list")
	}
	report := Report{
		Jams: []Incident{}, Roadworks: []Incident{}, SpeedChecks: []Incident{},
		Totals: Totals{Jams: wholeOrZero(a.Totals.All.Count), LengthKm: wholeOrZero(a.Totals.All.Distance), DelayMinutes: wholeOrZero(a.Totals.All.Delay)},
	}
	for _, road := range *a.Roads {
		for _, segment := range road.Segments {
			report.Jams = appendIncidents(report.Jams, road.Road, segment.Jams)
			report.Roadworks = appendIncidents(report.Roadworks, road.Road, segment.Roadworks)
			report.SpeedChecks = appendIncidents(report.SpeedChecks, road.Road, segment.Radars)
		}
	}
	return report, nil
}

func appendIncidents(list []Incident, road string, raw []json.RawMessage) []Incident {
	for _, item := range raw {
		var r rawIncident
		if err := json.Unmarshal(item, &r); err != nil {
			continue
		}
		if in, ok := r.incident(road); ok {
			list = append(list, in)
		}
	}
	return list
}

// incident turns what the ANWB sends into what a screen shows. An item
// without a road it belongs to is left out; anything else that is missing
// or out of bounds is left empty.
func (r rawIncident) incident(parentRoad string) (Incident, bool) {
	road := strings.ToUpper(cleanText(r.Road, maxPlaceRunes))
	if road == "" {
		road = strings.ToUpper(cleanText(parentRoad, maxPlaceRunes))
	}
	if road == "" {
		return Incident{}, false
	}
	in := Incident{
		Road:   road,
		From:   cleanText(r.From, maxPlaceRunes),
		To:     cleanText(r.To, maxPlaceRunes),
		Reason: cleanText(r.Reason, maxReasonRunes),
		Closed: r.IncidentType == "road-closed" || r.Label == "closed",
		Start:  parseTime(r.Start),
		Stop:   parseTime(r.Stop),
	}
	if r.Distance > 0 && r.Distance <= maxJamMetres {
		in.LengthKm = math.Round(r.Distance/metresPerKm*lengthScale) / lengthScale
	}
	if r.Delay > 0 && r.Delay <= maxDelaySeconds {
		in.DelayMinutes = int(math.Round(r.Delay / secondsPerMinute))
	}
	if r.HM > 0 && r.HM < maxHectometre {
		in.Hectometre = math.Round(r.HM*lengthScale) / lengthScale
	}
	for _, p := range []*rawPoint{r.Loc, r.FromLoc, r.ToLoc} {
		if p != nil && p.Lat >= -90 && p.Lat <= 90 && p.Lon >= -180 && p.Lon <= 180 && (p.Lat != 0 || p.Lon != 0) {
			in.points = append(in.points, Point{Latitude: p.Lat, Longitude: p.Lon})
		}
	}
	return in, true
}

// cleanText trims a text, drops control characters and cuts it at limit
// characters.
func cleanText(s string, limit int) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s))
	if utf8.RuneCountInString(s) > limit {
		s = strings.TrimSpace(string([]rune(s)[:limit]))
	}
	return s
}

func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// wholeOrZero rounds a count from the answer, or gives 0 for one that is
// not a sane number.
func wholeOrZero(v float64) int {
	if math.IsNaN(v) || v < 0 || v > math.MaxInt32 {
		return 0
	}
	return int(math.Round(v))
}
