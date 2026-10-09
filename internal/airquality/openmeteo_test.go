package airquality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/nookboard/internal/weather"
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

func recorded(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCurrentParsesEuropeanResponse(t *testing.T) {
	client, req := serve(t, http.StatusOK, recorded(t, "netherlands.json"))
	loc, _ := weather.NewLocation(51.646, 5.945)

	got, err := client.Current(context.Background(), loc)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.EuropeanAQI != 20 || got.ObservedAt != "2026-10-09T13:00" {
		t.Fatalf("report = %+v", got)
	}
	// Out of season Open-Meteo answers 0 for every kind: they are values,
	// and the widget decides to hide them.
	if len(got.Pollen) != 6 || got.Pollen[0] != (Pollen{Type: "alder"}) || got.Pollen[5].Type != "ragweed" {
		t.Fatalf("pollen = %+v, want six kinds in order", got.Pollen)
	}
	query := req.URL.Query()
	if query.Get("latitude") != "51.65" || query.Get("longitude") != "5.95" {
		t.Fatalf("location sent = %s, %s, want rounded 51.65, 5.95", query.Get("latitude"), query.Get("longitude"))
	}
	for _, variable := range []string{"european_aqi", "birch_pollen", "ragweed_pollen"} {
		if !strings.Contains(query.Get("current"), variable) {
			t.Errorf("current = %q, missing %s", query.Get("current"), variable)
		}
	}
}

func TestCurrentLeavesOutPollenThatIsNull(t *testing.T) {
	client, _ := serve(t, http.StatusOK, recorded(t, "new-york.json"))

	got, err := client.Current(context.Background(), weather.Location{})
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.EuropeanAQI != 54 {
		t.Fatalf("index = %d, want 54", got.EuropeanAQI)
	}
	if len(got.Pollen) != 0 {
		t.Fatalf("pollen = %+v, want none outside Europe", got.Pollen)
	}
}

func TestCurrentKeepsOnlyTheKindsWithAValue(t *testing.T) {
	body := []byte(`{"current":{"time":"2026-04-20T12:00","european_aqi":47.6,"alder_pollen":null,"birch_pollen":212.5,"grass_pollen":0,"mugwort_pollen":null,"olive_pollen":3,"ragweed_pollen":null}}`)
	client, _ := serve(t, http.StatusOK, body)

	got, err := client.Current(context.Background(), weather.Location{})
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	want := []Pollen{{Type: "birch", GrainsPerM3: 212.5}, {Type: "grass"}, {Type: "olive", GrainsPerM3: 3}}
	if !slices.Equal(got.Pollen, want) {
		t.Fatalf("pollen = %+v, want %+v", got.Pollen, want)
	}
	if got.EuropeanAQI != 48 {
		t.Fatalf("index = %d, want 47.6 rounded to 48", got.EuropeanAQI)
	}
}

func TestCurrentKeepsAnIndexOfZero(t *testing.T) {
	client, _ := serve(t, http.StatusOK, []byte(`{"current":{"time":"t","european_aqi":0}}`))
	got, err := client.Current(context.Background(), weather.Location{})
	if err != nil || got.EuropeanAQI != 0 {
		t.Fatalf("Current = %+v, %v; want a valid index of 0", got, err)
	}
}

func TestCurrentRejectsBadResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"error status", http.StatusTooManyRequests, `{"error":true}`},
		{"no current values", http.StatusOK, `{}`},
		{"null index", http.StatusOK, `{"current":{"time":"t","european_aqi":null,"birch_pollen":4}}`},
		{"negative index", http.StatusOK, `{"current":{"time":"t","european_aqi":-3}}`},
		{"not json", http.StatusOK, `<html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := serve(t, tt.status, []byte(tt.body))
			if got, err := client.Current(context.Background(), weather.Location{}); err == nil {
				t.Fatalf("Current accepted it: %+v", got)
			}
		})
	}
}
