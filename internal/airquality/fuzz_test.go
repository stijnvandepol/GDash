package airquality

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/weather"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func FuzzCurrent(f *testing.F) {
	for _, name := range []string{"netherlands.json", "new-york.json"} {
		body, err := os.ReadFile("testdata/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(body)
	}
	for _, s := range []string{
		"", "{}", "null", "[]", "{",
		`{"current":{"time":"2026-10-09T13:00","european_aqi":1e300}}`,
		`{"current":{"time":"2026-10-09T13:00","european_aqi":1.7976931348623157e308,"birch_pollen":1.7976931348623157e308}}`,
		`{"current":{"time":"2026-10-09T13:00","european_aqi":-1}}`,
		`{"current":{"time":"2026-10-09T13:00","european_aqi":9223372036854775807}}`,
		`{"current":{"time":"2026-10-09T13:00","european_aqi":1e999}}`,
		`{"current":{"time":"\u0000\ud800","european_aqi":0.5,"alder_pollen":-0.0,"grass_pollen":null}}`,
		`{"current":{"european_aqi":"20"}}`,
		`{"current":{"european_aqi":null,"birch_pollen":3}}`,
		`{"current":{"european_aqi":20,"alder_pollen":1,"birch_pollen":2,"grass_pollen":3,"mugwort_pollen":4,"olive_pollen":5,"ragweed_pollen":6}}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		client := Client{BaseURL: "http://air-quality.invalid/v1/air-quality", HTTP: &http.Client{
			Timeout: 5 * time.Second,
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
			}),
		}}
		report, err := client.Current(context.Background(), weather.Location{Latitude: 51.65, Longitude: 5.95})
		if err != nil {
			return
		}
		if report.EuropeanAQI < 0 {
			t.Fatalf("European AQI %d is negative", report.EuropeanAQI)
		}
		if len(report.Pollen) > 6 {
			t.Fatalf("%d kinds of pollen, want at most 6", len(report.Pollen))
		}
		for _, p := range report.Pollen {
			if p.GrainsPerM3 < 0 || math.IsNaN(p.GrainsPerM3) || math.IsInf(p.GrainsPerM3, 0) {
				t.Fatalf("pollen %+v is not a concentration", p)
			}
		}
		if _, err := json.Marshal(report); err != nil {
			t.Fatalf("report cannot be encoded as JSON: %v", err)
		}
	})
}
