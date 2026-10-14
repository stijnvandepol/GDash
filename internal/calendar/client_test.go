package calendar

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

func TestParseSources(t *testing.T) {
	sources, err := ParseSources(" webcal://example.org/a.ics , https://example.org/b.ics,")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].URL != "https://example.org/a.ics" || sources[1].Name != "Calendar 2" {
		t.Fatalf("sources = %+v", sources)
	}
	for _, bad := range []string{"file:///etc/passwd", "not a url", "ftp://example.org/x.ics"} {
		_, err := ParseSources(bad)
		if err == nil {
			t.Errorf("ParseSources(%q) accepted it", bad)
		} else if strings.Contains(err.Error(), bad) {
			t.Errorf("error %q repeats the URL, which can be a secret", err)
		}
	}
}

func TestUpcomingMergesCalendarsFromStartOfToday(t *testing.T) {
	family, err := os.ReadFile("testdata/family.ics")
	if err != nil {
		t.Fatal(err)
	}
	work := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:x\r\nX-WR-CALNAME:Work\r\nBEGIN:VEVENT\r\nUID:w\r\nDTSTAMP:20261001T120000Z\r\nDTSTART:20261008T060000Z\r\nDTEND:20261008T070000Z\r\nSUMMARY:Stand-up\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/work.ics" {
			w.Write([]byte(work))
			return
		}
		w.Write(family)
	}))
	defer srv.Close()

	loc := amsterdam(t)
	c := &Client{
		HTTP:     &http.Client{Timeout: 5 * time.Second},
		Location: loc,
		// Late in the day: "today" still starts at midnight, so the
		// morning stand-up that already ended stays listed; the TV shows
		// the whole day and greys out what is past.
		Now: func() time.Time { return time.Date(2026, 10, 8, 21, 0, 0, 0, loc) },
	}
	events, err := c.Upcoming(context.Background(), []Source{{Name: "a", URL: srv.URL + "/family.ics", Color: "blue"}, {Name: "b", URL: srv.URL + "/work.ics", Color: "red"}}, 2)
	if err != nil {
		t.Fatalf("Upcoming: %v", err)
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Calendar+": "+e.Title)
	}
	for _, e := range events {
		if (e.Calendar == "Family") != (e.Color == "blue") {
			t.Fatalf("event %q of %s has colour %q; want each calendar's own", e.Title, e.Calendar, e.Color)
		}
	}
	want := "Family: Autumn holiday|Work: Stand-up|Family: Dentist"
	if strings.Join(got, "|") != want {
		t.Fatalf("events = %s, want %s", strings.Join(got, "|"), want)
	}
}

func TestUpcomingErrorsDoNotLeakTheURL(t *testing.T) {
	c := &Client{HTTP: &http.Client{Timeout: time.Second}, Location: time.UTC, Now: time.Now}
	secret := "http://127.0.0.1:1/calendar/private-0123456789abcdef/basic.ics"
	_, err := c.Upcoming(context.Background(), []Source{{Name: "x", URL: secret}}, 1)
	if err == nil {
		t.Fatal("Upcoming succeeded against a closed port")
	}
	if strings.Contains(err.Error(), "private-0123456789abcdef") {
		t.Fatalf("error leaks the secret calendar URL: %v", err)
	}
}

func TestUpcomingRejectsServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}, Location: time.UTC, Now: time.Now}
	if _, err := c.Upcoming(context.Background(), []Source{{URL: srv.URL}}, 1); err == nil {
		t.Fatal("Upcoming accepted a 410")
	}
}

func TestUpcomingFetchesCalendarsInParallel(t *testing.T) {
	const delay = 300 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.Write([]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:x\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}, Location: time.UTC, Now: time.Now}
	var sources []Source
	for i := range parallelFetches {
		sources = append(sources, Source{Name: "c", URL: srv.URL + "/" + string(rune('a'+i))})
	}
	// One after another they would take four times the delay.
	ctx, cancel := context.WithTimeout(context.Background(), 2*delay)
	defer cancel()
	if _, err := c.Upcoming(ctx, sources, 7); err != nil {
		t.Fatalf("Upcoming of %d slow calendars within twice the delay of one: %v", len(sources), err)
	}
}

// TestReadFollowsGoogle reads a calendar the way Google serves one: behind
// a redirect to another host, compressed, at an address with %40 and %23
// in its path, and with years of history.
func TestReadFollowsGoogle(t *testing.T) {
	const path = "/calendar/ical/nl.dutch%23holiday%40group.v.calendar.google.com/private-0123/basic.ics"
	served := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != path {
			t.Errorf("path = %q, want %q", r.URL.EscapedPath(), path)
		}
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Errorf("Accept-Encoding = %q; Google's feeds are far smaller compressed", r.Header.Get("Accept-Encoding"))
		}
		w.Header().Set("Content-Type", "text/calendar; charset=UTF-8")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		if _, err := io.Copy(zw, newHistory(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC))); err != nil {
			t.Error(err)
		}
		zw.Close()
	}))
	defer served.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, served.URL+r.URL.EscapedPath(), http.StatusFound)
	}))
	defer front.Close()

	loc := amsterdam(t)
	c := &Client{HTTP: outbound.NewClient(20 * time.Second), Location: loc, Now: func() time.Time { return time.Date(2026, 10, 14, 9, 0, 0, 0, loc) }}
	events, err := c.Read(context.Background(), Source{Name: "Werk", URL: front.URL + path}, 7)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(events) != 2 || events[0].Title != "Vandaag" {
		t.Fatalf("events = %+v", events)
	}
}

func TestReadTellsWhatWentWrong(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/public/basic.ics" {
			// Google's answer for the public address of a private calendar.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("<html><title>Error 404 (Not Found)!!1</title>"))
			return
		}
		// An embed or web address: a page, with 200.
		w.Write([]byte("<!doctype html><html><head><title>Google Calendar</title>"))
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}, Location: time.UTC, Now: time.Now}

	_, err := c.Read(context.Background(), Source{URL: srv.URL + "/public/basic.ics"}, 7)
	var status *StatusError
	if !errors.As(err, &status) || status.Code != http.StatusNotFound {
		t.Errorf("Read of a missing calendar = %v, want a StatusError 404", err)
	}
	if _, err := c.Read(context.Background(), Source{URL: srv.URL + "/calendar/embed"}, 7); !errors.Is(err, ErrNotCalendar) {
		t.Errorf("Read of a web page = %v, want ErrNotCalendar", err)
	}
}
