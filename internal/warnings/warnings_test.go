package warnings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// testdata/feeds-netherlands.json is a small feed in the shape MeteoAlarm
// served on 2026-10-13: the "no warnings" entry of a province, an orange
// wind warning that replaced a yellow one (both still listed), a yellow rain
// warning later today, a red one named only by area, an expired one, a
// cancellation, an exercise and two entries that cannot be read.
func recorded(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/feeds-netherlands.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

var morning = time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)

func TestFeedBecomesWarningsPerRegion(t *testing.T) {
	got, err := parse(recorded(t), morning)
	if err != nil {
		t.Fatal(err)
	}
	brabant := got["Noord-Brabant"]
	if len(brabant) != 2 {
		t.Fatalf("Noord-Brabant has %d warnings, want the orange wind and the yellow rain: %+v", len(brabant), brabant)
	}
	wind, rain := brabant[0], brabant[1]
	// The update keeps the identifier of the first alert, and the newest
	// version (orange) wins over the yellow one it replaced.
	if wind.ID != "2.49.0.0.528.0.NL.20261012210000.aaaa-wind_1" || wind.Level != LevelOrange || wind.Kind != "wind" {
		t.Errorf("wind = %+v, want the orange wind warning under its first identifier", wind)
	}
	if wind.Event["nl"] != "Zware windstoten" || wind.Headline["en"] != "Code orange: severe gusts in the south" {
		t.Errorf("texts = %v and %v, want the KNMI's Dutch and English texts", wind.Event, wind.Headline)
	}
	if wind.Onset.Format(time.RFC3339) != "2026-10-13T12:00:00Z" || wind.Expires.Format(time.RFC3339) != "2026-10-13T22:00:00Z" {
		t.Errorf("window = %v to %v", wind.Onset, wind.Expires)
	}
	if rain.Level != LevelYellow || rain.Kind != "rain" {
		t.Errorf("second = %+v, want the yellow rain warning, which starts later", rain)
	}
	if limburg := got["Limburg"]; len(limburg) != 1 || limburg[0].ID != wind.ID {
		t.Errorf("Limburg = %+v, want the same wind warning, which names both provinces", limburg)
	}
	if zeeland := got["Zeeland"]; len(zeeland) != 1 || zeeland[0].Level != LevelRed || zeeland[0].Kind != "thunderstorm" {
		t.Errorf("Zeeland = %+v, want the red warning that names the area without a code", zeeland)
	}
	// Expired, cancelled, an exercise and an unreadable alert: nothing.
	if utrecht := got["Utrecht"]; len(utrecht) != 0 {
		t.Errorf("Utrecht = %+v, want no warnings", utrecht)
	}
}

func TestWarningIsInEffectBetweenOnsetAndExpiry(t *testing.T) {
	w := Warning{Onset: morning, Expires: morning.Add(time.Hour)}
	for at, want := range map[time.Time]bool{
		morning.Add(-time.Minute): false, morning: true, morning.Add(59 * time.Minute): true, morning.Add(time.Hour): false,
	} {
		if got := w.InEffect(at); got != want {
			t.Errorf("InEffect(%v) = %v, want %v", at, got, want)
		}
	}
}

func TestAFeedThatIsNoFeedFails(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "<html>",
		"no warnings":   `{"error":"maintenance"}`,
		"wrong type":    `{"warnings":"none"}`,
		"truncated end": `{"warnings":[{"alert":`,
	} {
		if _, err := parse([]byte(body), morning); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	got, err := parse([]byte(`{"warnings":[]}`), morning)
	if err != nil || len(got) != 0 {
		t.Fatalf("an empty list = %v, %v; want no warnings and no error", got, err)
	}
}

func TestClientReadsTheFeedOnceForAllRegions(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "GDash") {
			t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
		}
		_, _ = w.Write(recorded(t))
	}))
	t.Cleanup(srv.Close)
	c := Client{BaseURL: srv.URL, HTTP: srv.Client(), UserAgent: "GDash/test", Now: func() time.Time { return morning }}

	got, err := c.ForRegions(context.Background(), []Query{{Here: true, Region: "Noord-Brabant"}, {Region: "Utrecht"}, {Region: "Zeeland"}})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("%d requests, want one", requests)
	}
	if here := got["here"]; here.Region != "Noord-Brabant" || len(here.Warnings) != 2 {
		t.Errorf("here = %+v, want Noord-Brabant's two warnings", here)
	}
	if encoded, err := json.Marshal(got["Utrecht"]); err != nil || string(encoded) != `{"region":"Utrecht","warnings":[]}` {
		t.Errorf("Utrecht = %s, %v; want the region with an empty list", encoded, err)
	}

	if _, err := c.ForRegions(context.Background(), []Query{{Region: "Atlantis"}}); err == nil {
		t.Error("an unknown region was fetched")
	}
}

func TestClientKeepsTheLastFeedWhenMeteoAlarmFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c := Client{BaseURL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return morning }}
	if _, err := c.ForRegions(context.Background(), []Query{{Region: "Utrecht"}}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v, want the status", err)
	}
}

func TestRegionAtFindsTheProvince(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
		want     string
	}{
		{"Oss", 51.77, 5.53, "Noord-Brabant"},
		{"Utrecht", 52.09, 5.12, "Utrecht"},
		{"Amersfoort", 52.16, 5.39, "Utrecht"},
		{"Almere", 52.37, 5.22, "Flevoland"},
		{"Arnhem", 51.98, 5.91, "Gelderland"},
		{"Amsterdam", 52.37, 4.89, "Noord-Holland"},
		{"Rotterdam", 51.92, 4.48, "Zuid-Holland"},
		{"Maastricht", 50.85, 5.69, "Limburg"},
		{"Middelburg", 51.50, 3.61, "Zeeland"},
		{"Terschelling", 53.36, 5.22, "Waddeneilanden"},
	} {
		if got, ok := RegionAt(tc.lat, tc.lon); !ok || got != tc.want {
			t.Errorf("RegionAt(%s) = %q, %v; want %q", tc.name, got, ok, tc.want)
		}
	}
	for name, place := range map[string][2]float64{"Antwerp": {51.22, 4.40}, "Paris": {48.86, 2.35}, "Cologne": {50.94, 6.96}} {
		if got, ok := RegionAt(place[0], place[1]); ok {
			t.Errorf("RegionAt(%s) = %q, want no Dutch region", name, got)
		}
	}
}

func TestKeysMatchTheWidget(t *testing.T) {
	if got := (Query{Here: true, Region: "Utrecht"}).Key(); got != "here" {
		t.Errorf("Key = %q, want here", got)
	}
	if got := (Query{Region: "Noord-Brabant"}).Key(); got != "Noord-Brabant" {
		t.Errorf("Key = %q, want the region", got)
	}
}
