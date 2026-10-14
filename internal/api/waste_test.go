package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWasteCalendarsAreKeptApartFromTheCalendars(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/calendars", `{"name":"Family","url":"https://calendar.example.org/a.ics"}`); status != http.StatusCreated {
		t.Fatalf("add calendar = %d %s", status, body)
	}
	if status, body := env.do(t, "POST", "/api/calendars", `{"name":"Home","url":"webcal://afval.example.org/1234AB","purpose":"waste"}`); status != http.StatusCreated {
		t.Fatalf("add waste calendar = %d %s", status, body)
	}
	if status, body := env.do(t, "POST", "/api/calendars", `{"name":"Bins","url":"https://afval.example.org/b","purpose":"bins"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"purpose"`) {
		t.Fatalf("add with an unknown purpose = %d %s, want 422 naming purpose", status, body)
	}
	_, body := env.do(t, "GET", "/api/settings", "")
	var got struct {
		Settings struct {
			Calendars      []struct{ ID int64 }
			WasteCalendars []struct {
				ID   int64
				Name string
				Host string
			}
		}
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	// The Calendar widget and the Calendars card read calendars, the Waste
	// collection widget and its card wasteCalendars.
	cals, bins := got.Settings.Calendars, got.Settings.WasteCalendars
	if len(cals) != 1 || cals[0].ID != 1 || len(bins) != 1 || bins[0].ID != 2 || bins[0].Name != "Home" || bins[0].Host != "afval.example.org" {
		t.Fatalf("settings = %s; want Family among the calendars and Home among the waste calendars", body)
	}
	if status, _ := env.do(t, "DELETE", "/api/calendars/2", ""); status != http.StatusNoContent {
		t.Fatalf("delete the waste calendar = %d, want 204", status)
	}
}

func TestWasteCalendarTest(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	ics, err := os.ReadFile("../waste/testdata/afvalkalender.ics")
	if err != nil {
		t.Fatal(err)
	}
	calendarServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/afval.ics" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(ics)
	}))
	t.Cleanup(calendarServer.Close)

	status, body := env.do(t, "POST", "/api/waste-calendar-tests", `{"url":"`+calendarServer.URL+`/afval.ics"}`)
	if status != http.StatusOK {
		t.Fatalf("test = %d %s", status, body)
	}
	var got struct {
		Pickups []struct{ Kind, Title, Date string }
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pickups) != 6 || got.Pickups[0].Kind != "gft" || got.Pickups[0].Date != "2026-10-13" || got.Pickups[4].Kind != "other" || got.Pickups[4].Title != "Grofvuil op afspraak" {
		t.Fatalf("pickups = %s; want the six pickups of the recorded calendar, from GFT on 13 October", body)
	}
	if status, body := env.do(t, "GET", "/api/settings", ""); status != http.StatusOK || !strings.Contains(body, `"wasteCalendars":[]`) {
		t.Fatalf("settings after a test = %d %s; want nothing stored", status, body)
	}

	if status, body := env.do(t, "POST", "/api/waste-calendar-tests", `{"url":"`+calendarServer.URL+`/gone.ics"}`); status != http.StatusBadGateway || !strings.Contains(body, "404") {
		t.Fatalf("test of a missing calendar = %d %s, want 502 saying what the server answered", status, body)
	}
	if status, body := env.do(t, "POST", "/api/waste-calendar-tests", `{"url":"mijnafvalwijzer"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("test of no address = %d %s, want 422 naming url", status, body)
	}
}

func TestWasteCalendarTestNeedsLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "POST", "/api/waste-calendar-tests", `{"url":"https://afval.example.org/a.ics"}`); status != http.StatusUnauthorized {
		t.Fatalf("test without login = %d, want 401", status)
	}
}
