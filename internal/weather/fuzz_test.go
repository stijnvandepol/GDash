package weather

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// answering is a client whose server always answers with body, so a fuzz
// iteration costs no socket.
func answering(body []byte) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func FuzzCurrent(f *testing.F) {
	recorded, err := os.ReadFile("testdata/forecast.json")
	if err != nil {
		f.Fatal(err)
	}
	seeds := []string{
		string(recorded),
		"", "{}", "null", "[]", `"string"`, "{",
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":10.2,"weather_code":3},"daily":{}}`,
		`{"current":{"time":"","temperature_2m":1,"weather_code":1},"daily":{"time":["x"],"weather_code":[1],"temperature_2m_max":[1],"temperature_2m_min":[1],"precipitation_probability_max":[1]}}`,
		`{"current":{"time":"2026","temperature_2m":1,"weather_code":1},"daily":{"time":["2026"],"weather_code":[1],"temperature_2m_max":[1],"temperature_2m_min":[1],"precipitation_probability_max":[1]}}`,
		`{"current":{"time":"\u0000\ud800","temperature_2m":1e308,"weather_code":-1,"is_day":2,"wind_speed_10m":-1e308},"daily":{"time":["\u0000"],"weather_code":[9223372036854775807],"temperature_2m_max":[1e308],"temperature_2m_min":[-1e308],"precipitation_probability_max":[-9223372036854775808],"sunrise":["a"],"sunset":["b"]}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":1e999,"weather_code":1}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":1,"weather_code":1.5}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":"warm","weather_code":1}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":null,"weather_code":null}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":1,"weather_code":1},"daily":{"time":["2026-10-08","2026-10-09"],"weather_code":[1],"temperature_2m_max":[1,2,3],"temperature_2m_min":[],"precipitation_probability_max":[1,2],"sunrise":["a","b","c"],"sunset":[]}}`,
		`{"current":{"time":"2026-10-09T13:15","temperature_2m":1,"weather_code":1},"daily":{"time":` + manyStrings(5000, "2026-10-09") + `,"weather_code":` + manyNumbers(5000) + `,"temperature_2m_max":` + manyNumbers(5000) + `,"temperature_2m_min":` + manyNumbers(5000) + `,"precipitation_probability_max":` + manyNumbers(5000) + `,"sunrise":` + manyStrings(5000, "2026-10-09T07:50") + `,"sunset":` + manyStrings(5000, "2026-10-09T18:50") + `}}`,
		strings.Repeat("[", 100000),
		`{"current":` + strings.Repeat(`{"a":`, 10000) + `1` + strings.Repeat(`}`, 10000) + `}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		client := Client{BaseURL: "http://open-meteo.invalid/v1/forecast", HTTP: answering(body)}
		report, err := client.Current(context.Background(), Location{Latitude: 51.65, Longitude: 5.95})
		if err != nil {
			return
		}
		if len(report.Days) == 0 || len(report.Days) > forecastDays {
			t.Fatalf("%d days, want 1 to %d", len(report.Days), forecastDays)
		}
		if report.TodayMaxC != report.Days[0].MaxC || report.TodayMinC != report.Days[0].MinC {
			t.Fatalf("today %v/%v does not match the first day %+v", report.TodayMinC, report.TodayMaxC, report.Days[0])
		}
		if len(report.Sun) > forecastDays+pastDays {
			t.Fatalf("%d sun entries, want at most %d", len(report.Sun), forecastDays+pastDays)
		}
		for _, v := range []float64{report.TemperatureC, report.WindKmh, report.TodayMaxC, report.TodayMinC} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("report holds %v", v)
			}
		}
		// The report travels inside the display snapshot as JSON.
		if _, err := json.Marshal(report); err != nil {
			t.Fatalf("report cannot be encoded as JSON: %v", err)
		}
	})
}

func manyNumbers(n int) string { return "[" + strings.Repeat("1,", n-1) + "1]" }

func manyStrings(n int, s string) string {
	return "[" + strings.Repeat(`"`+s+`",`, n-1) + `"` + s + `"]`
}

func FuzzSearch(f *testing.F) {
	for _, s := range []string{
		`{"results":[{"id":2749234,"name":"Oss","latitude":51.765,"longitude":5.51806,"country":"Netherlands","admin1":"North Brabant"}]}`,
		`{"results":[{"name":"x","latitude":123,"longitude":5},{"name":"","latitude":1,"longitude":1},{"name":"y","latitude":-90,"longitude":180}]}`,
		`{"results":[{"name":"\u0000","latitude":1e999,"longitude":1}]}`,
		`{"results":[{"name":"z","latitude":"1","longitude":1}]}`,
		`{"results":null}`, `{}`, ``, `[]`,
		`{"results":[` + strings.Repeat(`{"name":"Oss","latitude":51.7,"longitude":5.5,"country":"NL","admin1":"NB"},`, 5000) + `{"name":"last","latitude":1,"longitude":1}]}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		g := Geocoder{BaseURL: "http://geocoding.invalid/v1/search", HTTP: answering(body)}
		places, err := g.Search(context.Background(), "Oss")
		if err != nil {
			return
		}
		if places == nil || len(places) > maxPlaces {
			t.Fatalf("%d places (nil: %v), want at most %d", len(places), places == nil, maxPlaces)
		}
		for _, p := range places {
			if _, err := NewLocation(p.Latitude, p.Longitude); err != nil || p.Name == "" {
				t.Fatalf("place %+v is not a usable result", p)
			}
		}
		if _, err := json.Marshal(places); err != nil {
			t.Fatalf("places cannot be encoded as JSON: %v", err)
		}
	})
}

func FuzzNewLocation(f *testing.F) {
	for _, p := range [][2]float64{{51.646, 5.945}, {0, 0}, {90, 180}, {-90, -180}, {90.001, 0}, {math.NaN(), 0}, {0, math.Inf(1)}, {math.Inf(-1), 0}, {1e308, 1e308}, {-0.004, 0.005}} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, lat, lon float64) {
		loc, err := NewLocation(lat, lon)
		if err != nil {
			return
		}
		if math.IsNaN(loc.Latitude) || math.IsNaN(loc.Longitude) || math.Abs(loc.Latitude) > 90 || math.Abs(loc.Longitude) > 180 {
			t.Fatalf("NewLocation(%v, %v) = %+v is not a place on earth", lat, lon, loc)
		}
		again, err := NewLocation(loc.Latitude, loc.Longitude)
		if err != nil || again != loc {
			t.Fatalf("rounding is not stable: %+v then %+v (%v)", loc, again, err)
		}
	})
}
