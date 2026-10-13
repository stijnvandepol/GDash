package speedcameras

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/traffic"
)

func recorded(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDecodeReadsCamerasAndTheirRoads(t *testing.T) {
	cameras, err := decode(recorded(t))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Four cameras: the relation next to the camera in Oss is the same
	// camera, and the node without a place is left out.
	if len(cameras) != 4 {
		t.Fatalf("cameras = %+v, want 4", cameras)
	}
	a2 := cameras[0]
	if a2.Kind != traffic.CameraFixed || !slices.Equal(a2.Roads, []string{"A2"}) || a2.MaxSpeed != 100 || a2.Hectometre != 236.4 {
		t.Errorf("camera on the A2 = %+v", a2)
	}
	if !slices.Equal(cameras[1].Roads, []string{"N279"}) {
		t.Errorf("camera on the N279 = %+v", cameras[1])
	}
	// In town: no A or N road, and "signals" is no speed.
	if town := cameras[2]; len(town.Roads) != 0 || town.MaxSpeed != 0 {
		t.Errorf("camera in town = %+v", town)
	}
	// An average speed check takes the road in its name, not the N65 that
	// crosses it.
	if section := cameras[3]; section.Kind != traffic.CameraSection || !slices.Equal(section.Roads, []string{"A2"}) {
		t.Errorf("trajectcontrole = %+v", section)
	}
}

func TestDecodeRefusesAnUnfinishedAnswer(t *testing.T) {
	for _, body := range []string{
		`{"elements":[],"remark":"runtime error: Query timed out in \"query\" at line 4 after 91 seconds."}`,
		`{"version":0.6}`,
		`<html>busy</html>`,
	} {
		if _, err := decode([]byte(body)); err == nil {
			t.Errorf("decode(%s) gave no error", body)
		}
	}
}

func TestClientPostsOneQueryAndNamesGDash(t *testing.T) {
	var query, agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		query, agent = form.Get("data"), r.Header.Get("User-Agent")
		w.Write(recorded(t))
	}))
	t.Cleanup(srv.Close)
	client := Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}, UserAgent: "GDash/test"}
	cameras, err := client.Fetch(context.Background())
	if err != nil || len(cameras) != 4 {
		t.Fatalf("Fetch: %v, %d cameras", err, len(cameras))
	}
	if agent != "GDash/test" || !strings.Contains(query, `"ISO3166-1"="NL"`) || !strings.Contains(query, "[timeout:90]") {
		t.Errorf("agent %q, query %q", agent, query)
	}
}

// fakeOverpass answers with the cameras or the error, and counts.
type fakeOverpass struct {
	calls   atomic.Int32
	cameras []traffic.Camera
	err     error
}

func (f *fakeOverpass) Fetch(context.Context) ([]traffic.Camera, error) {
	f.calls.Add(1)
	return f.cameras, f.err
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

var start = time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)

func newStore(t *testing.T, path string, overpass *fakeOverpass, c *clock) *Store {
	t.Helper()
	return &Store{Client: overpass, Path: path, Now: c.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestStoreAsksAtMostOnceAWeekAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cameras.json")
	overpass := &fakeOverpass{cameras: []traffic.Camera{{Kind: traffic.CameraFixed, Roads: []string{"A2"}, Latitude: 51, Longitude: 5.8}}}
	c := &clock{now: start}
	store := newStore(t, path, overpass, c)
	if _, ok := store.Cameras(); ok {
		t.Error("a list before the first fetch")
	}
	summary, err := store.Refresh(context.Background())
	if err != nil || summary.Cameras != 1 || !summary.FetchedAt.Equal(start) {
		t.Fatalf("Refresh: %+v, %v", summary, err)
	}

	// A restart six days later reads the file and does not ask.
	c.now = start.Add(6 * 24 * time.Hour)
	restarted := newStore(t, path, overpass, c)
	if _, err := restarted.Refresh(context.Background()); err != nil || overpass.calls.Load() != 1 {
		t.Errorf("after a restart: %v, %d queries", err, overpass.calls.Load())
	}
	if cameras, ok := restarted.Cameras(); !ok || len(cameras) != 1 || cameras[0].Roads[0] != "A2" {
		t.Errorf("cameras from disk = %+v, %v", cameras, ok)
	}

	c.now = start.Add(MaxAge)
	restarted.Refresh(context.Background())
	if overpass.calls.Load() != 2 {
		t.Errorf("%d queries after a week, want 2", overpass.calls.Load())
	}
}

func TestStoreKeepsTheOldListAndWaitsAfterAFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cameras.json")
	overpass := &fakeOverpass{cameras: []traffic.Camera{{Kind: traffic.CameraFixed}}}
	c := &clock{now: start}
	store := newStore(t, path, overpass, c)
	store.Refresh(context.Background())

	overpass.err = errors.New("overpass answered 504 Gateway Timeout")
	c.now = start.Add(MaxAge)
	if _, err := store.Refresh(context.Background()); err == nil {
		t.Fatal("a failure gave no error")
	}
	c.now = c.now.Add(time.Hour)
	if _, err := store.Refresh(context.Background()); !errors.Is(err, ErrWaiting) || overpass.calls.Load() != 2 {
		t.Errorf("an hour after a failure: %v, %d queries", err, overpass.calls.Load())
	}
	if cameras, ok := store.Cameras(); !ok || len(cameras) != 1 {
		t.Errorf("the old list is gone: %+v", cameras)
	}
}

func TestStoreIgnoresABrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cameras.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	overpass := &fakeOverpass{}
	store := newStore(t, path, overpass, &clock{now: start})
	if _, err := store.Refresh(context.Background()); err != nil || overpass.calls.Load() != 1 {
		t.Errorf("a broken file must lead to a new fetch: %v, %d", err, overpass.calls.Load())
	}
}
