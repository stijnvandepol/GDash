package fuel

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// serve answers like the ANWB and counts and remembers the requests.
func serve(t *testing.T, status int, body []byte) (Client, *http.Request, *atomic.Int32) {
	t.Helper()
	var got http.Request
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		count.Add(1)
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}, UserAgent: "GDash-test"}, &got, &count
}

func TestNearbyAsksOncePerAreaAndKeysTheAnswers(t *testing.T) {
	client, req, count := serve(t, http.StatusOK, recorded(t))
	queries := []Query{
		{Area: oss, Fuel: "EURO95"},
		{Area: oss, Fuel: "DIESEL"},
		{Here: true, Area: Area{Latitude: 51.77, Longitude: 5.52, RadiusKm: 3}, Fuel: "CNG"},
	}

	got, err := client.Nearby(context.Background(), queries)
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	if count.Load() != 2 {
		t.Errorf("%d requests, want one per area (2)", count.Load())
	}
	for _, key := range []string{"51.76,5.52,3,EURO95", "51.76,5.52,3,DIESEL", "here,3,CNG"} {
		if _, ok := got[key]; !ok {
			t.Errorf("no answer for %q in %v", key, got)
		}
	}
	if got["here,3,CNG"][0].Name != "CNG Express, Oss" {
		t.Errorf("CNG list = %+v", got["here,3,CNG"])
	}
	if req.Header.Get("User-Agent") != "GDash-test" || req.Header.Get("Accept") != "application/json" {
		t.Errorf("headers = %v", req.Header)
	}
	if req.URL.Query().Get("type-filter") != "FUEL_STATION" {
		t.Errorf("type-filter = %q", req.URL.Query().Get("type-filter"))
	}
	box := strings.Split(req.URL.Query().Get("bounding-box-filter"), ",")
	if len(box) != 4 {
		t.Fatalf("bounding-box-filter = %q, want minLat,minLon,maxLat,maxLon", req.URL.Query().Get("bounding-box-filter"))
	}
	if !strings.HasPrefix(box[0], "51.7") || !strings.HasPrefix(box[1], "5.") {
		t.Errorf("box %v does not start at the south-west corner", box)
	}
}

func TestNearbyFailsOnABadStatusAndKeepsNothing(t *testing.T) {
	client, _, _ := serve(t, http.StatusServiceUnavailable, []byte("down"))
	got, err := client.Nearby(context.Background(), []Query{{Area: oss, Fuel: "EURO95"}})
	if err == nil || got != nil {
		t.Fatalf("Nearby = %v, %v; want an error and no partial answer", got, err)
	}
}

func TestNearbyRefusesAnAnswerThatIsTooLarge(t *testing.T) {
	client, _, _ := serve(t, http.StatusOK, []byte(`{"value":[`+strings.Repeat(" ", maxAnswerBytes)+`]}`))
	if _, err := client.Nearby(context.Background(), []Query{{Area: oss, Fuel: "EURO95"}}); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("err = %v, want the size limit to stop it", err)
	}
}

func TestNearbyRefusesQueriesThatAreNotValid(t *testing.T) {
	client, _, count := serve(t, http.StatusOK, recorded(t))
	for _, q := range []Query{
		{Area: Area{Latitude: 91, Longitude: 5, RadiusKm: 5}, Fuel: "EURO95"},
		{Area: Area{Latitude: 51, Longitude: 181, RadiusKm: 5}, Fuel: "EURO95"},
		{Area: Area{Latitude: 51, Longitude: 5, RadiusKm: 0}, Fuel: "EURO95"},
		{Area: Area{Latitude: 51, Longitude: 5, RadiusKm: 26}, Fuel: "EURO95"},
		{Area: Area{Latitude: math.NaN(), Longitude: 5, RadiusKm: 5}, Fuel: "EURO95"},
		{Area: oss, Fuel: "DIESEL_SPECIAL"},
		{Area: oss, Fuel: "x&y"},
	} {
		if _, err := client.Nearby(context.Background(), []Query{q}); err == nil {
			t.Errorf("%+v was accepted", q)
		}
	}
	if count.Load() != 0 {
		t.Errorf("%d requests were sent for invalid queries", count.Load())
	}
}

func TestNearbyWithoutQueriesAsksNothing(t *testing.T) {
	client, _, count := serve(t, http.StatusOK, recorded(t))
	got, err := client.Nearby(context.Background(), nil)
	if err != nil || len(got) != 0 || count.Load() != 0 {
		t.Fatalf("got %v, %v after %d requests, want nothing", got, err, count.Load())
	}
}
