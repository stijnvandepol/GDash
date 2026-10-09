// Package datahub is the only part of GDash that talks to external data
// services. It refreshes each source on a schedule, keeps the last good value
// when a refresh fails, and tells displays when data is stale. However many
// TVs are on, every source is fetched once.
package datahub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"runtime/debug"
	"sync"
	"time"
)

const (
	firstRetryDelay = 30 * time.Second
	fetchTimeout    = 20 * time.Second
	// Data counts as fresh until one refresh has been missed, plus a margin
	// for a slow upstream.
	staleMargin = time.Minute
)

// Source is one external data feed.
type Source struct {
	Key      string
	Interval time.Duration
	// Version identifies the source's configuration, such as a hash of its
	// addresses. Configure restarts a source only when it changes.
	Version string
	// Timeout bounds one refresh; zero means fetchTimeout. A source that
	// downloads several calendars needs more than one that asks a single
	// weather service.
	Timeout time.Duration
	Fetch   func(ctx context.Context) (any, error)
}

// Entry is the latest known value of a source. Displays show Value and use
// StaleAfter to warn that the value may be outdated.
type Entry struct {
	Value      json.RawMessage `json:"value,omitempty"`
	FetchedAt  time.Time       `json:"fetchedAt"`
	StaleAfter time.Time       `json:"staleAfter"`
	// Failing is true while the last refresh failed. The error itself stays
	// in the server log, because it can contain URLs and internal details.
	Failing bool `json:"failing"`
}

// Hub refreshes sources and holds their latest entries.
type Hub struct {
	store    *FileStore
	logger   *slog.Logger
	now      func() time.Time
	onChange func()

	mu      sync.Mutex
	entries map[string]Entry
	running map[string]runningSource
}

// runningSource is a source whose loop is running.
type runningSource struct {
	version string
	cancel  context.CancelFunc
}

// New creates a hub. onChange is called after every entry update, from the
// refreshing goroutine.
func New(store *FileStore, logger *slog.Logger, onChange func()) *Hub {
	return &Hub{store: store, logger: logger, now: time.Now, onChange: onChange, entries: map[string]Entry{}, running: map[string]runningSource{}}
}

// Entries returns a copy of all current entries.
func (h *Hub) Entries() map[string]Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return maps.Clone(h.entries)
}

// Configure makes sources the set the hub refreshes, until ctx ends. A
// source with the same key and version keeps running; a changed one
// restarts; a removed one stops and its entry is dropped, so displays stop
// showing data that is no longer configured. A new source restores its last
// known value from disk, so a server that restarts without internet still
// has data, or else gets an empty entry: displays can then tell "loading"
// (an entry without a value) from "not set up" (no entry).
func (h *Hub) Configure(ctx context.Context, sources []Source) {
	wanted := make(map[string]bool, len(sources))
	for _, src := range sources {
		wanted[src.Key] = true
	}
	h.mu.Lock()
	changed := false
	for key, running := range h.running {
		if !wanted[key] {
			running.cancel()
			delete(h.running, key)
			delete(h.entries, key)
			changed = true
			if err := h.store.Delete(key); err != nil {
				h.logger.Warn("could not delete cached data", "source", key, "error", err)
			}
		}
	}
	for _, src := range sources {
		running, ok := h.running[src.Key]
		if ok && running.version == src.Version {
			continue
		}
		if ok {
			// The configuration changed: what was fetched before may
			// include a calendar or feed that is gone now, so displays
			// show "loading" until the new fetch arrives.
			running.cancel()
			h.entries[src.Key] = Entry{}
			changed = true
			if err := h.store.Delete(src.Key); err != nil {
				h.logger.Warn("could not delete cached data", "source", src.Key, "error", err)
			}
		} else if _, inMemory := h.entries[src.Key]; !inMemory {
			h.entries[src.Key] = h.restore(src)
			changed = true
		}
		loopCtx, cancel := context.WithCancel(ctx)
		h.running[src.Key] = runningSource{version: src.Version, cancel: cancel}
		go h.loop(loopCtx, src)
	}
	h.mu.Unlock()
	if changed {
		h.onChange()
	}
}

// restore returns a source's last known value from disk if it was fetched
// with the same configuration, or an empty entry.
func (h *Hub) restore(src Source) Entry {
	entry, _, err := h.store.Load(src.Key, src.Version)
	if err != nil {
		h.logger.Warn("could not restore cached data", "source", src.Key, "error", err)
		return Entry{}
	}
	return entry
}

func (h *Hub) loop(ctx context.Context, src Source) {
	retryDelay := firstRetryDelay
	for {
		wait := src.Interval
		if err := h.refresh(ctx, src); err != nil {
			if ctx.Err() != nil {
				return
			}
			h.logger.Warn("refresh failed, keeping last known value", "source", src.Key, "retry_in", retryDelay, "error", err)
			wait = min(retryDelay, src.Interval)
			retryDelay = min(retryDelay*2, src.Interval)
		} else {
			retryDelay = firstRetryDelay
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// refresh fetches one source once and records the outcome.
func (h *Hub) refresh(ctx context.Context, src Source) error {
	timeout := src.Timeout
	if timeout == 0 {
		timeout = fetchTimeout
	}
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	value, err := safeFetch(fetchCtx, src)
	if err != nil {
		h.markFailing(ctx, src.Key)
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		h.markFailing(ctx, src.Key)
		return fmt.Errorf("encode %s value: %w", src.Key, err)
	}
	now := h.now()
	entry := Entry{Value: raw, FetchedAt: now, StaleAfter: now.Add(src.Interval + staleMargin)}
	h.set(ctx, src, entry)
	return nil
}

// safeFetch runs the source's fetch and turns a panic into an error. The
// sources parse documents from outside, such as calendars and feeds; a bug
// in a parser must fail that one refresh, not end the whole server.
func safeFetch(ctx context.Context, src Source) (value any, err error) {
	defer func() {
		if r := recover(); r != nil {
			value, err = nil, fmt.Errorf("refresh of %s panicked: %v\n%s", src.Key, r, debug.Stack())
		}
	}()
	return src.Fetch(ctx)
}

func (h *Hub) markFailing(ctx context.Context, key string) {
	h.mu.Lock()
	// A stopped loop must not touch an entry its replacement owns.
	if ctx.Err() != nil {
		h.mu.Unlock()
		return
	}
	entry := h.entries[key]
	changed := !entry.Failing
	entry.Failing = true
	h.entries[key] = entry
	h.mu.Unlock()
	if changed {
		h.onChange()
	}
}

// set stores a fresh entry in memory and on disk. Configure cancels a loop
// while holding h.mu, so checking ctx under the same lock guarantees a
// stopped loop never overwrites the entry of its replacement, nor writes back
// a cache file that Configure just deleted.
func (h *Hub) set(ctx context.Context, src Source, entry Entry) {
	h.mu.Lock()
	if ctx.Err() != nil {
		h.mu.Unlock()
		return
	}
	h.entries[src.Key] = entry
	if err := h.store.Save(src.Key, src.Version, entry); err != nil {
		// The fresh value is already in memory; only the restart
		// fallback is lost, so this is a warning, not a failed refresh.
		h.logger.Warn("could not persist data", "source", src.Key, "error", err)
	}
	h.mu.Unlock()
	h.onChange()
}
