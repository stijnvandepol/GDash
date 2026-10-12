package calendar

import (
	"github.com/emersion/go-ical"
	"strconv"
	"strings"
	"testing"
	"time"
)

// vevent wraps properties in a minimal calendar.
func vevent(props ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:x\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTAMP:20261001T120000Z\r\n" +
		strings.Join(props, "\r\n") + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

// parseOne parses a calendar with one event built from the properties.
func parseOne(t *testing.T, props ...string) []Event {
	t.Helper()
	events, err := Parse(strings.NewReader(vevent(props...)), window(t), "Test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return events
}

func numbers(from, to int) string {
	list := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		list = append(list, strconv.Itoa(i))
	}
	return strings.Join(list, ",")
}

func TestParseBoundsHostileRecurrences(t *testing.T) {
	tests := []struct {
		name  string
		props []string
	}{
		{"per second since 1990", []string{"DTSTART:19900101T000000Z", "RRULE:FREQ=SECONDLY"}},
		{"per minute since year 1", []string{"DTSTART:00010101T000000Z", "RRULE:FREQ=MINUTELY"}},
		{"per second with a huge count", []string{"DTSTART:20260101T000000Z", "RRULE:FREQ=SECONDLY;COUNT=2000000000"}},
		{"daily since year 1", []string{"DTSTART:00010101T080000Z", "RRULE:FREQ=DAILY"}},
		{"daily since year 1 with a huge count", []string{"DTSTART:00010101T080000Z", "RRULE:FREQ=DAILY;COUNT=2000000000"}},
		{"hourly with every minute", []string{"DTSTART:19900101T000000Z", "RRULE:FREQ=HOURLY;BYMINUTE=" + numbers(0, 59)}},
		{"monthly with every day since year 1", []string{"DTSTART:00010101T080000Z", "RRULE:FREQ=MONTHLY;BYMONTHDAY=" + numbers(1, 31)}},
		{"yearly with every day since year 1", []string{"DTSTART:00010101T080000Z", "RRULE:FREQ=YEARLY;BYYEARDAY=" + numbers(1, 366)}},
		{"daily that never matches", []string{"DTSTART:00010101T080000Z", "RRULE:FREQ=DAILY;BYMONTH=2;BYMONTHDAY=30"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			began := time.Now()
			events := parseOne(t, tt.props...)
			// An unbounded expansion costs seconds to minutes; a second
			// leaves room for a busy machine without missing that.
			if took := time.Since(began); took > raceSlowdown*time.Second {
				t.Errorf("Parse took %v, want under a second", took)
			}
			if len(events) > maxOccurrencesPerEvent {
				t.Errorf("one event produced %d occurrences, want at most %d", len(events), maxOccurrencesPerEvent)
			}
		})
	}
}

func TestParseSkipsRulesThatPanicTheLibrary(t *testing.T) {
	broken := vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=MONTHLY;BYDAY=7MO,10FR;BYSETPOS=1,0100,01", "SUMMARY:Broken")
	fine := "BEGIN:VEVENT\r\nUID:b\r\nDTSTART:20261009T090000Z\r\nSUMMARY:Fine\r\nEND:VEVENT\r\n"
	ics := strings.Replace(broken, "END:VCALENDAR", fine+"END:VCALENDAR", 1)
	events, err := Parse(strings.NewReader(ics), window(t), "Test")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Title != "Fine" {
		t.Fatalf("events = %+v, want only the event with a working rule", events)
	}
}

func TestParseKeepsOldSeriesInTheWindow(t *testing.T) {
	tests := []struct {
		name  string
		props []string
		want  []string
	}{
		{"yearly birthday since 1960", []string{"DTSTART;VALUE=DATE:19601010", "RRULE:FREQ=YEARLY", "SUMMARY:Grandma"}, []string{"2026-10-10"}},
		{"daily since 1990 with an end after the window", []string{"DTSTART:19900101T090000Z", "RRULE:FREQ=DAILY;UNTIL=20261010T090000Z"}, []string{
			"2026-10-08T11:00:00+02:00", "2026-10-09T11:00:00+02:00", "2026-10-10T11:00:00+02:00"}},
		{"daily since 1990 with a count that is over", []string{"DTSTART:19900101T090000Z", "RRULE:FREQ=DAILY;COUNT=100"}, nil},
		{"every second week since 2010 on Monday and Wednesday", []string{"DTSTART:20100111T090000Z", "RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE"}, []string{
			"2026-10-12T11:00:00+02:00", "2026-10-14T11:00:00+02:00"}},
		{"every three hours since 2020", []string{"DTSTART:20200101T000000Z", "DTEND:20200101T003000Z", "RRULE:FREQ=HOURLY;INTERVAL=3;UNTIL=20261008T040000Z"}, []string{
			"2026-10-08T02:00:00+02:00", "2026-10-08T05:00:00+02:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, e := range parseOne(t, tt.props...) {
				got = append(got, e.Start)
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("starts:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// TestExpandMatchesUnboundedExpansion compares the bounded expansion with the
// library's own, which is exact but walks every occurrence from DTSTART. The
// starts are a few years back, which the library still handles quickly.
func TestExpandMatchesUnboundedExpansion(t *testing.T) {
	zones := []string{"", ";TZID=Europe/Amsterdam", ";TZID=America/New_York"}
	rules := []string{
		"FREQ=HOURLY", "FREQ=HOURLY;INTERVAL=5", "FREQ=HOURLY;BYMINUTE=0,30;INTERVAL=7",
		"FREQ=DAILY", "FREQ=DAILY;INTERVAL=3", "FREQ=DAILY;BYHOUR=8,20;INTERVAL=2", "FREQ=DAILY;BYDAY=MO,TU,FR",
		"FREQ=WEEKLY", "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,TH", "FREQ=WEEKLY;INTERVAL=3;WKST=SU;BYDAY=SU,SA",
		"FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR,SA,SU", "FREQ=MONTHLY", "FREQ=MONTHLY;BYDAY=2TU", "FREQ=YEARLY",
		"FREQ=DAILY;UNTIL=20261012T000000Z", "FREQ=DAILY;COUNT=1500",
	}
	starts := []string{"20230714T233000", "20240229T000000", "20250330T023000", "20241027T013000", "20260930T120000"}
	for _, zone := range zones {
		for _, rule := range rules {
			for _, start := range starts {
				// One hour long, so events that began before the window are found too.
				props := []string{"DTSTART" + zone + ":" + start, "DTEND" + zone + ":" + start[:9] + "5" + start[10:], "RRULE:" + rule}
				e := decodeEvent(t, vevent(props...))
				w := window(t)
				got, err := expand(e, w, maxOccurrences)
				if err != nil {
					t.Fatalf("%v: %v", props, err)
				}
				want := unboundedExpand(t, e, w)
				if len(got) != len(want) {
					t.Errorf("%v: %d occurrences, want %d", props, len(got), len(want))
					continue
				}
				for i := range got {
					if !got[i].start.Equal(want[i].start) {
						t.Errorf("%v: occurrence %d starts %v, want %v", props, i, got[i].start, want[i].start)
					}
				}
			}
		}
	}
}

func decodeEvent(t *testing.T, ics string) ical.Event {
	t.Helper()
	cal, err := ical.NewDecoder(strings.NewReader(ics)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	return cal.Events()[0]
}

// unboundedExpand is the library's Between over the whole series.
func unboundedExpand(t *testing.T, e ical.Event, w Window) []occurrence {
	t.Helper()
	first, err := single(e, w.Location)
	if err != nil {
		t.Fatal(err)
	}
	set, err := e.RecurrenceSet(w.Location)
	if err != nil {
		t.Fatal(err)
	}
	duration := first.end.Sub(first.start)
	var occs []occurrence
	for _, start := range set.Between(w.From.Add(-duration), w.To, true) {
		occ := occurrence{start: start, end: start.Add(duration), allDay: first.allDay}
		if overlaps(occ, w) {
			occs = append(occs, occ)
		}
	}
	return occs
}
