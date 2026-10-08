package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
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
	events, err := c.Upcoming(context.Background(), []Source{{Name: "a", URL: srv.URL + "/family.ics"}, {Name: "b", URL: srv.URL + "/work.ics"}}, 2)
	if err != nil {
		t.Fatalf("Upcoming: %v", err)
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Calendar+": "+e.Title)
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
