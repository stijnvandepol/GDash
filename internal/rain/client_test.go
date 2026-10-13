package rain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// stub stands in for Buienradar (/raintext) and Open-Meteo (/forecast). A
// path without a body answers 404, as Buienradar does outside its area.
type stub struct {
	mu       sync.Mutex
	bodies   map[string]string
	requests []string
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, r.URL.Path+"?"+r.URL.RawQuery)
	body, ok := s.bodies[r.URL.Path]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "Not found: location must be inside the Netherlands or Belgium.", http.StatusNotFound)
		return
	}
	_, _ = w.Write([]byte(body))
}

func client(t *testing.T, s *stub, now time.Time) Client {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return Client{
		BuienradarURL: srv.URL + "/raintext",
		OpenMeteoURL:  srv.URL + "/forecast",
		HTTP:          srv.Client(),
		Location:      amsterdam(t),
		Now:           func() time.Time { return now },
	}
}

func TestBuienradarAnswersForAPlaceInTheNetherlands(t *testing.T) {
	s := &stub{bodies: map[string]string{"/raintext": fixture(t, "buienradar-shower.txt")}}
	now := time.Date(2026, 10, 13, 14, 3, 0, 0, amsterdam(t))
	queries := []Query{{Here: true, Latitude: 51.77, Longitude: 5.53}, {Latitude: 51.765, Longitude: 5.531}}

	got, err := client(t, s, now).Forecasts(context.Background(), queries)
	if err != nil {
		t.Fatal(err)
	}
	here, oss := got["here"], got["51.77,5.53"]
	if here.Source != SourceBuienradar || here.StepMinutes != 5 || len(here.Points) != 24 || len(oss.Points) != 24 {
		t.Fatalf("forecasts = %+v, want Buienradar's 24 points for both widgets", got)
	}
	if len(s.requests) != 1 || s.requests[0] != "/raintext?lat=51.77&lon=5.53" {
		t.Fatalf("requests = %v, want one for the place both widgets show, with two decimals", s.requests)
	}
}

func TestOpenMeteoTakesOverWhenBuienradarFails(t *testing.T) {
	s := &stub{bodies: map[string]string{"/forecast": fixture(t, "openmeteo-paris.json")}}
	now := time.Date(2026, 10, 13, 12, 7, 0, 0, time.UTC)

	got, err := client(t, s, now).Forecasts(context.Background(), []Query{{Latitude: 51.44, Longitude: 5.48}})
	if err != nil {
		t.Fatal(err)
	}
	if f := got["51.44,5.48"]; f.Source != SourceOpenMeteo || f.StepMinutes != 15 || len(f.Points) != 8 {
		t.Fatalf("forecast = %+v, want Open-Meteo's quarter hours", f)
	}
	if len(s.requests) != 2 || !strings.HasPrefix(s.requests[1], "/forecast?") ||
		!strings.Contains(s.requests[1], "minutely_15=precipitation") || !strings.Contains(s.requests[1], "timezone=auto") {
		t.Fatalf("requests = %v, want Buienradar first and then Open-Meteo", s.requests)
	}
}

func TestBuienradarIsNotAskedOutsideItsArea(t *testing.T) {
	s := &stub{bodies: map[string]string{"/forecast": fixture(t, "openmeteo-paris.json")}}
	now := time.Date(2026, 10, 13, 12, 7, 0, 0, time.UTC)

	if _, err := client(t, s, now).Forecasts(context.Background(), []Query{{Latitude: 48.86, Longitude: 2.35}}); err != nil {
		t.Fatal(err)
	}
	if len(s.requests) != 1 || !strings.HasPrefix(s.requests[0], "/forecast?") {
		t.Fatalf("requests = %v, want only Open-Meteo for Paris", s.requests)
	}
}

func TestBothServicesFailingFailsTheCall(t *testing.T) {
	s := &stub{bodies: map[string]string{}}
	_, err := client(t, s, time.Now()).Forecasts(context.Background(), []Query{{Latitude: 51.77, Longitude: 5.53}})
	if err == nil || !strings.Contains(err.Error(), "buienradar") || !strings.Contains(err.Error(), "open-meteo") {
		t.Fatalf("err = %v, want one that names both services", err)
	}
}

func TestAPlaceOffTheEarthIsRefused(t *testing.T) {
	s := &stub{bodies: map[string]string{}}
	if _, err := client(t, s, time.Now()).Forecasts(context.Background(), []Query{{Latitude: 91, Longitude: 0}}); err == nil {
		t.Fatal("a latitude of 91 was fetched")
	}
	if len(s.requests) != 0 {
		t.Fatalf("requests = %v, want none", s.requests)
	}
}
