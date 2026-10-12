// Package waste turns the events of a waste collection calendar into
// pickups of a known kind. Many municipalities and the sites
// mijnafvalwijzer and afvalkalender offer an iCal link; each event is one
// pickup and its title says which bin.
package waste

import (
	"strings"
	"time"
	"unicode"

	"github.com/stijnvandepol/gdash/internal/calendar"
)

// Kind is the kind of waste a pickup is for. The screen has a colour and a
// plain label for each.
type Kind string

// The kinds of household waste that titles are recognised as. Other is
// anything else, which the screen shows by its own title.
const (
	Organic Kind = "gft"
	Plastic Kind = "pmd"
	Paper   Kind = "paper"
	General Kind = "rest"
	Glass   Kind = "glass"
	Textile Kind = "textile"
	Other   Kind = "other"
)

// maxPickups keeps the snapshot small: a household has a few kinds and a
// pickup of each every week or two, so this is far more than 15 days hold.
const maxPickups = 60

const dateLayout = "2006-01-02"

// Pickup is one collection day of one kind of waste.
type Pickup struct {
	Kind Kind `json:"kind"`
	// Title is the event's own title, shown for kinds that are not
	// recognised.
	Title string `json:"title"`
	// Date is the local day, YYYY-MM-DD.
	Date string `json:"date"`
}

// Report is what the waste source holds: the pickups of each calendar that
// a waste widget shows, by calendar ID as text (JSON keys are text).
type Report map[string][]Pickup

// rule recognises a kind: a title matches when one of its words starts with
// a prefix, or is exactly one of the words. Short terms such as "rest" are
// exact so that "restaurant" is not general waste.
type rule struct {
	kind     Kind
	prefixes []string
	words    []string
}

// rules are tried in order and the first match wins. Glass comes before
// organic so that "Groen glas" (green glass) is not organic waste.
var rules = []rule{
	{Glass, []string{"glas", "glass"}, nil},
	{Paper, []string{"papier", "paper", "karton", "cardboard"}, nil},
	{Textile, []string{"textiel", "textile", "kleding", "clothes", "clothing"}, nil},
	{Plastic, []string{"plastic", "packag", "verpakking", "drankkarton", "blik"}, []string{"pmd"}},
	{General, []string{"restafval", "residual", "general", "huisvuil", "household"}, []string{"rest"}},
	{Organic, []string{"gft", "groen", "green", "organic", "biobak", "bio", "tuinafval", "compost"}, nil},
}

// Classify returns the kind of waste a calendar event title is for, or
// Other. It knows the usual Dutch and English names, such as GFT, PMD,
// papier, restafval, glas, textiel, green, plastic, paper and general.
func Classify(title string) Kind {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, r := range rules {
		for _, w := range words {
			if r.matches(w) {
				return r.kind
			}
		}
	}
	return Other
}

func (r rule) matches(word string) bool {
	for _, p := range r.prefixes {
		if strings.HasPrefix(word, p) {
			return true
		}
	}
	for _, w := range r.words {
		if word == w {
			return true
		}
	}
	return false
}

// Pickups turns calendar events into pickups, in the order of the events,
// which the calendar package sorts by start. An event takes its start day,
// as the day of a pickup does not change with the hour a feed puts on it.
// Events without a readable start are skipped.
func Pickups(events []calendar.Event) []Pickup {
	pickups := []Pickup{}
	for _, e := range events {
		date, ok := localDate(e)
		if !ok {
			continue
		}
		pickups = append(pickups, Pickup{Kind: Classify(e.Title), Title: e.Title, Date: date})
		if len(pickups) == maxPickups {
			break
		}
	}
	return pickups
}

// localDate is the day an event starts on: all-day events carry it as it
// is, timed ones as an RFC 3339 instant in the server's time zone.
func localDate(e calendar.Event) (string, bool) {
	if e.AllDay {
		_, err := time.Parse(dateLayout, e.Start)
		return e.Start, err == nil
	}
	start, err := time.Parse(time.RFC3339, e.Start)
	if err != nil {
		return "", false
	}
	return start.Format(dateLayout), true
}
