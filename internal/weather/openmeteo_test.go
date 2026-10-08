package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
	if got.TemperatureC != 9.9 || got.WeatherCode != 3 || got.IsDay || got.WindKmh != 14.4 || got.ObservedAt != "2026-10-09T00:00" {
		t.Fatalf("current = %+v", got)
	}
	if got.TodayMaxC != 13.7 || got.TodayMinC != 8.0 {
		t.Fatalf("today = %v / %v, want the first forecast day", got.TodayMinC, got.TodayMaxC)
	}
	if len(got.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(got.Days))
	}
	if want := (Day{Date: "2026-10-13", WeatherCode: 51, MaxC: 20.1, MinC: 14.1, RainChance: 18}); got.Days[4] != want {
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
