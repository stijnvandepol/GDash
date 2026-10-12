package energyprice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the tests do not rely on the machine having a time zone database
)

func amsterdam(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// serve answers every request with the recorded file and reports the query.
func serve(t *testing.T, file string, query *string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query != nil {
			*query = r.URL.RawQuery
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func client(t *testing.T, srv *httptest.Server, now time.Time) Client {
	t.Helper()
	return Client{BaseURL: srv.URL, HTTP: srv.Client(), Location: amsterdam(t), Now: func() time.Time { return now }}
}

func TestPricesOfToday(t *testing.T) {
	var query string
	srv := serve(t, "today.json", &query)
	now := time.Date(2026, 10, 12, 8, 48, 0, 0, time.UTC)

	report, err := client(t, srv, now).Prices(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// The range is the Dutch today and tomorrow, as EnergyZero wants it.
	for _, want := range []string{
		"fromDate=2026-10-11T22%3A00%3A00.000Z", "tillDate=2026-10-13T21%3A59%3A59.999Z",
		"interval=4", "usageType=1", "inclBtw=true",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q lacks %q", query, want)
		}
	}
	if len(report.Days) != 1 {
		t.Fatalf("%d days, want only today, as tomorrow is not published yet", len(report.Days))
	}
	today := report.Days[0]
	if today.Date != "2026-10-12" || len(today.Hours) != 24 {
		t.Fatalf("today = %s with %d hours, want 2026-10-12 with 24", today.Date, len(today.Hours))
	}
	// Prices arrive in UTC and are written in Dutch time, so a screen can
	// read the hour from the text.
	if first := today.Hours[0]; first.Start.Format(time.RFC3339) != "2026-10-12T00:00:00+02:00" || first.Price != 0.22 {
		t.Errorf("first hour = %v at %v, want 0.22 at 2026-10-12T00:00:00+02:00", first.Price, first.Start)
	}
	if c := today.Cheapest; c == nil || c.Start.Format("15:04") != "12:00" || c.End.Format("15:04") != "15:00" {
		t.Errorf("cheapest = %+v, want 12:00 to 15:00", c)
	}
}

func TestPricesOfTodayAndTomorrow(t *testing.T) {
	srv := serve(t, "two-days.json", nil)
	now := time.Date(2026, 10, 12, 14, 5, 0, 0, time.UTC)

	report, err := client(t, srv, now).Prices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Days) != 2 || report.Days[1].Date != "2026-10-13" || len(report.Days[1].Hours) != 24 {
		t.Fatalf("days = %+v, want today and a full tomorrow", report.Days)
	}
	if c := report.Days[1].Cheapest; c == nil || c.Start.Format("15:04") != "12:00" {
		t.Errorf("tomorrow's cheapest = %+v, want from 12:00", c)
	}
}

func TestPricesFailWhenTodayIsMissing(t *testing.T) {
	srv := serve(t, "today.json", nil)
	// The recorded answer is for 12 October; asked on the 13th it has no
	// prices for today, and the dashboard must keep what it had.
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	if _, err := client(t, srv, now).Prices(context.Background()); err == nil {
		t.Fatal("Prices succeeded without prices for today")
	}
}

func TestPricesFailOnABadAnswer(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusBadGateway) },
		"not json":     func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) },
		"no prices":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"Prices":[]}`)) },
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			defer srv.Close()
			now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
			if _, err := client(t, srv, now).Prices(context.Background()); err == nil {
				t.Fatal("Prices succeeded")
			}
		})
	}
}

func TestParseLeavesOutBrokenHours(t *testing.T) {
	const body = `{"Prices":[
		{"readingDate":"2026-10-12T10:00:00Z","price":0.20},
		{"readingDate":"2026-10-12T10:00:00Z","price":0.20},
		{"readingDate":"2026-10-12T11:00:00Z"},
		{"readingDate":"2026-10-12T12:00:00Z","price":99},
		{"readingDate":"2026-10-12T13:00:00Z","price":-0.01},
		{"readingDate":"2026-10-12T13:15:00Z","price":0.30},
		{"readingDate":"yesterday","price":0.30},
		{"readingDate":"2026-10-12T09:00:00Z","price":0.10}
	]}`
	report, err := parse(strings.NewReader(body), amsterdam(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Days) != 1 {
		t.Fatalf("days = %+v", report.Days)
	}
	var got []string
	for _, h := range report.Days[0].Hours {
		got = append(got, h.Start.UTC().Format("15:04"))
	}
	// Sorted, without the repeat, the missing price, the absurd price,
	// the quarter hour and the unreadable date; a negative price is real.
	if strings.Join(got, " ") != "09:00 10:00 13:00" {
		t.Fatalf("hours = %v, want 09:00 10:00 13:00", got)
	}
}

func TestParseSplitsDaysAtDutchMidnight(t *testing.T) {
	// 22:00 UTC is midnight in Amsterdam in summer, so the hours of 12
	// October belong to two Dutch days.
	const body = `{"Prices":[
		{"readingDate":"2026-10-12T21:00:00Z","price":0.20},
		{"readingDate":"2026-10-12T22:00:00Z","price":0.21}
	]}`
	report, err := parse(strings.NewReader(body), amsterdam(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Days) != 2 || report.Days[0].Date != "2026-10-12" || report.Days[1].Date != "2026-10-13" {
		t.Fatalf("days = %+v, want 12 and 13 October", report.Days)
	}
	if report.Days[0].Cheapest != nil {
		t.Errorf("two hours have no three-hour stretch, got %+v", report.Days[0].Cheapest)
	}
}
