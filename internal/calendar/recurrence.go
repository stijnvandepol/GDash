package calendar

import (
	"errors"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

const (
	// maxOccurrencesPerEvent keeps one recurring event from filling the
	// list: a rule that fires every hour would otherwise list hundreds.
	maxOccurrencesPerEvent = 500
	// maxOccurrences bounds the memory one feed can take, whatever number of
	// recurring events it holds.
	maxOccurrences = 10000
	// maxScanned bounds the occurrences the library generates for one event
	// from its start, which it does one at a time. A birthday since 1900
	// needs about a hundred, a COUNT-limited daily event from 1990 about
	// thirteen thousand. Rules that need more are cut short instead of
	// stalling the refresh.
	maxScanned = 20000
	// maxInterval keeps the interval arithmetic inside int64 seconds.
	maxInterval = 1 << 30

	secondsPerHour = 3600
	secondsPerDay  = 24 * secondsPerHour
)

// errTooFrequent is returned for rules that repeat every second or minute,
// which no calendar app can create and which a TV cannot show anyway.
var errTooFrequent = errors.New("recurrence repeats more than hourly")

// expand lists the occurrences of a recurring event that overlap the window,
// at most limit of them. The work is bounded whatever DTSTART, COUNT and
// UNTIL say: the library generates occurrences one by one from DTSTART, so a
// start a century back or a rule without an end would otherwise run for
// minutes on a feed anyone can put in a shared calendar.
//
// rrule-go panics on some rules that pass its checks, such as
// MONTHLY;BYDAY=7MO,10FR;BYSETPOS=1,100,1; that becomes an error, which
// skips the event like any other broken rule.
func expand(e ical.Event, w Window, limit int) (occs []occurrence, err error) {
	defer func() {
		if recover() != nil {
			occs, err = nil, errors.New("recurrence rule cannot be expanded")
		}
	}()
	first, err := single(e, w.Location)
	if err != nil {
		return nil, err
	}
	set, err := e.RecurrenceSet(w.Location)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errors.New("event has no recurrence set")
	}
	duration := first.end.Sub(first.start)
	// Start the search one duration early, so an occurrence that began
	// before the window but still runs is found.
	searchFrom := w.From.Add(-duration)
	if err := boundRule(set, searchFrom, w.To); err != nil {
		return nil, err
	}

	next := set.Iterator()
	for scanned := 0; scanned < maxScanned && len(occs) < min(limit, maxOccurrencesPerEvent); scanned++ {
		start, ok := next()
		if !ok || start.After(w.To) {
			break
		}
		if start.Before(searchFrom) {
			continue
		}
		occ := occurrence{start: start, end: start.Add(duration), allDay: first.allDay}
		if overlaps(occ, w) {
			occs = append(occs, occ)
		}
	}
	return occs, nil
}

// boundRule limits the rule of the set to what is needed to list the period
// from..to: it ends at to, and when the series has no COUNT it starts a
// whole number of periods before from instead of at DTSTART. The occurrences
// from..to stay the same, because the shift is a multiple of the interval
// and keeps the weekday and the time of day.
func boundRule(set *rrule.Set, from, to time.Time) error {
	opt := set.GetRRule().OrigOptions
	if opt.Freq == rrule.SECONDLY || opt.Freq == rrule.MINUTELY {
		return errTooFrequent
	}
	if opt.Until.IsZero() {
		// The library ends a rule 290 years after DTSTART; keep that end
		// when DTSTART moves.
		opt.Until = opt.Dtstart.Add(time.Duration(1<<63 - 1))
	}
	if opt.Until.After(to) {
		opt.Until = to
	}
	if opt.Count == 0 {
		opt.Dtstart = closerStart(opt, from)
	}
	rule, err := rrule.NewRRule(opt)
	if err != nil {
		return err
	}
	set.RRule(rule)
	return nil
}

// closerStart moves DTSTART forward by whole periods to just before from.
// It returns DTSTART itself when the frequency is monthly or yearly (those
// have few periods anyway), when the interval is absurd, or when the new
// start does not exist on the clock of its zone (a daylight saving gap).
func closerStart(opt rrule.ROption, from time.Time) time.Time {
	var unit int64
	switch opt.Freq {
	case rrule.HOURLY:
		unit = secondsPerHour
	case rrule.DAILY:
		unit = secondsPerDay
	case rrule.WEEKLY:
		unit = 7 * secondsPerDay
	default:
		return opt.Dtstart
	}
	interval := int64(max(opt.Interval, 1))
	if interval > maxInterval {
		return opt.Dtstart
	}
	period := interval * unit

	// The rule counts on the clock of DTSTART's zone, so shift there.
	loc := opt.Dtstart.Location()
	startClock, fromClock := clockSeconds(opt.Dtstart, loc), clockSeconds(from, loc)
	// Two periods of margin, so the wall clock and the instant never
	// disagree about whether the new start is still before from.
	periods := (fromClock-startClock)/period - 2
	if periods <= 0 {
		return opt.Dtstart
	}
	shifted := time.Unix(startClock+periods*period, 0).UTC()
	start := time.Date(shifted.Year(), shifted.Month(), shifted.Day(), shifted.Hour(), shifted.Minute(), shifted.Second(), 0, loc)
	if clockSeconds(start, loc) != startClock+periods*period {
		return opt.Dtstart
	}
	return start
}

// clockSeconds is the wall clock of t in loc counted as if it were UTC, so
// that adding a whole number of days keeps the time of day across daylight
// saving changes.
func clockSeconds(t time.Time, loc *time.Location) int64 {
	_, offset := t.In(loc).Zone()
	return t.Unix() + int64(offset)
}
