package departures

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// The list of stop areas changes a few times a year.
	refreshAfter = 24 * time.Hour
	// A failed refresh keeps the old list and tries again after this, so a
	// person typing in the editor does not make a request per keystroke.
	retryAfter = 10 * time.Minute
	// maxResults is what the editor lists; more would need scrolling on a
	// phone for a search that should be narrowed anyway.
	maxResults = 20
)

// StopLister loads the list of all stop areas.
type StopLister interface {
	Stops(ctx context.Context) ([]Stop, error)
}

// Directory searches the stop areas by name. It loads the list when first
// searched and again once a day, and searches it in memory.
type Directory struct {
	Source StopLister
	Now    func() time.Time
	Logger *slog.Logger

	mu       sync.Mutex
	entries  []entry
	nextLoad time.Time
}

// entry is a stop with what searching needs, worked out once per load.
type entry struct {
	stop   Stop
	lower  string
	tokens []string
}

// Search returns the stops whose name contains every word of query, best
// matches first: words that start a word of the name come before words
// found in the middle. It fails only when no list has ever been loaded.
func (d *Directory) Search(ctx context.Context, query string) ([]Stop, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return []Stop{}, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.refresh(ctx); err != nil {
		return nil, err
	}
	type match struct {
		stop Stop
		rank int
	}
	var found []match
	for _, e := range d.entries {
		if rank, ok := e.rank(words); ok {
			found = append(found, match{e.stop, rank})
		}
	}
	slices.SortFunc(found, func(a, b match) int {
		return cmp.Or(cmp.Compare(a.rank, b.rank), strings.Compare(a.stop.Name, b.stop.Name), strings.Compare(a.stop.Code, b.stop.Code))
	})
	found = found[:min(len(found), maxResults)]
	stops := make([]Stop, len(found))
	for i, m := range found {
		stops[i] = m.stop
	}
	return stops, nil
}

// refresh loads the list when there is none or it is due. A failure with an
// older list in hand is logged and the older list stays in use.
func (d *Directory) refresh(ctx context.Context) error {
	now := d.Now()
	if d.entries != nil && now.Before(d.nextLoad) {
		return nil
	}
	stops, err := d.Source.Stops(ctx)
	if err != nil {
		if d.entries == nil {
			return fmt.Errorf("load stop areas: %w", err)
		}
		d.Logger.Warn("could not refresh the list of stop areas; using the previous one", "error", err)
		d.nextLoad = now.Add(retryAfter)
		return nil
	}
	entries := make([]entry, len(stops))
	for i, s := range stops {
		lower := strings.ToLower(s.Name)
		entries[i] = entry{stop: s, lower: lower, tokens: strings.FieldsFunc(lower, isSeparator)}
	}
	d.entries, d.nextLoad = entries, now.Add(refreshAfter)
	return nil
}

func isSeparator(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }

// rank is 0 when every word starts a word of the name ("utr cs" finds
// "Utrecht, CS Centrumzijde"), 1 when every word is found somewhere in the
// name, and not ok when a word is missing.
func (e entry) rank(words []string) (rank int, ok bool) {
	for _, w := range words {
		if !strings.Contains(e.lower, w) {
			return 0, false
		}
		if !slices.ContainsFunc(e.tokens, func(t string) bool { return strings.HasPrefix(t, w) }) {
			rank = 1
		}
	}
	return rank, true
}
