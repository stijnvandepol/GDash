package calendar

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// maxParseTime is how long one feed may take. The feed is arbitrary bytes
// from an address an admin typed in, so a rule that expands for seconds
// would stall the refresh of every display.
const maxParseTime = time.Second

func FuzzParse(f *testing.F) {
	family, err := os.ReadFile("testdata/family.ics")
	if err != nil {
		f.Fatal(err)
	}
	seeds := []string{
		string(family),
		"",
		"BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		"<html>login page</html>",
		"BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n",
		vevent("DTSTART:20261009T080000Z", "SUMMARY:\x00nul\x00 bytes"),
		vevent("DTSTART:20261009T080000Z", "SUMMARY:\xff\xfe invalid \xc3\x28 utf8"),
		vevent("DTSTART:20261009T080000Z", "SUMMARY:"+strings.Repeat("long title ", 5000)),
		// Recurrences with no end, from far in the past.
		vevent("DTSTART:20000101T000000Z", "RRULE:FREQ=SECONDLY"),
		vevent("DTSTART:00010101T000000Z", "RRULE:FREQ=SECONDLY"),
		vevent("DTSTART:19700101T000000Z", "DTEND:19700101T000001Z", "RRULE:FREQ=SECONDLY;INTERVAL=1"),
		vevent("DTSTART:20000101T000000Z", "RRULE:FREQ=MINUTELY;BYSECOND=0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15"),
		vevent("DTSTART:20000101T000000Z", "RRULE:FREQ=HOURLY;BYMINUTE=0,1,2,3,4,5,6,7,8,9;BYSECOND=0,1,2,3,4,5"),
		vevent("DTSTART:20000101T000000Z", "RRULE:FREQ=DAILY;BYHOUR=0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23;BYMINUTE=0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29;BYSECOND=0,1,2,3,4,5,6,7,8,9"),
		// Rules that never match.
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=YEARLY;BYWEEKNO=53;BYDAY=MO;BYMONTH=2"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=MONTHLY;INTERVAL=2147483647"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;INTERVAL=0"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;INTERVAL=-1"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;COUNT=-5"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;COUNT=99999999999999999999"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;UNTIL=00010101T000000Z"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;UNTIL=99991231T235959Z"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR,SA,SU,1MO,-1MO,2TU,-2TU,53MO,-53SU"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=MONTHLY;BYDAY=5MO,-5FR;BYSETPOS=1,-1,366,-366"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=YEARLY;BYYEARDAY=1,100,366,-1,-366;BYMONTHDAY=-31,31"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=BOGUS"),
		vevent("DTSTART:20261009T080000Z", "RRULE:"),
		vevent("DTSTART:20261009T080000Z", "RRULE:FREQ=DAILY;FREQ=WEEKLY;WKST=XX"),
		// Odd dates and durations.
		vevent("DTSTART:00000101T000000Z"),
		vevent("DTSTART:00010101T000000Z", "DTEND:99991231T235959Z"),
		vevent("DTSTART:99991231T235959Z", "DTEND:99991231T235959Z", "RRULE:FREQ=YEARLY"),
		vevent("DTSTART:20261301T250000Z"),
		vevent("DTSTART:20260230T080000Z"),
		vevent("DTSTART;VALUE=DATE:00010101", "RRULE:FREQ=DAILY"),
		vevent("DTSTART;VALUE=DATE:99991231", "RRULE:FREQ=DAILY"),
		vevent("DTSTART;VALUE=DATE:20261009", "DTEND;VALUE=DATE:20261001"),
		vevent("DTSTART:20261009T080000Z", "DURATION:-PT1H"),
		vevent("DTSTART:20261009T080000Z", "DTEND:20261009T070000Z", "RRULE:FREQ=DAILY"),
		vevent("DTSTART:20261009T080000Z", "DTEND:20261009T080000Z", "RRULE:FREQ=HOURLY"),
		vevent("DTSTART:20261009T080000Z", "DTEND:20991231T080000Z", "RRULE:FREQ=DAILY"),
		vevent("DTSTART;TZID=Nowhere/Land:20261009T080000", "RRULE:FREQ=DAILY"),
		vevent("DTSTART;TZID=Europe/Amsterdam:20261025T023000", "RRULE:FREQ=DAILY"),
		vevent("DTSTART;TZID=Europe/Amsterdam:20260329T023000", "RRULE:FREQ=YEARLY"),
		vevent("DTSTART;TZID=../../etc/passwd:20261009T080000"),
		vevent("DTSTART:20261009T080000", "EXDATE:20261009T080000", "RDATE:20261010T080000,20261011T080000"),
		vevent("DTSTART:20261009T080000Z", "RDATE;VALUE=PERIOD:20261010T080000Z/PT1H"),
		vevent("DTSTART:20261009T080000Z", "RDATE:"+strings.Repeat("20261010T080000Z,", 3000)),
		vevent("DTSTART:20261009T080000Z", "EXDATE:"+strings.Repeat("20261010T080000Z,", 3000), "RRULE:FREQ=DAILY"),
		vevent("DTSTART:20261009T080000Z", "STATUS:CANCELLED"),
		vevent("DTSTART:20261009T080000Z", "RECURRENCE-ID:20261009T080000Z"),
		vevent("DTSTART:20261009T080000Z", "RECURRENCE-ID;RANGE=THISANDFUTURE:20261009T080000Z", "RRULE:FREQ=DAILY"),
		// Structure: folding, nesting, many properties.
		"BEGIN:VCALENDAR\r\nBEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nDTSTART:20261009T080000Z\r\nSUMMARY:fold\r\n ded\r\n\tmore\r\nEND:VEVENT\r\nEND:VCALENDAR\r\nEND:VCALENDAR\r\n",
		"BEGIN:VCALENDAR\r\nX-WR-CALNAME:" + strings.Repeat("n", 100000) + "\r\nEND:VCALENDAR\r\n",
		"BEGIN:VCALENDAR\r\nX-WR-CALNAME:\x00\xff\r\nEND:VCALENDAR\r\n",
		strings.Repeat("BEGIN:VEVENT\r\n", 10000),
		strings.Repeat("BEGIN:VCALENDAR\r\n", 5000) + strings.Repeat("END:VCALENDAR\r\n", 5000),
		strings.Repeat("X-A:b\r\n", 50000),
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		f.Skipf("no time zone data: %v", err)
	}
	from := time.Date(2026, 10, 8, 0, 0, 0, 0, loc)
	w := Window{From: from, To: from.AddDate(0, 0, 8), Location: loc}

	f.Fuzz(func(t *testing.T, data []byte) {
		began := time.Now()
		events, err := Parse(bytes.NewReader(data), w, "fallback")
		if took := time.Since(began); took > maxParseTime {
			t.Fatalf("Parse took %v for %d bytes, want under %v", took, len(data), maxParseTime)
		}
		if err != nil {
			if events != nil {
				t.Fatalf("Parse returned %d events together with error %v", len(events), err)
			}
			return
		}
		checkEvents(t, events, w)
	})
}

// checkEvents asserts what the TV relies on: events sorted by start, inside
// the window, with a one-line title and well-formed times.
func checkEvents(t *testing.T, events []Event, w Window) {
	t.Helper()
	var prev time.Time
	for i, e := range events {
		start, end := parseEventTime(t, e, e.Start, w), parseEventTime(t, e, e.End, w)
		// A timed event without an end is a moment; only all-day events span time.
		if end.Before(start) || e.AllDay && !end.After(start) {
			t.Fatalf("event %d %+v ends before it starts", i, e)
		}
		if !start.Before(w.To) || !end.After(w.From) {
			t.Fatalf("event %d %+v lies outside the window %v to %v", i, e, w.From, w.To)
		}
		if i > 0 && start.Before(prev) {
			t.Fatalf("event %d %+v starts before the previous event (%v)", i, e, prev)
		}
		prev = start
		if e.Title == "" {
			t.Fatalf("event %d has no title", i)
		}
		if strings.ContainsAny(e.Title, "\r\n\t") || len([]rune(e.Title)) > maxSummaryRunes {
			t.Fatalf("event %d title %q is not one short line", i, e.Title)
		}
	}
}

func parseEventTime(t *testing.T, e Event, value string, w Window) time.Time {
	t.Helper()
	layout := time.RFC3339
	if e.AllDay {
		layout = dateLayout
	}
	got, err := time.ParseInLocation(layout, value, w.Location)
	if err != nil {
		t.Fatalf("event %+v has time %q that is not %s: %v", e, value, layout, err)
	}
	return got
}

func FuzzParseSources(f *testing.F) {
	for _, s := range []string{
		"", ",", " , ,", "https://example.org/a.ics", "webcal://example.org/a.ics, https://example.org/b.ics,",
		"file:///etc/passwd", "not a url", "ftp://example.org/x.ics", "http://", "https:///path", "https://user:secret@example.org/x.ics",
		"https://example.org/\x00", "https://exa mple.org/", "%zz", "WEBCAL://EXAMPLE.ORG/A.ICS", "javascript:alert(1)",
		"https://[::1]/x.ics", "https://[::1", "https://example.org/x.ics?token=" + strings.Repeat("s", 10000),
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, list string) {
		sources, err := ParseSources(list)
		if err != nil {
			if sources != nil {
				t.Fatalf("ParseSources(%q) returned sources together with an error", list)
			}
			// The address is a secret and must not come back in errors.
			for _, part := range strings.Split(list, ",") {
				part = strings.TrimSpace(part)
				if len(part) >= 8 && strings.Contains(err.Error(), part) {
					t.Fatalf("error %q repeats the address %q", err, part)
				}
			}
			return
		}
		for _, s := range sources {
			if s.Name == "" || !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") {
				t.Fatalf("source %+v is incomplete or not http(s)", s)
			}
		}
	})
}
