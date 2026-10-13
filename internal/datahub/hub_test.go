package datahub

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 8, 7, 30, 0, 0, time.UTC)

func newTestHub(t *testing.T) (*Hub, *FileStore, *atomic.Int32) {
	t.Helper()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// onChange runs on the hub's goroutines, so the counter must be atomic.
	changes := &atomic.Int32{}
	h := New(store, slog.New(slog.NewTextHandler(io.Discard, nil)), func() { changes.Add(1) })
	h.now = func() time.Time { return fixedNow }
	return h, store, changes
}

func source(fetch func(context.Context) (any, error)) Source {
	return Source{Key: "weather.current", Interval: 15 * time.Minute, Fetch: fetch}
}

func TestRefreshStoresValueWithStaleDeadline(t *testing.T) {
	h, store, changes := newTestHub(t)
	src := source(func(context.Context) (any, error) { return map[string]float64{"t": 14.3}, nil })

	if err := h.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	got := h.Entries()["weather.current"]
	if string(got.Value) != `{"t":14.3}` || got.Failing {
		t.Fatalf("entry = %+v", got)
	}
	if want := fixedNow.Add(16 * time.Minute); !got.StaleAfter.Equal(want) {
		t.Fatalf("StaleAfter = %v, want %v", got.StaleAfter, want)
	}
	if n := changes.Load(); n != 1 {
		t.Fatalf("onChange called %d times, want 1", n)
	}
	if _, ok, err := store.Load("weather.current", ""); err != nil || !ok {
		t.Fatalf("value not persisted: ok=%v err=%v", ok, err)
	}
}

func TestFailedRefreshKeepsLastKnownValue(t *testing.T) {
	h, _, changes := newTestHub(t)
	ok := source(func(context.Context) (any, error) { return 14.3, nil })
	failing := source(func(context.Context) (any, error) { return nil, errors.New("network down") })

	if err := h.refresh(context.Background(), ok); err != nil {
		t.Fatal(err)
	}
	if err := h.refresh(context.Background(), failing); err == nil {
		t.Fatal("refresh hid the fetch error")
	}
	got := h.Entries()["weather.current"]
	if string(got.Value) != "14.3" || !got.Failing {
		t.Fatalf("entry after failure = %+v, want old value marked failing", got)
	}
	if err := h.refresh(context.Background(), failing); err == nil {
		t.Fatal("refresh hid the fetch error")
	}
	if n := changes.Load(); n != 2 {
		t.Fatalf("onChange called %d times, want 2 (success, first failure)", n)
	}
}

func TestPanickingFetchFailsOnlyThatRefresh(t *testing.T) {
	h, _, _ := newTestHub(t)
	ok := source(func(context.Context) (any, error) { return 14.3, nil })
	broken := source(func(context.Context) (any, error) { panic("parser bug") })

	if err := h.refresh(context.Background(), ok); err != nil {
		t.Fatal(err)
	}
	err := h.refresh(context.Background(), broken)
	if err == nil || !strings.Contains(err.Error(), "parser bug") {
		t.Fatalf("refresh after a panic = %v, want an error naming it", err)
	}
	if got := h.Entries()["weather.current"]; string(got.Value) != "14.3" || !got.Failing {
		t.Fatalf("entry after a panic = %+v, want the old value marked failing", got)
	}
}

func TestRecoveryClearsFailing(t *testing.T) {
	h, _, _ := newTestHub(t)
	failing := source(func(context.Context) (any, error) { return nil, errors.New("timeout") })
	ok := source(func(context.Context) (any, error) { return 15, nil })

	_ = h.refresh(context.Background(), failing)
	if err := h.refresh(context.Background(), ok); err != nil {
		t.Fatal(err)
	}
	if got := h.Entries()["weather.current"]; got.Failing || string(got.Value) != "15" {
		t.Fatalf("entry after recovery = %+v", got)
	}
}

func TestConfigureRestoresPersistedValueBeforeFirstFetch(t *testing.T) {
	h, store, _ := newTestHub(t)
	saved := Entry{Value: []byte(`12.5`), FetchedAt: fixedNow.Add(-time.Hour), StaleAfter: fixedNow.Add(-44 * time.Minute)}
	if err := store.Save("weather.current", "", saved); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fetched := make(chan struct{})
	src := source(func(context.Context) (any, error) {
		close(fetched)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	h.Configure(ctx, []Source{src})

	<-fetched
	if got := h.Entries()["weather.current"]; string(got.Value) != "12.5" {
		t.Fatalf("entry during first fetch = %+v, want restored value", got)
	}
	cancel()
}

// counting returns a source that counts its fetches and returns value.
func counting(key, version string, value any, fetches *atomic.Int32) Source {
	return Source{Key: key, Version: version, Interval: time.Hour, Fetch: func(context.Context) (any, error) {
		fetches.Add(1)
		return value, nil
	}}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestConfigureRestartsOnlyChangedSources(t *testing.T) {
	h, _, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var weatherFetches, newsFetches atomic.Int32
	h.Configure(ctx, []Source{
		counting("weather.current", "oss", 14, &weatherFetches),
		counting("news.default", "nos", "v1", &newsFetches),
	})
	waitFor(t, "first fetches", func() bool { return weatherFetches.Load() == 1 && newsFetches.Load() == 1 })

	// Same weather version, new news version: only news fetches again.
	h.Configure(ctx, []Source{
		counting("weather.current", "oss", 14, &weatherFetches),
		counting("news.default", "nos+tweakers", "v2", &newsFetches),
	})
	waitFor(t, "news refetch", func() bool { return string(h.Entries()["news.default"].Value) == `"v2"` })
	if n := weatherFetches.Load(); n != 1 {
		t.Fatalf("weather fetched %d times, want once: its configuration did not change", n)
	}
}

func TestConfigureDropsRemovedSources(t *testing.T) {
	h, store, changes := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fetches atomic.Int32
	h.Configure(ctx, []Source{counting("calendar.default", "a", []string{"dentist"}, &fetches)})
	waitFor(t, "calendar entry", func() bool { _, ok := h.Entries()["calendar.default"]; return ok })
	before := changes.Load()

	h.Configure(ctx, nil)
	if _, ok := h.Entries()["calendar.default"]; ok {
		t.Fatal("a removed source still has an entry")
	}
	if _, ok, _ := store.Load("calendar.default", "a"); ok {
		t.Fatal("a removed source is still cached on disk")
	}
	if changes.Load() == before {
		t.Fatal("removing a source did not notify the displays")
	}
}

func TestConfigureAddsEmptyEntryUntilFirstFetch(t *testing.T) {
	h, _, changes := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	h.Configure(ctx, []Source{{Key: "weather.current", Interval: time.Hour, Fetch: func(context.Context) (any, error) {
		<-release
		return 14, nil
	}}})
	got, ok := h.Entries()["weather.current"]
	if !ok || got.Value != nil || changes.Load() != 1 {
		t.Fatalf("entry before the first fetch = %+v (present %v, changes %d); want an empty entry and a notification", got, ok, changes.Load())
	}
	close(release)
	waitFor(t, "first value", func() bool { return string(h.Entries()["weather.current"].Value) == "14" })
}

func TestStoppedLoopCannotOverwriteEntry(t *testing.T) {
	h, _, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := source(func(context.Context) (any, error) { return "late result", nil })
	if err := h.refresh(ctx, src); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Entries()["weather.current"]; ok {
		t.Fatal("a refresh from a stopped loop was stored")
	}
}

func TestFileStoreRejectsUnsafeKey(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("../escape", "", Entry{}); err == nil {
		t.Fatal("Save accepted a path traversal key")
	}
	if _, ok, err := store.Load("missing.key", ""); err != nil || ok {
		t.Fatalf("Load of missing key = ok %v, err %v", ok, err)
	}
}

func TestChangedSourceDropsItsOldValue(t *testing.T) {
	h, store, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fetches atomic.Int32
	h.Configure(ctx, []Source{counting("calendar.default", "family+work", []string{"work meeting"}, &fetches)})
	waitFor(t, "first fetch", func() bool { return string(h.Entries()["calendar.default"].Value) != "" })

	// The work calendar is removed while the remaining one cannot be
	// fetched: its events must not stay on screen.
	h.Configure(ctx, []Source{{Key: "calendar.default", Version: "family", Interval: time.Hour, Fetch: func(ctx context.Context) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}})
	if got := h.Entries()["calendar.default"]; got.Value != nil {
		t.Fatalf("entry after the change = %s, want no value until the new fetch", got.Value)
	}
	if _, ok, _ := store.Load("calendar.default", "family+work"); ok {
		t.Fatal("the cache of the old configuration is still on disk")
	}
}

func TestRestoreIgnoresCacheOfAnotherConfiguration(t *testing.T) {
	h, store, _ := newTestHub(t)
	if err := store.Save("weather.current", "oss", Entry{Value: []byte(`14`)}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	block := func(ctx context.Context) (any, error) { <-ctx.Done(); return nil, ctx.Err() }
	h.Configure(ctx, []Source{{Key: "weather.current", Version: "uden", Interval: time.Hour, Fetch: block}})
	if got := h.Entries()["weather.current"]; got.Value != nil {
		t.Fatalf("restored %s from another location", got.Value)
	}
}

func TestStoppedLoopDoesNotWriteTheCache(t *testing.T) {
	h, store, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := source(func(context.Context) (any, error) { return "late", nil })
	if err := h.refresh(ctx, src); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.Load(src.Key, src.Version); ok {
		t.Fatal("a stopped loop wrote its result to disk")
	}
}

func pushed(interval time.Duration) Source {
	return Source{Key: "custom.source.1", Version: "push", Interval: interval, FreshFor: 2*interval + 30*time.Second}
}

func TestPutStoresPushedValue(t *testing.T) {
	h, store, changes := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.Configure(ctx, []Source{pushed(time.Minute)})
	if got, ok := h.Entries()["custom.source.1"]; !ok || len(got.Value) != 0 {
		t.Fatalf("entry before the first push = %+v, %v; want an empty one", got, ok)
	}
	before := changes.Load()
	if err := h.Put("custom.source.1", []byte(`{"on":true}`)); err != nil {
		t.Fatal(err)
	}
	got := h.Entries()["custom.source.1"]
	if string(got.Value) != `{"on":true}` || got.Failing || !got.FetchedAt.Equal(fixedNow) {
		t.Fatalf("entry = %+v", got)
	}
	if want := fixedNow.Add(150 * time.Second); !got.StaleAfter.Equal(want) {
		t.Fatalf("StaleAfter = %v, want %v (2 intervals and 30 seconds)", got.StaleAfter, want)
	}
	if changes.Load() != before+1 {
		t.Fatal("a push did not report a change")
	}
	if saved, ok, err := store.Load("custom.source.1", "push"); err != nil || !ok || string(saved.Value) != `{"on":true}` {
		t.Fatalf("pushed value on disk = %+v, %v, %v", saved, ok, err)
	}

	// A new interval with the same version keeps the value and applies
	// to the next push.
	h.Configure(ctx, []Source{pushed(5 * time.Minute)})
	if got := h.Entries()["custom.source.1"]; string(got.Value) != `{"on":true}` {
		t.Fatalf("entry after a new interval = %+v, want the pushed value kept", got)
	}
	if err := h.Put("custom.source.1", []byte(`1`)); err != nil {
		t.Fatal(err)
	}
	if got := h.Entries()["custom.source.1"]; !got.StaleAfter.Equal(fixedNow.Add(630 * time.Second)) {
		t.Fatalf("StaleAfter after a new interval = %v", got.StaleAfter)
	}
}

func TestPutRefusesFetchedAndUnknownSources(t *testing.T) {
	h, _, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fetches atomic.Int32
	h.Configure(ctx, []Source{counting("custom.source.2", "a", 1, &fetches)})
	for _, key := range []string{"custom.source.2", "custom.source.3"} {
		if err := h.Put(key, []byte(`1`)); !errors.Is(err, ErrNotPushed) {
			t.Errorf("Put(%s) = %v, want ErrNotPushed", key, err)
		}
	}
}

func TestPushedValueSurvivesARestart(t *testing.T) {
	h, store, _ := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.Configure(ctx, []Source{pushed(time.Minute)})
	if err := h.Put("custom.source.1", []byte(`42`)); err != nil {
		t.Fatal(err)
	}
	restarted := New(store, h.logger, func() {})
	restarted.Configure(ctx, []Source{pushed(time.Minute)})
	if got := restarted.Entries()["custom.source.1"]; string(got.Value) != "42" || !got.FetchedAt.Equal(fixedNow) {
		t.Fatalf("entry after a restart = %+v, want the last pushed value", got)
	}
}
