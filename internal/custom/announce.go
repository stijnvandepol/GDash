package custom

import (
	"strconv"
	"sync"
	"time"
)

// AnnounceGap is the least time between two announcements of one rule, so
// a value that flips back and forth cannot flood the screens.
const AnnounceGap = 10 * time.Minute

// Target is a Custom widget with announcing rules and its source's answer.
type Target struct {
	// Key identifies the widget, such as "kitchen/custom-1".
	Key    string
	Widget Widget
	Doc    any
}

// Announcement is a rule that just became true.
type Announcement struct {
	Key     string
	Title   string
	Message string
}

// Announcer remembers which announcing rules hold, and reports the ones
// that start to hold. The first time a rule is seen, such as after a
// restart, only counts as its state: a server that restarts must not
// announce everything that is true at that moment again.
type Announcer struct {
	Gap time.Duration

	mu    sync.Mutex
	rules map[string]ruleState
}

type ruleState struct {
	holds  bool
	posted time.Time
}

// Check evaluates the announcing rules of targets at now and returns the
// announcements to post. Rules of widgets that are no longer in targets
// are forgotten, so the memory stays as small as the dashboards.
func (a *Announcer) Check(targets []Target, now time.Time) []Announcement {
	a.mu.Lock()
	defer a.mu.Unlock()
	seen := map[string]ruleState{}
	var out []Announcement
	for _, t := range targets {
		matched := t.Widget.Matching(t.Doc, now)
		for i, rule := range t.Widget.Rules {
			if !rule.Announce {
				continue
			}
			// The condition is part of the key: an edited rule starts over.
			key := t.Key + "#" + strconv.Itoa(i) + "#" + rule.When.String()
			prev, known := a.rules[key]
			state := ruleState{holds: matched[i], posted: prev.posted}
			if known && !prev.holds && matched[i] && (prev.posted.IsZero() || now.Sub(prev.posted) >= a.Gap) {
				state.posted = now
				out = append(out, Announcement{Key: t.Key, Title: t.Widget.Title, Message: rule.Message})
			}
			seen[key] = state
		}
	}
	a.rules = seen
	return out
}
