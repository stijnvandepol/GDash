package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stijnvandepol/gdash/internal/departures"
)

// fakeStops stands in for the directory of stop areas. Tests change its
// answer while the server runs, hence the lock.
type fakeStops struct {
	mu    sync.Mutex
	stops []departures.Stop
	err   error
	asked string
}

func (f *fakeStops) Search(_ context.Context, query string) ([]departures.Stop, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = query
	return f.stops, f.err
}

func (f *fakeStops) set(stops []departures.Stop, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops, f.err = stops, err
}

func (f *fakeStops) lastQuery() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

func TestDepartureStopSearch(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/departures/stops?q=utrecht", ""); status != http.StatusUnauthorized {
		t.Fatalf("without login = %d, want 401", status)
	}
	env.setUp(t)
	env.stops.set([]departures.Stop{{Code: "utrcsw", Name: "Utrecht, CS Jaarbeurszijde"}}, nil)

	status, body := env.do(t, "GET", "/api/departures/stops?q=utrecht+cs", "")
	if status != http.StatusOK || body != `{"stops":[{"code":"utrcsw","name":"Utrecht, CS Jaarbeurszijde"}]}`+"\n" {
		t.Fatalf("search = %d %q", status, body)
	}
	if got := env.stops.lastQuery(); got != "utrecht cs" {
		t.Errorf("the search received %q, want the decoded query", got)
	}

	for _, q := range []string{"", "u", " u ", strings.Repeat("a", 101)} {
		status, body := env.do(t, "GET", "/api/departures/stops?q="+url.QueryEscape(q), "")
		if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"q"`) {
			t.Errorf("q=%q = %d %s, want 422 on q", q, status, body)
		}
	}
}

func TestDepartureStopSearchWhenOVapiIsDown(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.stops.set(nil, errors.New("dial tcp 136.243.70.142:80: i/o timeout"))

	status, body := env.do(t, "GET", "/api/departures/stops?q=utrecht", "")
	if status != http.StatusBadGateway || strings.Contains(body, "136.243") || !strings.Contains(body, "upstream_unavailable") {
		t.Fatalf("OVapi down = %d %s, want 502 without internals", status, body)
	}
	if !strings.Contains(env.logs.String(), "i/o timeout") {
		t.Error("the cause was not logged")
	}
}
