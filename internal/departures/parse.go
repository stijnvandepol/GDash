package departures

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// timeLayout is how OVapi writes times: local time without an offset.
const timeLayout = "2006-01-02T15:04:05"

// amsterdam is the zone of every time in the feed. The server's own zone
// does not matter: a GDash on a VPS in UTC must still read 11:46 as 11:46
// in the Netherlands.
var amsterdam = sync.OnceValues(func() (*time.Location, error) {
	return time.LoadLocation("Europe/Amsterdam")
})

// Statuses and journey stop types of KV78Turbo that decide whether a pass
// is a departure.
const (
	statusPassed    = "PASSED"
	statusCancelled = "CANCEL"
	// stopLast means the journey ends here: the vehicle arrives and nobody
	// boards. Its "departure" time is the arrival.
	stopLast = "LAST"
)

// apiPass is the part of a KV78Turbo pass that GDash uses. A pass is one
// vehicle at one stop.
type apiPass struct {
	Line                  string `json:"LinePublicNumber"`
	Destination           string `json:"DestinationName50"`
	Status                string `json:"TripStopStatus"`
	StopType              string `json:"JourneyStopType"`
	TargetDepartureTime   string `json:"TargetDepartureTime"`
	ExpectedDepartureTime string `json:"ExpectedDepartureTime"`
}

// apiTimingPoint is one stop (one side of the street) in a stop area.
type apiTimingPoint struct {
	Passes map[string]apiPass `json:"Passes"`
}

// apiAnswer is the answer to /stopareacode/<codes>: stop area code to
// timing point code to stop.
type apiAnswer map[string]map[string]apiTimingPoint

// parseBoards turns an answer into the departures of each requested stop
// area, soonest first and at most MaxPerStop. Every requested code gets an
// entry, empty when OVapi knows no departures, so a screen can tell a quiet
// stop from one that has not loaded. now drops vehicles that have left.
func parseBoards(body []byte, codes []string, now time.Time) (map[string][]Departure, error) {
	var answer apiAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode ovapi departures: %w", err)
	}
	zone, err := amsterdam()
	if err != nil {
		return nil, fmt.Errorf("load the Amsterdam time zone (does this system have time zone data?): %w", err)
	}
	boards := make(map[string][]Departure, len(codes))
	for _, code := range codes {
		board := []Departure{}
		for _, point := range answer[code] {
			for key, pass := range point.Passes {
				d, ok, err := pass.departure(zone, now)
				if err != nil {
					return nil, fmt.Errorf("stop %s, pass %s: %w", code, key, err)
				}
				if ok {
					board = append(board, d)
				}
			}
		}
		slices.SortFunc(board, compareDepartures)
		boards[code] = board[:min(len(board), MaxPerStop)]
	}
	return boards, nil
}

// departure converts a pass, or reports false for one that is not a
// departure to show: a vehicle that ends its journey here, one that has
// passed, one that left a minute or more ago, or a pass without times. A
// time that is there but not in OVapi's format is an error: the feed has
// changed, and a board that silently empties would hide that.
func (p apiPass) departure(zone *time.Location, now time.Time) (Departure, bool, error) {
	if p.Status == statusPassed || p.StopType == stopLast || p.Line == "" || p.TargetDepartureTime == "" || p.ExpectedDepartureTime == "" {
		return Departure{}, false, nil
	}
	planned, err := time.ParseInLocation(timeLayout, p.TargetDepartureTime, zone)
	if err != nil {
		return Departure{}, false, fmt.Errorf("planned time: %w", err)
	}
	expected, err := time.ParseInLocation(timeLayout, p.ExpectedDepartureTime, zone)
	if err != nil {
		return Departure{}, false, fmt.Errorf("expected time: %w", err)
	}
	// Whole minutes, as a timetable shows them: 11:29:01 is not late for
	// a bus planned at 11:29.
	if expected.Truncate(time.Minute).Before(now.Truncate(time.Minute)) {
		return Departure{}, false, nil
	}
	return Departure{
		Line:         strings.TrimSpace(p.Line),
		Destination:  strings.TrimSpace(p.Destination),
		Expected:     expected,
		DelayMinutes: delayMinutes(planned, expected),
		Cancelled:    p.Status == statusCancelled,
	}, true, nil
}

// delayMinutes is how many whole minutes expected is after planned; zero
// when on time or early, because an early guess is not something to warn
// about.
func delayMinutes(planned, expected time.Time) int {
	late := expected.Truncate(time.Minute).Sub(planned.Truncate(time.Minute))
	return max(0, int(late/time.Minute))
}

// compareDepartures orders by departure time and then by line and
// destination, so equal minutes always come out in the same order.
func compareDepartures(a, b Departure) int {
	return cmp.Or(
		a.Expected.Compare(b.Expected),
		strings.Compare(a.Line, b.Line),
		strings.Compare(a.Destination, b.Destination),
	)
}
