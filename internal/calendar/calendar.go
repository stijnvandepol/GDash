// Package calendar reads iCalendar (ICS) feeds, such as the secret iCal
// address of a Google, Outlook or iCloud calendar, and lists the events of
// the coming days with recurrences expanded.
package calendar

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// maxSummaryRunes cuts long titles; a TV shows one line per event.
const maxSummaryRunes = 120

// dateLayout is how all-day events are sent: a date without a time zone, so
// a TV in another zone does not move a birthday to the previous day.
const dateLayout = "2006-01-02"

// Event is one occurrence in the window.
type Event struct {
	Title string `json:"title"`
	// Start and End are RFC 3339 instants, or dates for all-day events. End
	// is exclusive.
	Start    string `json:"start"`
	End      string `json:"end"`
	AllDay   bool   `json:"allDay"`
	Calendar string `json:"calendar"`
	// Color is the calendar's colour on the TV, such as "blue".
	Color string `json:"color,omitempty"`

	sortKey time.Time
}

// Window is the period to list events for. Events that overlap it are
// included, so one that started yesterday and still runs is listed.
type Window struct {
	From time.Time
	To   time.Time
	// Location resolves floating times and all-day dates; usually the
	// server's time zone.
	Location *time.Location
}

type occurrence struct {
	start, end time.Time
	allDay     bool
}

// Parse lists the events of one ICS document in the window, sorted by start.
// fallbackName names the calendar when the feed does not.
func Parse(r io.Reader, w Window, fallbackName string) ([]Event, error) {
	cal, err := ical.NewDecoder(r).Decode()
	if err != nil {
		return nil, fmt.Errorf("parse calendar: %w", err)
	}
	name := fallbackName
	if p := cal.Props.Get("X-WR-CALNAME"); p != nil && strings.TrimSpace(p.Value) != "" {
		name = strings.TrimSpace(p.Value)
	}

	masters, singles, overridden := classify(cal.Events(), w.Location)

	var events []Event
	for _, e := range singles {
		occ, err := single(e, w.Location)
		if err != nil || isCancelled(e) {
			continue
		}
		if overlaps(occ, w) {
			events = append(events, newEvent(e, occ, name, w.Location))
		}
	}
	for _, e := range masters {
		if isCancelled(e) {
			continue
		}
		occs, err := expand(e, w)
		if err != nil {
			// One broken rule must not hide the rest of the calendar.
			continue
		}
		uid := propValue(e.Props, ical.PropUID)
		for _, occ := range occs {
			if !overridden[uid][occ.start.Unix()] {
				events = append(events, newEvent(e, occ, name, w.Location))
			}
		}
	}
	Sort(events)
	return events, nil
}

// classify splits events into recurring masters and single occurrences, and
// records which occurrences of a master an override (RECURRENCE-ID) moved or
// cancelled, so expanding the master skips them.
func classify(all []ical.Event, loc *time.Location) (masters, singles []ical.Event, overridden map[string]map[int64]bool) {
	overridden = map[string]map[int64]bool{}
	for _, e := range all {
		uid := propValue(e.Props, ical.PropUID)
		if rid := e.Props.Get(ical.PropRecurrenceID); rid != nil {
			if t, err := rid.DateTime(loc); err == nil {
				markOverridden(overridden, uid, t)
			}
			singles = append(singles, e)
			continue
		}
		if e.Props.Get(ical.PropRecurrenceRule) != nil || e.Props.Get(ical.PropRecurrenceDates) != nil {
			masters = append(masters, e)
		} else {
			singles = append(singles, e)
		}
	}
	return masters, singles, overridden
}

func markOverridden(overridden map[string]map[int64]bool, uid string, t time.Time) {
	if overridden[uid] == nil {
		overridden[uid] = map[int64]bool{}
	}
	overridden[uid][t.Unix()] = true
}

// Sort orders events by start; all-day events come first on their day.
func Sort(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].sortKey.Equal(events[j].sortKey) {
			return events[i].sortKey.Before(events[j].sortKey)
		}
		return events[i].AllDay && !events[j].AllDay
	})
}

func single(e ical.Event, loc *time.Location) (occurrence, error) {
	start, err := e.DateTimeStart(loc)
	if err != nil {
		return occurrence{}, err
	}
	allDay := isAllDay(e)
	end, err := e.DateTimeEnd(loc)
	if err != nil || !end.After(start) {
		end = defaultEnd(start, allDay)
	}
	return occurrence{start: start, end: end, allDay: allDay}, nil
}

func expand(e ical.Event, w Window) ([]occurrence, error) {
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
	var occs []occurrence
	for _, start := range set.Between(w.From.Add(-duration), w.To, true) {
		occ := occurrence{start: start, end: start.Add(duration), allDay: first.allDay}
		if overlaps(occ, w) {
			occs = append(occs, occ)
		}
	}
	return occs, nil
}

func overlaps(occ occurrence, w Window) bool {
	return occ.start.Before(w.To) && occ.end.After(w.From)
}

func defaultEnd(start time.Time, allDay bool) time.Time {
	if allDay {
		return start.AddDate(0, 0, 1)
	}
	return start
}

func isAllDay(e ical.Event) bool {
	p := e.Props.Get(ical.PropDateTimeStart)
	return p != nil && p.ValueType() == ical.ValueDate
}

func isCancelled(e ical.Event) bool {
	return strings.EqualFold(propValue(e.Props, ical.PropStatus), string(ical.EventCancelled))
}

func propValue(props ical.Props, name string) string {
	if p := props.Get(name); p != nil {
		return p.Value
	}
	return ""
}

// newEvent shows times in loc, so every event of a calendar is written the
// same way whether the feed used UTC or a named zone.
func newEvent(e ical.Event, occ occurrence, calendar string, loc *time.Location) Event {
	title := strings.Join(strings.Fields(propValue(e.Props, ical.PropSummary)), " ")
	if title == "" {
		title = "(no title)"
	}
	if r := []rune(title); len(r) > maxSummaryRunes {
		title = string(r[:maxSummaryRunes-1]) + "…"
	}
	ev := Event{Title: title, AllDay: occ.allDay, Calendar: calendar, sortKey: occ.start}
	if occ.allDay {
		ev.Start, ev.End = occ.start.Format(dateLayout), occ.end.Format(dateLayout)
	} else {
		ev.Start, ev.End = occ.start.In(loc).Format(time.RFC3339), occ.end.In(loc).Format(time.RFC3339)
	}
	return ev
}
