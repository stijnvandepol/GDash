// Package speedcameras reads the fixed speed cameras and average speed
// checks (trajectcontroles) in the Netherlands from OpenStreetMap, through
// the public Overpass API. The data is © OpenStreetMap contributors under
// the Open Database License; screens that show it say so.
//
// Fixed cameras hardly move, so the list is fetched at most once a week and
// kept on disk (see Store): one query of the whole country, as the Overpass
// usage policy prefers over many small ones.
package speedcameras

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/stijnvandepol/gdash/internal/traffic"
)

// DefaultBaseURL is the main public Overpass instance.
const DefaultBaseURL = "https://overpass-api.de/api/interpreter"

// maxTaggedSpeed is the highest speed limit that is a real one.
const maxTaggedSpeed = 200

// roadInName finds a road in a name such as "Trajectcontrole A2 Vught".
var roadInName = regexp.MustCompile(`\b([AN][1-9][0-9]{0,2})\b`)

// hectometreInName finds a marker post in a name such as "A2 hm 123,4".
var hectometreInName = regexp.MustCompile(`(?i)\bhmp?\.?\s*([0-9]{1,3}[.,][0-9])\b`)

// roadsFromRefs reads A and N roads from ref tags such as "A2;E25",
// distinct and sorted.
func roadsFromRefs(refs []string) []string {
	var roads []string
	for _, ref := range refs {
		for part := range strings.SplitSeq(ref, ";") {
			part = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(part), " ", ""))
			if traffic.ValidRoad(part) {
				roads = append(roads, part)
			}
		}
	}
	slices.Sort(roads)
	return slices.Compact(roads)
}

// hectometre reads a marker post from a name, or 0.
func hectometre(name string) float64 {
	m := hectometreInName.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	hm, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if err != nil {
		return 0
	}
	return hm
}

// maxSpeed reads a maxspeed tag in km/h, such as "100", or 0.
func maxSpeed(tag string) int {
	n, err := strconv.Atoi(strings.TrimSpace(tag))
	if err != nil || n <= 0 || n > maxTaggedSpeed {
		return 0
	}
	return n
}
