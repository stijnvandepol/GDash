package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCalendarTest(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	// The test environment's clock is 8 October 2026; see newTestEnv.
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Google Inc//Google Calendar 70.9054//EN\r\nX-WR-CALNAME:Gezin\r\n" +
		"BEGIN:VEVENT\r\nUID:a\r\nDTSTART:20261009T080000Z\r\nDTEND:20261009T090000Z\r\nSUMMARY:Tandarts\r\nDESCRIPTION:secret notes\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:b\r\nDTSTART;VALUE=DATE:20261001\r\nRRULE:FREQ=DAILY\r\nSUMMARY:Elke dag\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/calendar/ical/x%40gmail.com/private-1/basic.ics", "/calendar/ical/x@gmail.com/private-1/basic.ics":
			w.Header().Set("Content-Type", "text/calendar")
			_, _ = w.Write([]byte(ics))
		case "/calendar/embed":
			_, _ = w.Write([]byte("<!doctype html><html><title>Google Calendar</title>"))
		case "/locked.ics":
			w.WriteHeader(http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	status, body := env.do(t, "POST", "/api/calendar-tests", `{"url":"`+srv.URL+`/calendar/ical/x%40gmail.com/private-1/basic.ics"}`)
	if status != http.StatusOK {
		t.Fatalf("test = %d %s", status, body)
	}
	var got struct {
		Name   string
		Count  int
		Events []struct{ Title, Start string }
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Gezin" || got.Count != 31 || len(got.Events) != 5 || got.Events[0].Title != "Elke dag" || got.Events[2].Title != "Tandarts" {
		t.Fatalf("test = %s; want Gezin with 31 events, the first five of them", body)
	}
	if strings.Contains(body, "secret notes") {
		t.Fatalf("test = %s; it sends more of an event than the TV shows", body)
	}
	if status, body := env.do(t, "GET", "/api/settings", ""); status != http.StatusOK || !strings.Contains(body, `"calendars":[]`) {
		t.Fatalf("settings after a test = %d %s; want nothing stored", status, body)
	}

	for path, code := range map[string]string{
		"/public/basic.ics": codeCalendarNotFound,
		"/locked.ics":       codeCalendarRefused,
		"/calendar/embed":   codeNotACalendar,
	} {
		status, body := env.do(t, "POST", "/api/calendar-tests", `{"url":"`+srv.URL+path+`"}`)
		if status != http.StatusBadGateway || !strings.Contains(body, `"code":"`+code+`"`) {
			t.Errorf("test of %s = %d %s, want 502 %s", path, status, body, code)
		}
		if strings.Contains(body, srv.URL) || strings.Contains(body, "127.0.0.1") {
			t.Errorf("test of %s = %s; the message repeats the address, which can be a secret", path, body)
		}
	}
	if status, body := env.do(t, "POST", "/api/calendar-tests", `{"url":"calendar.google.com"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("test of no address = %d %s, want 422 naming url", status, body)
	}
}

func TestCalendarTestNeedsLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "POST", "/api/calendar-tests", `{"url":"https://calendar.example.org/a.ics"}`); status != http.StatusUnauthorized {
		t.Fatalf("test without login = %d, want 401", status)
	}
}
