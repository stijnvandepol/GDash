package nowplaying

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
)

const (
	speaker = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	group   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tv      = "cccccccccccccccccccccccccccccccc"
)

var network = []cast.Device{
	{ID: speaker, Name: "Kitchen speaker", Model: "Google Nest Mini", Addr: "192.0.2.10:8009"},
	{ID: group, Name: "Whole house", Model: groupModel, Addr: "192.0.2.10:32187"},
	{ID: tv, Name: "Living room TV", Model: "Chromecast", Addr: "192.0.2.20:8009", HasScreen: true},
}

// fakeFollow stands in for cast.Monitor: it resolves its device like a
// monitor does and lets the test report what the device plays.
type fakeFollow struct {
	mu      sync.Mutex
	reports map[string]func(cast.Playing)
	stopped map[string]bool
	started chan string
}

func (f *fakeFollow) follow(ctx context.Context, resolve func(context.Context) (cast.Device, error), report func(cast.Playing)) {
	d, err := resolve(ctx)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.reports[d.ID] = report
	delete(f.stopped, d.ID)
	f.mu.Unlock()
	f.started <- d.ID
	<-ctx.Done()
	f.mu.Lock()
	f.stopped[d.ID] = true
	f.mu.Unlock()
}

func (f *fakeFollow) report(t *testing.T, id string, p cast.Playing) {
	t.Helper()
	f.mu.Lock()
	report := f.reports[id]
	f.mu.Unlock()
	if report == nil {
		t.Fatalf("%s is not followed", id)
	}
	report(p)
}

type harness struct {
	watcher   *Watcher
	follow    *fakeFollow
	published chan map[string]Item
	discovers atomic.Int32
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		follow:    &fakeFollow{reports: map[string]func(cast.Playing){}, stopped: map[string]bool{}, started: make(chan string, 16)},
		published: make(chan map[string]Item, 64),
	}
	h.watcher = &Watcher{
		Discover: func(context.Context) ([]cast.Device, error) {
			h.discovers.Add(1)
			return network, nil
		},
		Publish: func(v map[string]Item) { h.published <- v },
		Artwork: &Artwork{Now: time.Now},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Follow:  h.follow.follow,
	}
	return h
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// started collects the devices that start being followed until want of
// them have.
func (h *harness) started(t *testing.T, ctx context.Context, want int) []string {
	t.Helper()
	var ids []string
	for len(ids) < want {
		select {
		case id := <-h.follow.started:
			ids = append(ids, id)
		case <-ctx.Done():
			t.Fatalf("followed %v, want %d devices", ids, want)
		}
	}
	slices.Sort(ids)
	return ids
}

// latest waits for a published value that matches.
func (h *harness) latest(t *testing.T, ctx context.Context, what string, match func(map[string]Item) bool) map[string]Item {
	t.Helper()
	for {
		select {
		case v := <-h.published:
			if match(v) {
				return v
			}
		case <-ctx.Done():
			t.Fatalf("never published %s", what)
		}
	}
}

func TestWatcherFollowsTheChosenDevicesButNeverADisplayTV(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{IDs: []string{speaker, tv}, Exclude: []string{tv}})
	if got := h.started(t, ctx, 1); !slices.Equal(got, []string{speaker}) {
		t.Fatalf("followed %v", got)
	}
	// Nothing plays yet, but the screens must hear that.
	h.latest(t, ctx, "an empty value", func(v map[string]Item) bool { return v != nil && len(v) == 0 })
	select {
	case id := <-h.follow.started:
		t.Fatalf("also followed %s", id)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWatcherWithAnyFollowsEveryDeviceButTheDisplayTVs(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{Any: true, Exclude: []string{tv}})
	if got := h.started(t, ctx, 2); !slices.Equal(got, []string{speaker, group}) {
		t.Fatalf("followed %v", got)
	}
	// The monitors found their devices in the list the scan fetched.
	if n := h.discovers.Load(); n != 1 {
		t.Fatalf("discovered %d times, want 1 shared discovery", n)
	}
}

func TestWatcherStopsFollowingWhatNoWidgetShows(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{IDs: []string{speaker}})
	h.started(t, ctx, 1)
	h.follow.report(t, speaker, cast.Playing{App: "Spotify", Media: cast.Media{Title: "Song", State: cast.PlayerPlaying}, At: time.Now()})
	h.latest(t, ctx, "the song", func(v map[string]Item) bool { return len(v) == 1 })

	h.watcher.Configure(ctx, Want{})
	h.latest(t, ctx, "nothing", func(v map[string]Item) bool { return len(v) == 0 })
	deadline := time.Now().Add(time.Second)
	for {
		h.follow.mu.Lock()
		stopped := h.follow.stopped[speaker]
		h.follow.mu.Unlock()
		if stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still following the speaker")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// A late report of the stopped monitor changes nothing.
	h.follow.report(t, speaker, cast.Playing{App: "Spotify", Media: cast.Media{Title: "Late", State: cast.PlayerPlaying}, At: time.Now()})
	select {
	case v := <-h.published:
		t.Fatalf("published %v after the widget was removed", v)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWatcherPublishesWhatPlays(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{IDs: []string{speaker, group}})
	h.started(t, ctx, 2)
	at := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	cover := "https://i.scdn.co/image/ab67616d0000b273"
	song := cast.Playing{App: "Spotify", At: at, Media: cast.Media{
		Title: "One More Time", Artist: "Daft Punk", Album: "Discovery", Image: cover,
		State: cast.PlayerPlaying, Position: 60, Duration: 320,
	}}
	h.follow.report(t, group, song)
	got := h.latest(t, ctx, "the song", func(v map[string]Item) bool { return len(v) == 1 })
	want := Item{
		Name: "Whole house", Group: true, App: "Spotify", Title: "One More Time", Artist: "Daft Punk", Album: "Discovery",
		Image: ArtworkPath + artworkID(cover), State: StatePlaying, Position: 60, Duration: 320, At: at,
	}
	if got[group] != want {
		t.Fatalf("published %+v\nwant      %+v", got[group], want)
	}
	if strings.Contains(got[group].Image, "scdn") {
		t.Fatal("the cover address reached the screens")
	}

	// The same song a few seconds on, as an app says now and then: the
	// screens move the position on themselves.
	later := song
	later.At, later.Position = at.Add(20*time.Second), 80.5
	h.follow.report(t, group, later)
	// A seek is news.
	seek := song
	seek.At, seek.Position = at.Add(30*time.Second), 200
	h.follow.report(t, group, seek)
	// The next value the screens get is the seek, not the drift before it.
	if got := h.latest(t, ctx, "the seek", func(map[string]Item) bool { return true }); got[group].Position != 200 {
		t.Fatalf("published %+v, want the seek to 200 s", got[group])
	}

	paused := seek
	paused.State = cast.PlayerPaused
	h.follow.report(t, group, paused)
	h.latest(t, ctx, "the pause", func(v map[string]Item) bool { return v[group].State == StatePaused })

	// Idle, or nothing at all, takes the device out.
	idle := paused
	idle.State = cast.PlayerIdle
	h.follow.report(t, group, idle)
	h.latest(t, ctx, "the end", func(v map[string]Item) bool { _, ok := v[group]; return !ok })
}

func TestWatcherPublishesNothingForADriftingPosition(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{IDs: []string{speaker}})
	h.started(t, ctx, 1)
	h.latest(t, ctx, "the first value", func(map[string]Item) bool { return true })
	at := time.Now()
	song := cast.Playing{App: "YouTube Music", At: at, Media: cast.Media{Title: "Song", State: cast.PlayerPlaying, Position: 10, Duration: 200}}
	h.follow.report(t, speaker, song)
	h.latest(t, ctx, "the song", func(v map[string]Item) bool { return len(v) == 1 })
	for i := 1; i <= 5; i++ {
		next := song
		next.At, next.Position = at.Add(time.Duration(i)*10*time.Second), 10+float64(i)*10+1.5
		h.follow.report(t, speaker, next)
	}
	select {
	case v := <-h.published:
		t.Fatalf("published %v for a track that plays on as expected", v)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWatcherKeepsOnlyTheCoversThatPlay(t *testing.T) {
	ctx := testContext(t)
	h := newHarness(t)
	h.watcher.Configure(ctx, Want{IDs: []string{speaker}})
	h.started(t, ctx, 1)
	cover := "https://example.com/cover.jpg"
	h.follow.report(t, speaker, cast.Playing{App: "Plex", At: time.Now(), Media: cast.Media{Title: "Film", Image: cover, State: cast.PlayerPlaying}})
	h.latest(t, ctx, "the film", func(v map[string]Item) bool { return len(v) == 1 })
	h.watcher.Artwork.mu.Lock()
	kept := maps.Clone(h.watcher.Artwork.addresses)
	h.watcher.Artwork.mu.Unlock()
	if kept[artworkID(cover)] != cover {
		t.Fatalf("covers %v, want the film's", kept)
	}
	h.follow.report(t, speaker, cast.Playing{})
	h.latest(t, ctx, "nothing", func(v map[string]Item) bool { return len(v) == 0 })
	if _, err := h.watcher.Artwork.Get(ctx, artworkID(cover)); err == nil {
		t.Fatal("the cover of what stopped is still served")
	}
}
