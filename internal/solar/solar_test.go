package solar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// oss is the place the recorded answers in testdata are for.
var oss = weather.Location{Latitude: 51.77, Longitude: 5.52}

// serve answers every request with the recorded file and keeps the
// requested addresses.
func serve(t *testing.T, file string, requests *[]string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEnergyFollowsIrradianceOnThePanels(t *testing.T) {
	for _, tc := range []struct {
		name            string
		gti, kwp, ratio float64
		want            float64
	}{
		// Peak power is rated at 1000 W/m²: a full hour of it gives kWp.
		{"rated sun", 1000, 4, 1, 4},
		{"half the sun with losses", 500, 4, 0.85, 1.7},
		{"a cloudy hour", 120, 6.5, 0.85, 0.663},
		{"night", 0, 4, 0.85, 0},
		{"a negative value is no production", -3, 4, 0.85, 0},
	} {
		if got := EnergyKWh(tc.gti, tc.kwp, tc.ratio); round(got, 1e6) != tc.want {
			t.Errorf("%s: EnergyKWh(%v, %v, %v) = %v, want %v", tc.name, tc.gti, tc.kwp, tc.ratio, got, tc.want)
		}
	}
}

func TestOpenMeteoEstimate(t *testing.T) {
	var requests []string
	srv := serve(t, "open-meteo.json", &requests)
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 35, Azimuth: -20, PeakKW: 5}}, PerformanceRatio: 0.85, Source: SourceOpenMeteo}

	report, err := OpenMeteo{BaseURL: srv.URL, HTTP: srv.Client()}.Forecast(context.Background(), setup)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("%d requests, want 1 for one array", len(requests))
	}
	for _, want := range []string{
		"latitude=51.77", "longitude=5.52", "hourly=global_tilted_irradiance", "tilt=35", "azimuth=-20",
		"timezone=auto", "forecast_days=2",
	} {
		if !strings.Contains(requests[0], want) {
			t.Errorf("request %q lacks %q", requests[0], want)
		}
	}
	if report.Source != SourceOpenMeteo || report.UTCOffsetSeconds != 7200 || len(report.Days) != 2 {
		t.Fatalf("report = %s, offset %d, %d days; want open-meteo, 7200 and today and tomorrow", report.Source, report.UTCOffsetSeconds, len(report.Days))
	}
	// The recorded GTI sums to 611.8 W/m² hours today and 3932.7
	// tomorrow; 5 kWp at 0.85 turns each W/m² hour into 4.25 Wh.
	today, tomorrow := report.Days[0], report.Days[1]
	if today.Date != "2026-10-13" || today.TotalKWh != 2.6 {
		t.Errorf("today = %s with %v kWh, want 2026-10-13 with 2.6", today.Date, today.TotalKWh)
	}
	if tomorrow.Date != "2026-10-14" || tomorrow.TotalKWh != 16.71 {
		t.Errorf("tomorrow = %s with %v kWh, want 2026-10-14 with 16.71", tomorrow.Date, tomorrow.TotalKWh)
	}
	// The value at 10:00 is the mean of 09:00 to 10:00: the first hour
	// with sun starts at 08:00 (3.7 W/m² at 09:00), in local time.
	if first := today.Hours[0]; first.Start.Format(time.RFC3339) != "2026-10-13T08:00:00+02:00" || first.KWh != 0.016 {
		t.Errorf("first hour today = %v kWh at %v, want 0.016 at 08:00+02:00", first.KWh, first.Start.Format(time.RFC3339))
	}
	peak := tomorrow.Hours[0]
	for _, h := range tomorrow.Hours {
		if h.KWh > peak.KWh {
			peak = h
		}
	}
	if peak.Start.Format("15:04") != "12:00" || peak.KWh != 2.763 {
		t.Errorf("peak tomorrow = %v kWh at %s, want 2.763 at 12:00", peak.KWh, peak.Start.Format("15:04"))
	}
	if last := tomorrow.Hours[len(tomorrow.Hours)-1]; last.Start.Format("15:04") != "18:00" {
		t.Errorf("last hour tomorrow starts at %s, want 18:00: the hours without sun are left out", last.Start.Format("15:04"))
	}
}

func TestOpenMeteoAddsUpTwoArrays(t *testing.T) {
	// Each array gets its own request; the east one sees 400 W/m² in the
	// hour from 09:00, the west one 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gti := "200"
		if r.URL.Query().Get("azimuth") == "-90" {
			gti = "400"
		}
		_, _ = w.Write([]byte(`{"utc_offset_seconds":3600,"hourly":{"time":["2026-01-05T09:00","2026-01-05T10:00"],"global_tilted_irradiance":[0,` + gti + `]}}`))
	}))
	t.Cleanup(srv.Close)
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 30, Azimuth: -90, PeakKW: 3}, {Tilt: 30, Azimuth: 90, PeakKW: 2}}, PerformanceRatio: 0.8, Source: SourceOpenMeteo}

	report, err := OpenMeteo{BaseURL: srv.URL, HTTP: srv.Client()}.Forecast(context.Background(), setup)
	if err != nil {
		t.Fatal(err)
	}
	// 0.4 × 3 × 0.8 + 0.2 × 2 × 0.8 = 0.96 + 0.32.
	if len(report.Days) != 1 || len(report.Days[0].Hours) != 1 || report.Days[0].Hours[0].KWh != 1.28 || report.Days[0].TotalKWh != 1.28 {
		t.Fatalf("days = %+v, want one hour from 09:00 with 1.28 kWh", report.Days)
	}
	if got := report.Days[0].Hours[0].Start.Format(time.RFC3339); got != "2026-01-05T09:00:00+01:00" {
		t.Errorf("hour starts at %s, want 09:00+01:00", got)
	}
}

func TestOpenMeteoFailures(t *testing.T) {
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 35, PeakKW: 5}}, PerformanceRatio: 0.85, Source: SourceOpenMeteo}
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"error status", http.StatusBadRequest, `{"error":true,"reason":"Latitude must be in range"}`},
		{"no hours", http.StatusOK, `{"utc_offset_seconds":0,"hourly":{"time":[],"global_tilted_irradiance":[]}}`},
		{"no offset", http.StatusOK, `{"hourly":{"time":["2026-01-05T09:00"],"global_tilted_irradiance":[1]}}`},
		{"broken time", http.StatusOK, `{"utc_offset_seconds":0,"hourly":{"time":["noon"],"global_tilted_irradiance":[1]}}`},
		{"not JSON", http.StatusOK, `<html>`},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		if _, err := (OpenMeteo{BaseURL: srv.URL, HTTP: srv.Client()}).Forecast(context.Background(), setup); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
		srv.Close()
	}
}

func TestForecastSolarEstimate(t *testing.T) {
	var requests []string
	srv := serve(t, "forecast-solar.json", &requests)
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 35, Azimuth: 0, PeakKW: 5}, {Tilt: 20, Azimuth: 90, PeakKW: 1.5}}, PerformanceRatio: 0.85, Source: SourceForecastSolar}

	report, err := ForecastSolar{BaseURL: srv.URL + "/estimate", HTTP: srv.Client()}.Forecast(context.Background(), setup)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"/estimate/51.77/5.52/35/0/5?time=iso8601", "/estimate/51.77/5.52/20/90/1.5?time=iso8601"}
	if strings.Join(requests, " ") != strings.Join(want, " ") {
		t.Errorf("requests = %v, want one per array: %v", requests, want)
	}
	if report.Source != SourceForecastSolar || report.UTCOffsetSeconds != 7200 || len(report.Days) != 2 {
		t.Fatalf("report = %s, offset %d, %d days", report.Source, report.UTCOffsetSeconds, len(report.Days))
	}
	// Both arrays get the recorded answer here, so each day is twice its
	// watt_hours_day (6165 and 6115 Wh).
	if today := report.Days[0]; today.Date != "2026-10-13" || today.TotalKWh != 12.33 {
		t.Errorf("today = %s with %v kWh, want 2026-10-13 with 12.33", today.Date, today.TotalKWh)
	}
	if tomorrow := report.Days[1]; tomorrow.Date != "2026-10-14" || tomorrow.TotalKWh != 12.23 {
		t.Errorf("tomorrow = %s with %v kWh, want 2026-10-14 with 12.23", tomorrow.Date, tomorrow.TotalKWh)
	}
	hours := report.Days[0].Hours
	// The period that ends at 08:00 (1 Wh since sunrise at 07:58) is the
	// hour from 07:00; the one that ends at sunset, 18:49, the hour from 18:00.
	if first := hours[0]; first.Start.Format(time.RFC3339) != "2026-10-13T07:00:00+02:00" || first.KWh != 0.002 {
		t.Errorf("first hour = %v kWh at %s, want 0.002 at 07:00+02:00", first.KWh, first.Start.Format(time.RFC3339))
	}
	if last := hours[len(hours)-1]; last.Start.Format("15:04") != "18:00" || last.KWh != 0.132 {
		t.Errorf("last hour = %v kWh at %s, want 0.132 at 18:00", last.KWh, last.Start.Format("15:04"))
	}
}

func TestForecastSolarRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 35, PeakKW: 5}}, Source: SourceForecastSolar}
	_, err := ForecastSolar{BaseURL: srv.URL, HTTP: srv.Client()}.Forecast(context.Background(), setup)
	if err == nil || !strings.Contains(err.Error(), "12 requests") {
		t.Errorf("err = %v, want one that names the limit", err)
	}
}

func TestClientUsesTheChosenSource(t *testing.T) {
	var openMeteo, forecastSolar []string
	om := serve(t, "open-meteo.json", &openMeteo)
	fs := serve(t, "forecast-solar.json", &forecastSolar)
	c := Client{OpenMeteo: OpenMeteo{BaseURL: om.URL, HTTP: om.Client()}, ForecastSolar: ForecastSolar{BaseURL: fs.URL, HTTP: fs.Client()}}
	setup := Setup{Location: oss, Arrays: []Array{{Tilt: 35, PeakKW: 5}}, PerformanceRatio: 0.85, Source: SourceForecastSolar}
	if _, err := c.Forecast(context.Background(), setup); err != nil {
		t.Fatal(err)
	}
	if len(openMeteo) != 0 || len(forecastSolar) != 1 {
		t.Errorf("open-meteo got %d requests and forecast.solar %d, want 0 and 1", len(openMeteo), len(forecastSolar))
	}
	setup.Source = "pvgis"
	if _, err := c.Forecast(context.Background(), setup); err == nil {
		t.Error("an unknown source gave no error")
	}
}

func TestCheckPanels(t *testing.T) {
	good := Array{Tilt: 35, Azimuth: 0, PeakKW: 4.2}
	for _, tc := range []struct {
		name   string
		arrays []Array
		ratio  float64
		source string
		fields []string
	}{
		{"one array", []Array{good}, 0.85, SourceOpenMeteo, nil},
		{"two arrays at the edges", []Array{{Tilt: 0, Azimuth: -180, PeakKW: 0.1}, {Tilt: 90, Azimuth: 180, PeakKW: 100}}, 1, SourceForecastSolar, nil},
		{"none", nil, 0.85, SourceOpenMeteo, []string{"arrays"}},
		{"three", []Array{good, good, good}, 0.85, SourceOpenMeteo, []string{"arrays"}},
		{"out of range", []Array{{Tilt: 91, Azimuth: -181, PeakKW: 0}}, 0.85, SourceOpenMeteo, []string{"arrays[0].tilt", "arrays[0].azimuth", "arrays[0].kwp"}},
		{"ratio and source", []Array{good}, 0.4, "pvgis", []string{"performanceRatio", "source"}},
	} {
		var got []string
		for _, f := range validation.Fields(CheckPanels(tc.arrays, tc.ratio, tc.source)) {
			got = append(got, f.Field)
		}
		if strings.Join(got, ",") != strings.Join(tc.fields, ",") {
			t.Errorf("%s: invalid fields %v, want %v", tc.name, got, tc.fields)
		}
	}
}
