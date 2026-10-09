package calendar

import (
	"os"
	"strings"
	"testing"
	"time"
)

func amsterdam(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	return loc
}

// window is Thursday 8 October 2026 from midnight up to Friday 16 October,
// so it holds two Thursdays: one lesson excluded, one regular.
func window(t *testing.T) Window {
	loc := amsterdam(t)
	from := time.Date(2026, 10, 8, 0, 0, 0, 0, loc)
	return Window{From: from, To: from.AddDate(0, 0, 8), Location: loc}
}

func parseFamily(t *testing.T) []Event {
	t.Helper()
	f, err := os.Open("testdata/family.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := Parse(f, window(t), "fallback")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return events
}

func TestParseExpandsAndOrdersTheWeek(t *testing.T) {
	var got []string
	for _, e := range parseFamily(t) {
		got = append(got, e.Start+" "+e.Title)
	}
	want := []string{
		// Started on the 5th, still running on the 8th; end date exclusive.
		"2026-10-05 Autumn holiday",
		// The Thursday 8 October lesson is excluded by EXDATE.
		"2026-10-09T10:00:00+02:00 Dentist",
		"2026-10-10 Birthday Britt",
		"2026-10-11T10:00:00+02:00 Floating time",
		// The Tuesday lesson is moved an hour earlier by RECURRENCE-ID.
		"2026-10-13T18:00:00+02:00 Swimming lesson (moved)",
		"2026-10-15T19:00:00+02:00 Swimming lesson",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParseFillsEventFields(t *testing.T) {
	events := parseFamily(t)
	holiday, dentist := events[0], events[1]
	if !holiday.AllDay || holiday.End != "2026-10-10" || holiday.Calendar != "Family" {
		t.Errorf("all-day event = %+v", holiday)
	}
	if dentist.AllDay || dentist.End != "2026-10-09T11:00:00+02:00" {
		t.Errorf("timed event = %+v", dentist)
	}
}

func TestParseCutsEndlessEventsAtYear9999(t *testing.T) {
	events, err := Parse(strings.NewReader(vevent("DTSTART:20261009T080000Z", "DTEND:99991231T235959Z", "SUMMARY:Forever")), window(t), "Work")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	// The end was written with five digits (year 10000) in the server's zone.
	if _, err := time.Parse(time.RFC3339, events[0].End); err != nil {
		t.Errorf("End %q is not RFC 3339: %v", events[0].End, err)
	}
}

func TestParseUsesFallbackNameAndRejectsGarbage(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:x\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTAMP:20261001T120000Z\r\nDTSTART:20261009T080000Z\r\nSUMMARY:   Many    spaces  \r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	events, err := Parse(strings.NewReader(ics), window(t), "Work")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Calendar != "Work" || events[0].Title != "Many spaces" {
		t.Fatalf("events = %+v", events)
	}
	if _, err := Parse(strings.NewReader("<html>login page</html>"), window(t), "Work"); err == nil {
		t.Fatal("Parse accepted an HTML page")
	}
}
