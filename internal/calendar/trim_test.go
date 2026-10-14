package calendar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// googleWindow is the week from Wednesday 14 October 2026, the window the
// Calendar widget asks for.
func googleWindow(t *testing.T) Window {
	t.Helper()
	loc := amsterdam(t)
	from := time.Date(2026, 10, 14, 0, 0, 0, 0, loc)
	return Window{From: from, To: from.AddDate(0, 0, 7), Location: loc}
}

func TestParseReadsAGoogleCalendar(t *testing.T) {
	f, err := os.Open("testdata/google.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := Parse(f, googleWindow(t), "fallback")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Start+" "+e.Title)
		if e.Calendar != "Gezin" {
			t.Errorf("event %q is in calendar %q, want the X-WR-CALNAME", e.Title, e.Calendar)
		}
	}
	want := []string{
		// A yearly birthday since 1980.
		"2026-10-16 Verjaardag opa",
		"2026-10-16T08:30:00+02:00 Tandarts",
		// Outlook's Windows name for the zone of Amsterdam and Berlin.
		"2026-10-18T15:00:00+02:00 Koffie met Sanne",
		// A zone nobody knows counts in the server's zone.
		"2026-10-19T09:00:00+02:00 Oudergesprek",
		// The weekly lesson of Monday the 19th, moved to Tuesday.
		"2026-10-20T19:30:00+02:00 Zwemles (verplaatst)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTrimKeepsOnlyWhatParseNeeds(t *testing.T) {
	raw, err := os.ReadFile("testdata/google.ics")
	if err != nil {
		t.Fatal(err)
	}
	kept, err := trim(bytes.NewReader(raw), googleWindow(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(kept)
	for _, gone := range []string{"DESCRIPTION", "ATTENDEE", "VALARM", "VTIMEZONE", "Tandarts 2015", "Ver weg", "LOCATION", "W. Europe", "(UTC+01:00)"} {
		if strings.Contains(text, gone) {
			t.Errorf("trimmed calendar still holds %q", gone)
		}
	}
	for _, kept := range []string{"X-WR-CALNAME:Gezin", "RRULE:FREQ=WEEKLY;BYDAY=MO", "Verjaardag oma", "RECURRENCE-ID;TZID=Europe/Amsterdam:20261019T190000", "DTSTART;TZID=Europe/Berlin:20261018T150000", "DTSTART:20261019T090000"} {
		if !strings.Contains(text, kept) {
			t.Errorf("trimmed calendar lost %q:\n%s", kept, text)
		}
	}
}

func TestParseTellsAWebPageFromACalendar(t *testing.T) {
	pages := []string{
		// What a calendar.google.com/calendar/embed or /r address answers.
		"<!doctype html><html><head><title>Google Calendar</title>",
		"\r\n\r\n<html lang=\"nl\">" + strings.Repeat("x", 5<<20),
		`{"error": "not a calendar"}`,
		"",
	}
	for _, page := range pages {
		_, err := Parse(strings.NewReader(page), googleWindow(t), "x")
		if !errors.Is(err, ErrNotCalendar) {
			t.Errorf("Parse(%.30q) = %v, want ErrNotCalendar", page, err)
		}
	}
	// A byte order mark and blank lines before the calendar are fine.
	if _, err := Parse(strings.NewReader("\ufeff\r\n"+vevent("DTSTART:20261015T080000Z")), googleWindow(t), "x"); err != nil {
		t.Errorf("Parse of a calendar with a byte order mark: %v", err)
	}
}

// history writes a calendar of single events, one a day from 1 January
// 2000 and each with a long description, as a calendar with years of
// history is, followed by one event in the window and a weekly one.
type history struct {
	day     time.Time
	last    time.Time
	pending bytes.Buffer
	ended   bool
}

func newHistory(last time.Time) *history {
	h := &history{day: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC), last: last}
	h.pending.WriteString("BEGIN:VCALENDAR\r\nPRODID:-//Google Inc//Google Calendar 70.9054//EN\r\nVERSION:2.0\r\nX-WR-CALNAME:Werk\r\n")
	return h
}

func (h *history) Read(p []byte) (int, error) {
	for h.pending.Len() < len(p) && !h.ended {
		if h.day.Before(h.last) {
			fmt.Fprintf(&h.pending, "BEGIN:VEVENT\r\nDTSTART:%s\r\nDTEND:%s\r\nUID:%d@google.com\r\nSUMMARY:Overleg\r\nDESCRIPTION:%s\r\n %s\r\nEND:VEVENT\r\n",
				h.day.Format("20060102T150405Z"), h.day.Add(time.Hour).Format("20060102T150405Z"), h.day.Unix(), strings.Repeat("Agenda ", 10), strings.Repeat("notes ", 1000))
			h.day = h.day.AddDate(0, 0, 1)
			continue
		}
		h.pending.WriteString("BEGIN:VEVENT\r\nDTSTART:20261015T080000Z\r\nUID:now@google.com\r\nSUMMARY:Vandaag\r\nEND:VEVENT\r\n")
		h.pending.WriteString("BEGIN:VEVENT\r\nDTSTART:20000103T150000Z\r\nRRULE:FREQ=WEEKLY\r\nUID:weekly@google.com\r\nSUMMARY:Wekelijks\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
		h.ended = true
	}
	if h.pending.Len() == 0 {
		return 0, io.EOF
	}
	return h.pending.Read(p)
}

func TestParseReadsACalendarWithYearsOfHistory(t *testing.T) {
	// Ten years of daily events with descriptions: about 22 MB, twice the
	// limit GDash had when it read whole feeds into memory.
	feed := &countingReader{r: newHistory(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC))}
	events, err := Parse(feed, googleWindow(t), "x")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if feed.n < 20<<20 {
		t.Fatalf("the feed is %d bytes; the test needs one far above 10 MB", feed.n)
	}
	var titles []string
	for _, e := range events {
		titles = append(titles, e.Title)
	}
	if strings.Join(titles, ",") != "Vandaag,Wekelijks" {
		t.Fatalf("events = %v", titles)
	}
}

func TestParseRefusesAFeedAboveTheLimit(t *testing.T) {
	// Daily events until 2040: more than maxCalendarBytes, read as it
	// arrives and never held in memory whole.
	_, err := Parse(newHistory(time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)), googleWindow(t), "x")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Parse = %v, want ErrTooLarge", err)
	}
}

func TestTrimRefusesTooMuchToKeep(t *testing.T) {
	// Recurring events are always kept, so a feed of only those is bounded
	// by what trim keeps.
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	for i := 0; b.Len() < maxKeptBytes+1<<20; i++ {
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:%d\r\nDTSTART:20000101T000000Z\r\nRRULE:FREQ=YEARLY\r\nSUMMARY:%s\r\nEND:VEVENT\r\n", i, strings.Repeat("y", 200))
	}
	b.WriteString("END:VCALENDAR\r\n")
	if _, err := trim(strings.NewReader(b.String()), googleWindow(t)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("trim = %v, want ErrTooLarge", err)
	}
}

func TestTrimRejectsMismatchedComponents(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nDTSTART:20261015T080000Z\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	if _, err := Parse(strings.NewReader(ics), googleWindow(t), "x"); err == nil {
		t.Fatal("Parse accepted an END that closes another component")
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}
