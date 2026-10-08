// Package datahub is the only part of Daycast that talks to external data
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
	Fetch    func(ctx context.Context) (any, error)
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
}

// New creates a hub. onChange is called after every entry update, from the
// refreshing goroutine.
func New(store *FileStore, logger *slog.Logger, onChange func()) *Hub {
	return &Hub{store: store, logger: logger, now: time.Now, onChange: onChange, entries: map[string]Entry{}}
}

// Entries returns a copy of all current entries.
func (h *Hub) Entries() map[string]Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return maps.Clone(h.entries)
}

// Run restores the last known values from disk, so a server that restarts
// without internet still has data, and then refreshes every source until ctx
// is cancelled.
func (h *Hub) Run(ctx context.Context, sources []Source) {
	var wg sync.WaitGroup
	for _, src := range sources {
		if entry, ok, err := h.store.Load(src.Key); err != nil {
			h.logger.Warn("could not restore cached data", "source", src.Key, "error", err)
		} else if ok {
			h.set(src.Key, entry)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.loop(ctx, src)
		}()
	}
	wg.Wait()
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
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	value, err := src.Fetch(fetchCtx)
	if err != nil {
		h.markFailing(src.Key)
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		h.markFailing(src.Key)
		return fmt.Errorf("encode %s value: %w", src.Key, err)
	}
	now := h.now()
	entry := Entry{Value: raw, FetchedAt: now, StaleAfter: now.Add(src.Interval + staleMargin)}
	h.set(src.Key, entry)
	if err := h.store.Save(src.Key, entry); err != nil {
		// The fresh value is already in memory; only the restart
		// fallback is lost, so this is a warning, not a failed refresh.
		h.logger.Warn("could not persist data", "source", src.Key, "error", err)
	}
	return nil
}

func (h *Hub) markFailing(key string) {
	h.mu.Lock()
	entry := h.entries[key]
	changed := !entry.Failing
	entry.Failing = true
	h.entries[key] = entry
	h.mu.Unlock()
	if changed {
		h.onChange()
	}
}

func (h *Hub) set(key string, entry Entry) {
	h.mu.Lock()
	h.entries[key] = entry
	h.mu.Unlock()
	h.onChange()
}
