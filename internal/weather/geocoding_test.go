package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchPlaces(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"results":[
			{"id":2749234,"name":"Oss","latitude":51.765,"longitude":5.51806,"country":"Netherlands","admin1":"North Brabant"},
			{"id":1,"name":"Nowhere","latitude":123,"longitude":5,"country":"Broken"},
			{"id":2,"name":"Oss Island","latitude":-10,"longitude":20}
		]}`))
	}))
	defer srv.Close()
	g := Geocoder{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}

	places, err := g.Search(context.Background(), "  Oss ")
	if err != nil {
		t.Fatal(err)
	}
	if query != "count=8&format=json&language=en&name=Oss" {
		t.Fatalf("query = %s", query)
	}
	if len(places) != 2 || places[0].Region != "North Brabant, Netherlands" || places[0].Latitude != 51.765 {
		t.Fatalf("places = %+v; want two, without the invalid coordinate", places)
	}
	if places[1].Region != "" {
		t.Fatalf("region without admin data = %q, want empty", places[1].Region)
	}
}

func TestSearchWithoutResultsAndErrors(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(`{"generationtime_ms":0.2}`))
	}))
	defer srv.Close()
	g := Geocoder{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}
	if places, err := g.Search(context.Background(), "Xyzzy"); err != nil || places == nil || len(places) != 0 {
		t.Fatalf("no results = %#v, %v; want an empty list", places, err)
	}
	status = http.StatusBadRequest
	if _, err := g.Search(context.Background(), "Xyzzy"); err == nil {
		t.Fatal("Search accepted a 400")
	}
}

func TestValidQuery(t *testing.T) {
	for q, want := range map[string]bool{"": false, " O ": false, "Os": true, "Oss": true, string(make([]rune, 101)): false} {
		if got := ValidQuery(q); got != want {
			t.Errorf("ValidQuery(%q) = %v, want %v", q, got, want)
		}
	}
}
