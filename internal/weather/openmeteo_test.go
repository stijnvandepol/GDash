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
	want := Report{TemperatureC: 14.3, WeatherCode: 3, IsDay: true, WindKmh: 20.9, TodayMaxC: 15.6, TodayMinC: 10.2, ObservedAt: "2026-10-08T13:30"}
	if got != want {
		t.Fatalf("Current = %+v, want %+v", got, want)
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
	body := []byte(`{"current":{"time":"t","temperature_2m":0,"weather_code":0,"is_day":0,"wind_speed_10m":0},"daily":{"temperature_2m_max":[1],"temperature_2m_min":[-2]}}`)
	client, _ := serve(t, http.StatusOK, body)
	got, err := client.Current(context.Background(), Location{})
	if err != nil {
		t.Fatalf("Current rejected a valid 0 °C report: %v", err)
	}
	if got.TemperatureC != 0 || got.TodayMinC != -2 || got.IsDay {
		t.Fatalf("Current = %+v", got)
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
