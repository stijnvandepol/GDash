package datahub

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 8, 7, 30, 0, 0, time.UTC)

func newTestHub(t *testing.T) (*Hub, *FileStore, *int) {
	t.Helper()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	changes := 0
	h := New(store, slog.New(slog.NewTextHandler(io.Discard, nil)), func() { changes++ })
	h.now = func() time.Time { return fixedNow }
	return h, store, &changes
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
	if *changes != 1 {
		t.Fatalf("onChange called %d times, want 1", *changes)
	}
	if _, ok, err := store.Load("weather.current"); err != nil || !ok {
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
	if *changes != 2 {
		t.Fatalf("onChange called %d times, want 2 (success, first failure)", *changes)
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

func TestRunRestoresPersistedValueBeforeFirstFetch(t *testing.T) {
	h, store, _ := newTestHub(t)
	saved := Entry{Value: []byte(`12.5`), FetchedAt: fixedNow.Add(-time.Hour), StaleAfter: fixedNow.Add(-44 * time.Minute)}
	if err := store.Save("weather.current", saved); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fetched := make(chan struct{})
	src := source(func(context.Context) (any, error) {
		close(fetched)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan struct{})
	go func() { h.Run(ctx, []Source{src}); close(done) }()

	<-fetched
	if got := h.Entries()["weather.current"]; string(got.Value) != "12.5" {
		t.Fatalf("entry during first fetch = %+v, want restored value", got)
	}
	cancel()
	<-done
}

func TestFileStoreRejectsUnsafeKey(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("../escape", Entry{}); err == nil {
		t.Fatal("Save accepted a path traversal key")
	}
	if _, ok, err := store.Load("missing.key"); err != nil || ok {
		t.Fatalf("Load of missing key = ok %v, err %v", ok, err)
	}
}
