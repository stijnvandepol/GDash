package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, status int, body []byte) (Client, *http.Request) {
	t.Helper()
	var got http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}, &got
}

func TestCurrentParsesRecordedResponse(t *testing.T) {
	body, err := os.ReadFile("testdata/forecast.json")
	if err != nil {
		t.Fatal(err)
	}
	client, req := serve(t, http.StatusOK, body)
	loc, _ := NewLocation(51.646, 5.945)

	got, err := client.Current(context.Background(), loc)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.TemperatureC != 10.2 || got.WeatherCode != 3 || !got.IsDay || got.WindKmh != 28.8 || got.ObservedAt != "2026-10-09T13:15" {
		t.Fatalf("current = %+v", got)
	}
	if got.TodayMaxC != 14.0 || got.TodayMinC != 8.3 {
		t.Fatalf("today = %v / %v, want the first forecast day", got.TodayMinC, got.TodayMaxC)
	}
	if len(got.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(got.Days))
	}
	if want := (Day{Date: "2026-10-13", WeatherCode: 51, MaxC: 19.5, MinC: 11.0, RainChance: 25}); got.Days[4] != want {
		t.Fatalf("day 5 = %+v, want %+v", got.Days[4], want)
	}
	if lat := req.URL.Query().Get("latitude"); lat != "51.65" {
		t.Fatalf("latitude sent = %q, want rounded 51.65", lat)
	}
}

func TestCurrentRejectsErrorStatus(t *testing.T) {
	client, _ := serve(t, http.StatusTooManyRequests, []byte(`{"error":true}`))
	if _, err := client.Current(context.Background(), Location{}); err == nil {
		t.Fatal("Current accepted a 429 response")
	}
}

func TestCurrentRejectsIncompleteResponse(t *testing.T) {
	client, _ := serve(t, http.StatusOK, []byte(`{"current":{"time":"2026-10-08T13:30"},"daily":{}}`))
	if _, err := client.Current(context.Background(), Location{}); err == nil {
		t.Fatal("Current accepted a response without temperature")
	}
}

func TestCurrentKeepsZeroDegrees(t *testing.T) {
	body := []byte(`{"current":{"time":"t","temperature_2m":0,"weather_code":0,"is_day":0,"wind_speed_10m":0},"daily":{"time":["2026-01-10"],"weather_code":[71],"temperature_2m_max":[1],"temperature_2m_min":[-2],"precipitation_probability_max":[0]}}`)
	client, _ := serve(t, http.StatusOK, body)
	got, err := client.Current(context.Background(), Location{})
	if err != nil {
		t.Fatalf("Current rejected a valid 0 °C report: %v", err)
	}
	if got.TemperatureC != 0 || got.TodayMinC != -2 || got.IsDay {
		t.Fatalf("Current = %+v", got)
	}
}

func TestDaysStopAtTheShortestArray(t *testing.T) {
	// A missing value must not move the next day's numbers onto this day.
	body := []byte(`{"current":{"time":"t","temperature_2m":5,"weather_code":3,"is_day":1,"wind_speed_10m":4},
		"daily":{"time":["2026-10-09","2026-10-10","2026-10-11"],"weather_code":[61,63],"temperature_2m_max":[13,14,15],
		"temperature_2m_min":[8,9,10],"precipitation_probability_max":[100,92,53]}}`)
	client, _ := serve(t, http.StatusOK, body)
	got, err := client.Current(context.Background(), Location{})
	if err != nil || len(got.Days) != 2 || got.Days[1].WeatherCode != 63 {
		t.Fatalf("Days = %+v, %v; want the two complete days", got.Days, err)
	}
}

func TestCurrentKeepsYesterdayOutOfTheForecastButInTheSun(t *testing.T) {
	body, err := os.ReadFile("testdata/forecast.json")
	if err != nil {
		t.Fatal(err)
	}
	client, req := serve(t, http.StatusOK, body)

	got, err := client.Current(context.Background(), Location{})
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.Days[0].Date != "2026-10-09" {
		t.Fatalf("first forecast day = %s, want today", got.Days[0].Date)
	}
	if len(got.Sun) != 8 || got.Sun[0] != (SunTimes{Date: "2026-10-08", Sunrise: "2026-10-08T07:48", Sunset: "2026-10-08T18:58"}) {
		t.Fatalf("sun = %+v, want yesterday first and eight days", got.Sun)
	}
	if got.Sun[1].Sunrise != "2026-10-09T07:50" || got.Sun[1].Sunset != "2026-10-09T18:56" {
		t.Fatalf("today's sun = %+v", got.Sun[1])
	}
	if got.UTCOffsetSeconds == nil || *got.UTCOffsetSeconds != 7200 {
		t.Fatalf("UTCOffsetSeconds = %v, want 7200", got.UTCOffsetSeconds)
	}
	query := req.URL.Query()
	if query.Get("past_days") != "1" || !strings.Contains(query.Get("daily"), "sunrise,sunset") {
		t.Fatalf("query = %v, want past_days=1 and sunrise,sunset", query)
	}
}

func TestSunDegradesWithoutBreakingTheWeather(t *testing.T) {
	tests := []struct {
		name    string
		today   string
		daily   string
		wantSun []SunTimes
	}{
		{"no sun arrays", "2026-10-09", `"time":["2026-10-09"]`, nil},
		{"sunset missing", "2026-10-09", `"time":["2026-10-09"],"sunrise":["2026-10-09T07:50"]`, nil},
		{"no yesterday", "2026-10-09", `"time":["2026-10-09"],"sunrise":["2026-10-09T07:50"],"sunset":["2026-10-09T18:56"]`,
			[]SunTimes{{Date: "2026-10-09", Sunrise: "2026-10-09T07:50", Sunset: "2026-10-09T18:56"}}},
		{"polar night", "2026-12-20", `"time":["2026-12-20"],"sunrise":[null],"sunset":[null]`,
			[]SunTimes{{Date: "2026-12-20"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"current":{"time":"` + tt.today + `T12:00","temperature_2m":5,"weather_code":3,"is_day":1,"wind_speed_10m":4},
				"daily":{` + tt.daily + `,"weather_code":[61],"temperature_2m_max":[13],"temperature_2m_min":[8],"precipitation_probability_max":[100]}}`)
			client, _ := serve(t, http.StatusOK, body)
			got, err := client.Current(context.Background(), Location{})
			if err != nil || len(got.Days) != 1 {
				t.Fatalf("Current = %+v, %v; want the weather to survive", got, err)
			}
			if !slices.Equal(got.Sun, tt.wantSun) {
				t.Fatalf("sun = %+v, want %+v", got.Sun, tt.wantSun)
			}
		})
	}
}

func TestNewLocation(t *testing.T) {
	tests := []struct {
		lat, lon float64
		wantErr  bool
	}{
		{51.6461, 5.9449, false},
		{-90, 180, false},
		{91, 0, true},
		{0, -181, true},
	}
	for _, tt := range tests {
		_, err := NewLocation(tt.lat, tt.lon)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewLocation(%v, %v) error = %v, wantErr %v", tt.lat, tt.lon, err, tt.wantErr)
		}
	}
}
