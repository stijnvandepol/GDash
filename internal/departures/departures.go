// Package departures reads real-time public transport departures in the
// Netherlands from OVapi (v0.ovapi.nl), which publishes the KV78Turbo feed
// of buses, trams, metros and ferries without an API key. Trains are not
// in that feed. A widget shows the next departures of one stop area; the
// stop is chosen by searching the list of all stop areas.
package departures

import (
	"regexp"
	"strings"
	"time"
)

// MaxPerStop is how many departures the server keeps for a stop. Widgets
// show a part of it, and a Cast snapshot has to stay below 64 KB.
const MaxPerStop = 20

// Stop is a stop area: the stops of one place that belong together, such
// as both sides of a street or all platforms at a bus station.
type Stop struct {
	Code string `json:"code"`
	// Name includes the town, such as "Utrecht, CS Jaarbeurszijde".
	Name string `json:"name"`
}

// Departure is one vehicle leaving a stop area.
type Departure struct {
	Line        string `json:"line"`
	Destination string `json:"destination"`
	// Expected is when the vehicle leaves according to the real-time
	// data; it equals the timetable when OVapi has no better guess.
	Expected time.Time `json:"expected"`
	// DelayMinutes is how many whole minutes Expected is later than the
	// timetable, or zero when the vehicle is on time or early.
	DelayMinutes int  `json:"delayMinutes,omitempty"`
	Cancelled    bool `json:"cancelled,omitempty"`
}

// validCode matches the stop area codes the web interface can choose. A
// handful of codes in OVapi's list contain spaces or symbols; they would
// need escaping in an address, and none of them has departures anyway.
var validCode = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)

// ValidCode reports whether code can be a stop area code. Only such codes
// go into a request.
func ValidCode(code string) bool { return validCode.MatchString(code) }

// label joins the town and the stop's own name, unless the name already
// starts with the town, as most bus stops do ("Utrecht, CS Centrumzijde").
func label(town, name string) string {
	town, name = strings.TrimSpace(town), strings.TrimSpace(name)
	if town == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(town)) {
		return name
	}
	return town + ", " + name
}
