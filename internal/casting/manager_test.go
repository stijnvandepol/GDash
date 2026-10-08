package casting

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/displays"
)

// recorder tracks which displays have a running keeper.
type recorder struct {
	mu      sync.Mutex
	running map[string]string
	events  chan string
}

func newRecorder() *recorder {
	return &recorder{running: map[string]string{}, events: make(chan string, 32)}
}

func (r *recorder) run(ctx context.Context, d displays.Display) {
	r.mu.Lock()
	r.running[d.ID] = d.CastDeviceID
	r.mu.Unlock()
	r.events <- "start " + d.ID
	<-ctx.Done()
	r.mu.Lock()
	delete(r.running, d.ID)
	r.mu.Unlock()
	r.events <- "stop " + d.ID
}

func (r *recorder) expect(t *testing.T, want ...string) {
	t.Helper()
	var got []string
	for range want {
		select {
		case e := <-r.events:
			got = append(got, e)
		case <-time.After(2 * time.Second):
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

type fakeList struct {
	mu   sync.Mutex
	list []displays.Display
}

func (f *fakeList) set(list ...displays.Display) {
	f.mu.Lock()
	f.list = list
	f.mu.Unlock()
}

func (f *fakeList) get(context.Context) ([]displays.Display, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]displays.Display(nil), f.list...), nil
}

func TestManagerFollowsDisplayChanges(t *testing.T) {
	list := &fakeList{}
	list.set(
		displays.Display{ID: "kitchen", CastDeviceID: "aaa"},
		displays.Display{ID: "browser-only"},
	)
	rec := newRecorder()
	m := &Manager{List: list.get, RunDisplay: rec.run, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { m.Run(ctx); close(stopped) }()

	rec.expect(t, "start kitchen")

	list.set(
		displays.Display{ID: "kitchen", CastDeviceID: "bbb"},
		displays.Display{ID: "living-room", CastDeviceID: "ccc"},
	)
	m.Changed()
	rec.expect(t, "stop kitchen", "start kitchen", "start living-room")

	list.set(displays.Display{ID: "living-room", CastDeviceID: "ccc"})
	m.Changed()
	rec.expect(t, "stop kitchen")

	cancel()
	<-stopped
	rec.expect(t, "stop living-room")
	if len(rec.running) != 0 {
		t.Fatalf("keepers still running after Run returned: %v", rec.running)
	}
}

func TestChangedNeverBlocks(t *testing.T) {
	m := &Manager{}
	for range 10 {
		m.Changed()
	}
}
