package knmi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testKey is a made-up key; the stand-in below accepts only it.
const testKey = "made-up-knmi-test-key"

var now = time.Date(2026, 10, 13, 10, 25, 0, 0, time.UTC)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// standIn answers like the EDR API for the test key, and counts station
// list requests.
type standIn struct {
	client   *Client
	lists    atomic.Int32
	asked    []string
	datetime string
}

func newStandIn(t *testing.T) *standIn {
	t.Helper()
	s := &standIn{}
	locations, observation := fixture(t, "locations.json"), fixture(t, "observation.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			return
		}
		switch {
		case r.URL.Path == "/locations":
			s.lists.Add(1)
			w.Write(locations)
		case strings.HasPrefix(r.URL.Path, "/locations/"):
			s.asked = append(s.asked, strings.TrimPrefix(r.URL.Path, "/locations/"))
			s.datetime = r.URL.Query().Get("datetime")
			if r.URL.Query().Get("parameter-name") != "ta,rh,ff" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if strings.HasSuffix(r.URL.Path, "06375") {
				w.Write(observation)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	s.client = &Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}, UserAgent: "GDash/test", Now: func() time.Time { return now }}
	return s
}

func TestLatestReadsTheNearestStation(t *testing.T) {
	s := newStandIn(t)
	// Uden is 10 km from Volkel.
	obs, err := s.client.Latest(context.Background(), testKey, 51.66, 5.62)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if obs.Station != "Volkel" || obs.DistanceKm < 5 || obs.DistanceKm > 7 {
		t.Errorf("station = %q at %.1f km", obs.Station, obs.DistanceKm)
	}
	// The newest temperature is that of 10:10; 10:20 is still null.
	if *obs.TemperatureC != 12.6 || !obs.ObservedAt.Equal(time.Date(2026, 10, 13, 10, 10, 0, 0, time.UTC)) {
		t.Errorf("temperature = %v at %v", *obs.TemperatureC, obs.ObservedAt)
	}
	// Humidity is in at 10:20; wind 4.2 m/s is 15.1 km/h.
	if *obs.Humidity != 86 || *obs.WindKmh != 15.1 {
		t.Errorf("humidity %v, wind %v", *obs.Humidity, *obs.WindKmh)
	}
	if s.datetime != "2026-10-13T09:25:00Z/2026-10-13T10:25:00Z" {
		t.Errorf("datetime = %q", s.datetime)
	}
}

func TestLatestTriesTheNextStation(t *testing.T) {
	s := newStandIn(t)
	// Eindhoven is nearest to Veldhoven, but answers nothing here.
	obs, err := s.client.Latest(context.Background(), testKey, 51.42, 5.40)
	if err != nil || obs.Station != "Volkel" {
		t.Fatalf("Latest = %+v, %v", obs, err)
	}
	if len(s.asked) != 2 || !strings.HasSuffix(s.asked[0], "06370") {
		t.Errorf("asked %v, want Eindhoven first", s.asked)
	}
}

func TestLatestFarFromAnyStation(t *testing.T) {
	s := newStandIn(t)
	if _, err := s.client.Latest(context.Background(), testKey, 64.1, -21.9); !errors.Is(err, ErrNoStation) {
		t.Errorf("Reykjavik: %v", err)
	}
}

func TestStationListIsKeptForADay(t *testing.T) {
	s := newStandIn(t)
	for range 3 {
		if _, err := s.client.Latest(context.Background(), testKey, 51.66, 5.62); err != nil {
			t.Fatal(err)
		}
	}
	if s.lists.Load() != 1 {
		t.Errorf("%d station list requests, want 1", s.lists.Load())
	}
}

func TestAWrongKeyIsNamedAndNeverShown(t *testing.T) {
	s := newStandIn(t)
	const wrong = "wrong-made-up-knmi-key"
	err := s.client.Check(context.Background(), wrong)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Check = %v, want ErrUnauthorized", err)
	}
	_, latestErr := s.client.Latest(context.Background(), wrong, 51.66, 5.62)
	for _, e := range []error{err, latestErr} {
		if e == nil || strings.Contains(e.Error(), wrong) {
			t.Errorf("error %v holds the key", e)
		}
	}
	if err := s.client.Check(context.Background(), testKey); err != nil {
		t.Errorf("Check with the right key: %v", err)
	}
}

func TestAnUnreachableServerDoesNotShowTheKey(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:1", HTTP: &http.Client{Timeout: time.Second}, Now: func() time.Time { return now }}
	err := c.Check(context.Background(), testKey)
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("error = %v", err)
	}
}

func TestDecodeObservationTakesASingleCoverageToo(t *testing.T) {
	body := `{"type":"Coverage","domain":{"axes":{"t":{"values":["2026-10-13T10:10:00Z"]}}},"ranges":{"ta":{"values":[-3.5]}}}`
	obs, err := decodeObservation([]byte(body))
	if err != nil || *obs.TemperatureC != -3.5 || obs.Humidity != nil || obs.WindKmh != nil {
		t.Errorf("observation = %+v, %v", obs, err)
	}
}

func TestDecodeObservationRefusesWhatItCannotTrust(t *testing.T) {
	for name, body := range map[string]string{
		"no temperature":   `{"type":"Coverage","domain":{"axes":{"t":{"values":["2026-10-13T10:10:00Z"]}}},"ranges":{"rh":{"values":[80]}}}`,
		"only nulls":       `{"type":"Coverage","domain":{"axes":{"t":{"values":["2026-10-13T10:10:00Z"]}}},"ranges":{"ta":{"values":[null]}}}`,
		"times off":        `{"type":"Coverage","domain":{"axes":{"t":{"values":[]}}},"ranges":{"ta":{"values":[10]}}}`,
		"empty collection": `{"type":"CoverageCollection","coverages":[]}`,
		"not json":         `<html>`,
	} {
		if _, err := decodeObservation([]byte(body)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestStationNames(t *testing.T) {
	for in, want := range map[string]string{"VOLKEL": "Volkel", "VALKENBURG VK": "Valkenburg Vk", "IJmond": "IJmond", "": "0-20000-0-1"} {
		if got := stationName(in, "0-20000-0-1"); got != want {
			t.Errorf("stationName(%q) = %q, want %q", in, got, want)
		}
	}
}
