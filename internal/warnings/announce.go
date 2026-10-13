package warnings

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

// Notice is a code orange or red warning that has just come into effect.
type Notice struct {
	Region  string
	Warning Warning
}

// Announcer remembers which severe warnings are in effect per region, and
// reports the ones that come into effect. The first time a region is seen,
// such as after a restart or when a widget starts to announce, only counts
// as its state: a warning already in effect is not news.
type Announcer struct {
	mu sync.Mutex
	// inEffect holds, per region, the warnings in effect by noticeKey.
	inEffect map[string]map[string]bool
}

// noticeKey is a warning's identity and level: a yellow warning that the
// KNMI raises to orange, or orange to red, is announced again; the same
// warning with a new end time is not.
func noticeKey(w Warning) string { return w.ID + "#" + w.Level }

// Check takes the warnings of the regions whose widgets announce and
// returns the code orange and red ones that came into effect since the
// last check, at most once each. Regions that are no longer given are
// forgotten, so the memory stays as small as the dashboards.
func (a *Announcer) Check(areas []Area, now time.Time) []Notice {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := map[string]map[string]bool{}
	var out []Notice
	for _, area := range areas {
		if next[area.Region] != nil {
			continue
		}
		prev, known := a.inEffect[area.Region]
		current := map[string]bool{}
		for _, w := range area.Warnings {
			if !w.Severe() || !w.InEffect(now) {
				continue
			}
			key := noticeKey(w)
			if known && !prev[key] && !current[key] {
				out = append(out, Notice{Region: area.Region, Warning: w})
			}
			current[key] = true
		}
		next[area.Region] = current
	}
	a.inEffect = next
	slices.SortFunc(out, func(x, y Notice) int {
		return cmp.Or(cmp.Compare(x.Region, y.Region), x.Warning.Onset.Compare(y.Warning.Onset))
	})
	return out
}

// codeNames name the levels that are announced, per display language.
var codeNames = map[string]map[string]string{
	"en": {LevelOrange: "Code orange", LevelRed: "Code red"},
	"nl": {LevelOrange: "Code oranje", LevelRed: "Code rood"},
	"de": {LevelOrange: "Warnstufe Orange", LevelRed: "Warnstufe Rot"},
	"fr": {LevelOrange: "Vigilance orange", LevelRed: "Vigilance rouge"},
}

// Text is the title and message that announce n in language. The KNMI
// writes its texts in Dutch and English, so a German or French screen gets
// the English headline under a title in its own language.
func (n Notice) Text(language string) (title, message string) {
	names, ok := codeNames[language]
	if !ok {
		names = codeNames["en"]
	}
	title = names[n.Warning.Level] + " · " + n.Region
	text := language
	if text != "nl" {
		text = "en"
	}
	message = cmp.Or(n.Warning.Headline[text], n.Warning.Event[text], n.Warning.Headline["en"], n.Warning.Event["en"], n.Warning.Kind, title)
	return title, message
}
