package casting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/displays"
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

func (r *recorder) run(ctx context.Context, d displays.Display, keep Keep) {
	keep.Report(cast.StateShowing, nil)
	r.mu.Lock()
	r.running[d.ID] = d.CastDeviceID
	r.mu.Unlock()
	r.events <- "start " + d.ID
	for running := true; running; {
		select {
		case <-keep.TakeOver:
			r.events <- "take over " + d.ID
		case <-keep.Hidden:
			keep.Restarted(cast.RestartHidden)
			r.events <- "hidden " + d.ID
		case <-ctx.Done():
			running = false
		}
	}
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

func TestManagerKeepsTheLatestStatusPerDisplay(t *testing.T) {
	clock := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	m := &Manager{Now: func() time.Time { return clock }}
	report := m.reporter("kitchen")

	report(cast.StateShowing, nil)
	clock = clock.Add(30 * time.Second)
	report(cast.StateShowing, nil) // a status poll: nothing changed
	if got := m.Statuses()["kitchen"]; got.State != cast.StateShowing || !got.Since.Equal(clock.Add(-30*time.Second)) {
		t.Fatalf("status = %+v; want showing since the first report", got)
	}

	clock = clock.Add(time.Minute)
	report(cast.StateUnreachable, errors.New("no answer from 192.168.1.160:8009"))
	got := m.Statuses()["kitchen"]
	if got.State != cast.StateUnreachable || got.Problem != "no answer from 192.168.1.160:8009" || got.Reason != ReasonNoAnswer || !got.Since.Equal(clock) {
		t.Fatalf("status = %+v", got)
	}

	report(cast.StateUnreachable, fmt.Errorf("%w: abc123", cast.ErrDeviceNotFound))
	if got := m.Statuses()["kitchen"]; got.Reason != ReasonNotFound {
		t.Fatalf("reason = %q for a device discovery did not find, want %q", got.Reason, ReasonNotFound)
	}
	report(cast.StateShowing, nil)
	if got := m.Statuses()["kitchen"]; got.Reason != "" || got.Problem != "" {
		t.Fatalf("status = %+v; a device on screen has no problem", got)
	}

	m.forget("kitchen")
	if _, ok := m.Statuses()["kitchen"]; ok {
		t.Fatal("status of a stopped keeper is still listed")
	}
}

func TestManagerForgetsStatusOfRemovedDisplay(t *testing.T) {
	list := &fakeList{}
	list.set(displays.Display{ID: "kitchen", CastDeviceID: "aaa"})
	rec := newRecorder()
	m := &Manager{List: list.get, RunDisplay: rec.run, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	rec.expect(t, "start kitchen")
	if m.Statuses()["kitchen"].State != cast.StateShowing {
		t.Fatalf("statuses = %v", m.Statuses())
	}
	list.set()
	m.Changed()
	rec.expect(t, "stop kitchen")
	deadline := time.Now().Add(2 * time.Second)
	for len(m.Statuses()) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("statuses after removing the display = %v", m.Statuses())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerPassesTakeOverToTheDisplaysKeeper(t *testing.T) {
	list := &fakeList{}
	list.set(displays.Display{ID: "kitchen", CastDeviceID: "aaa"}, displays.Display{ID: "browser-only"})
	rec := newRecorder()
	m := &Manager{List: list.get, RunDisplay: rec.run, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	rec.expect(t, "start kitchen")

	if !m.TakeOver("kitchen") {
		t.Fatal("TakeOver(kitchen) = false for a running keeper")
	}
	rec.expect(t, "take over kitchen")
	if m.TakeOver("browser-only") || m.TakeOver("unknown") {
		t.Fatal("TakeOver = true for a display without a keeper")
	}

	list.set()
	m.Changed()
	rec.expect(t, "stop kitchen")
	deadline := time.Now().Add(2 * time.Second)
	for m.TakeOver("kitchen") {
		if time.Now().After(deadline) {
			t.Fatal("TakeOver still reaches the keeper of a removed display")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerPassesHiddenScreensAndRecordsRestarts(t *testing.T) {
	clock := time.Date(2026, 10, 14, 11, 52, 0, 0, time.UTC)
	list := &fakeList{}
	list.set(displays.Display{ID: "kitchen", CastDeviceID: "aaa"})
	rec := newRecorder()
	m := &Manager{List: list.get, RunDisplay: rec.run, Now: func() time.Time { return clock }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	rec.expect(t, "start kitchen")

	if !m.ScreenHidden("kitchen") {
		t.Fatal("ScreenHidden(kitchen) = false for a running keeper")
	}
	rec.expect(t, "hidden kitchen")
	got := m.Statuses()["kitchen"]
	if got.HiddenAt == nil || !got.HiddenAt.Equal(clock) || got.RestartedAt == nil || !got.RestartedAt.Equal(clock) || got.RestartReason != cast.RestartHidden {
		t.Fatalf("status = %+v, want hidden and restarted at 11:52 for the hidden screen", got)
	}
	if m.ScreenHidden("unknown") {
		t.Fatal("ScreenHidden = true for a display without a keeper")
	}

	// A change of state keeps when the receiver was last restarted.
	m.reporter("kitchen")(cast.StateStandby, nil)
	if got := m.Statuses()["kitchen"]; got.State != cast.StateStandby || got.RestartedAt == nil || got.HiddenAt == nil {
		t.Fatalf("status after standby = %+v, want the restart and hidden times kept", got)
	}
}
