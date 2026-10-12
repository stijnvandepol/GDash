package departures

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// fakeLister counts loads and can be made to fail.
type fakeLister struct {
	stops []Stop
	err   error
	loads int
}

func (f *fakeLister) Stops(context.Context) ([]Stop, error) {
	f.loads++
	return f.stops, f.err
}

func newDirectory(src StopLister, now *time.Time) *Directory {
	return &Directory{
		Source: src,
		Now:    func() time.Time { return *now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func names(stops []Stop) string {
	var out []string
	for _, s := range stops {
		out = append(out, s.Name)
	}
	return strings.Join(out, " | ")
}

func TestSearchFindsEveryWordAndRanksWordStartsFirst(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{stops: []Stop{
		{"a1", "Ossendrecht, Kerk"},
		{"a2", "Oss, Station"},
		{"a3", "Boss, Dorp"},
		{"a4", "Utrecht, CS Jaarbeurszijde"},
		{"a5", "Utrecht, CS Centrumzijde"},
		{"a6", "Utrecht, Station Overvecht (Perron B)"},
	}}
	d := newDirectory(lister, &now)

	tests := []struct{ query, want string }{
		// "oss" starts a word in two names and sits inside "Boss".
		{"oss", "Oss, Station | Ossendrecht, Kerk | Boss, Dorp"},
		{"OSS", "Oss, Station | Ossendrecht, Kerk | Boss, Dorp"},
		{"utr cs", "Utrecht, CS Centrumzijde | Utrecht, CS Jaarbeurszijde"},
		{"station utrecht", "Utrecht, Station Overvecht (Perron B)"},
		{"  overvecht ", "Utrecht, Station Overvecht (Perron B)"},
		{"nothing like this", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got, err := d.Search(context.Background(), tt.query)
		if err != nil {
			t.Fatalf("Search(%q): %v", tt.query, err)
		}
		if names(got) != tt.want {
			t.Errorf("Search(%q) = %s, want %s", tt.query, names(got), tt.want)
		}
	}
}

func TestSearchResultsAreLimitedAndNeverNull(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{}
	for i := range 3 * maxResults {
		lister.stops = append(lister.stops, Stop{Code: fmt.Sprintf("c%02d", i), Name: fmt.Sprintf("Oss, Straat %02d", i)})
	}
	d := newDirectory(lister, &now)

	got, err := d.Search(context.Background(), "oss")
	if err != nil || len(got) != maxResults || got[0].Name != "Oss, Straat 00" {
		t.Fatalf("Search = %d results (first %v), %v; want %d", len(got), got[0], err, maxResults)
	}
	if none, _ := d.Search(context.Background(), "zzz"); none == nil {
		t.Fatal("no match gave nil, which the API would send as null")
	}
	if blank, _ := d.Search(context.Background(), " "); blank == nil || lister.loads != 1 {
		t.Fatalf("blank search = %v after %d loads", blank, lister.loads)
	}
}

func TestListIsLoadedOnceADay(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{stops: []Stop{{"a1", "Oss, Station"}}}
	d := newDirectory(lister, &now)
	search := func() { t.Helper(); d.Search(context.Background(), "oss") }

	search()
	now = now.Add(23 * time.Hour)
	search()
	if lister.loads != 1 {
		t.Fatalf("loads after 23 hours = %d, want the list in memory", lister.loads)
	}
	lister.stops = []Stop{{"a1", "Oss, Station"}, {"a2", "Oss, Markt"}}
	now = now.Add(2 * time.Hour)
	got, _ := d.Search(context.Background(), "oss")
	if lister.loads != 2 || len(got) != 2 {
		t.Fatalf("after a day: %d loads, %s; want a fresh list", lister.loads, names(got))
	}
}

func TestFailedRefreshKeepsTheOldListAndWaits(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{stops: []Stop{{"a1", "Oss, Station"}}}
	d := newDirectory(lister, &now)
	if _, err := d.Search(context.Background(), "oss"); err != nil {
		t.Fatal(err)
	}

	lister.err = errors.New("ovapi is down")
	now = now.Add(25 * time.Hour)
	got, err := d.Search(context.Background(), "oss")
	if err != nil || len(got) != 1 {
		t.Fatalf("Search with OVapi down = %v, %v; want the old list", got, err)
	}
	d.Search(context.Background(), "oss")
	if lister.loads != 2 {
		t.Fatalf("loads = %d; a second search a moment later must not ask OVapi again", lister.loads)
	}
	now = now.Add(retryAfter)
	lister.err = nil
	d.Search(context.Background(), "oss")
	if lister.loads != 3 {
		t.Fatalf("loads = %d, want a retry after %s", lister.loads, retryAfter)
	}
}

func TestFirstLoadFailureIsAnError(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{err: errors.New("ovapi is down")}
	d := newDirectory(lister, &now)

	if _, err := d.Search(context.Background(), "oss"); err == nil {
		t.Fatal("no list and no error")
	}
	lister.err, lister.stops = nil, []Stop{{"a1", "Oss, Station"}}
	if got, err := d.Search(context.Background(), "oss"); err != nil || len(got) != 1 {
		t.Fatalf("Search after OVapi came back = %v, %v; want it to load at once", got, err)
	}
}
