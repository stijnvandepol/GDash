// Package p2000 reads the alerts of the Dutch emergency services (P2000)
// from the RSS feeds of alarmeringen.nl, per safety region and discipline.
// Feed content is untrusted: texts are reduced to plain text and a place is
// only ever a slug.
package p2000

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// MaxPerFeed is how many alerts of one feed are kept, as many as the feeds
// hold: for fire a few weeks, so that a quiet place still has its last ones.
const MaxPerFeed = 100

// Disciplines are the services a feed can be limited to, in the order the
// editor lists them.
var Disciplines = []string{"brandweer", "politie", "ambulance", "knrm", "trauma"}

// DefaultDisciplines are all but ambulance: ambulance rides are most of the
// alerts and say little about what happens in the street.
var DefaultDisciplines = []string{"brandweer", "politie", "knrm", "trauma"}

// ValidDiscipline reports whether d is one of Disciplines.
func ValidDiscipline(d string) bool { return slices.Contains(Disciplines, d) }

// Alert is one call-out.
type Alert struct {
	// ID is the feed's guid, the same in every feed the alert is in.
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	Discipline string    `json:"discipline"`
	// Place is the slug of the place, as in the alert's link on
	// alarmeringen.nl; a widget for places filters on it.
	Place string `json:"place"`
	// Text is the readable sentence, such as "Woningbrand op Dorpsstraat in
	// Nuenen", not the raw pager message.
	Text string `json:"text"`
}

// Feed is one feed of alerts: one region, one discipline.
type Feed struct {
	Region     string
	Discipline string
}

// Key is how a feed is named in Value.Feeds and by a widget.
func (f Feed) Key() string { return f.Region + "/" + f.Discipline }

// Value is what the server publishes for the P2000 widgets.
type Value struct {
	// Feeds holds the alerts by Feed.Key, newest first.
	Feeds map[string][]Alert `json:"feeds"`
	// Skipped lists the feed keys that were not fetched because the
	// dashboards ask for more feeds than GDash fetches.
	Skipped []string `json:"skipped"`
}

// Place is a place as the place test found it.
type Place struct {
	// Name is the name the admin typed, for the editor and the tile title.
	Name string `json:"name"`
	// Slug is the place in alarmeringen.nl's links.
	Slug string `json:"slug"`
	// Region is the slug of the safety region the place is in.
	Region string `json:"region"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// maxSlugLength is longer than any Dutch place name.
const maxSlugLength = 60

// ValidSlug reports whether s can be a place slug.
func ValidSlug(s string) bool { return len(s) <= maxSlugLength && slugPattern.MatchString(s) }

// accents maps the letters with accents in Dutch place names to plain ones.
var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n", "ĳ", "ij",
)

// SlugOf turns a place name as people write it into the slug alarmeringen.nl
// uses: "Hengelo (Ov)" is hengelo-ov, "'s-Hertogenbosch" s-hertogenbosch.
// The result may still be no slug at all; see ValidSlug.
func SlugOf(name string) string {
	plain := accents.Replace(strings.ToLower(strings.TrimSpace(name)))
	var b strings.Builder
	dash := false
	for _, r := range plain {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
