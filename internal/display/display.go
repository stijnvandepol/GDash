// Package display builds the state that displays render and streams it to
// them. Browser displays receive it over Server-Sent Events and Cast
// displays over the Cast message channel, in the same message format.
package display

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/stijnvandepol/daycast/internal/datahub"
)

// CastNamespace is the Cast message namespace between the Daycast server
// and its receiver. web/src/protocol.ts uses the same value.
const CastNamespace = "urn:x-cast:app.daycast.display"

// Widget is one tile on the dashboard grid. Positions are in grid units.
type Widget struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	X        int            `json:"x"`
	Y        int            `json:"y"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	Source   string         `json:"source,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

// Dashboard is a layout on a fixed 16:9 grid.
type Dashboard struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Theme   string   `json:"theme"`
	Columns int      `json:"columns"`
	Rows    int      `json:"rows"`
	Widgets []Widget `json:"widgets"`
}

// DefaultDashboard is the only dashboard in the prototype. The editor in
// v0.5 replaces it with dashboards stored in SQLite.
func DefaultDashboard() Dashboard {
	return Dashboard{
		ID:      "default",
		Name:    "Standaard",
		Theme:   "minimal-dark",
		Columns: 24,
		Rows:    14,
		Widgets: []Widget{
			{ID: "clock", Type: "clock", X: 0, Y: 0, W: 14, H: 14},
			{ID: "weather", Type: "weather", X: 14, Y: 0, W: 10, H: 14, Source: "weather.current"},
		},
	}
}

// Snapshot is the complete state of a display. Version increases with every
// change, so a display can ignore a snapshot it has already rendered.
type Snapshot struct {
	Type       string                   `json:"type"`
	Version    int64                    `json:"version"`
	ServerTime time.Time                `json:"serverTime"`
	Dashboard  Dashboard                `json:"dashboard"`
	Data       map[string]datahub.Entry `json:"data"`
}

// Broadcaster holds the latest encoded snapshot and hands it to every
// subscriber. A slow subscriber only ever gets the newest snapshot, never a
// backlog.
type Broadcaster struct {
	mu          sync.Mutex
	version     int64
	latest      []byte
	subscribers map[chan []byte]struct{}
}

// NewBroadcaster creates an empty broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subscribers: map[chan []byte]struct{}{}}
}

// Publish encodes a new snapshot of dashboard and data and sends it to all
// subscribers.
func (b *Broadcaster) Publish(now time.Time, dashboard Dashboard, data map[string]datahub.Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.version++
	msg, err := json.Marshal(Snapshot{Type: "snapshot", Version: b.version, ServerTime: now, Dashboard: dashboard, Data: data})
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	b.latest = msg
	for ch := range b.subscribers {
		replaceLatest(ch, msg)
	}
	return nil
}

// Subscribe returns a channel that first receives the current snapshot (if
// one exists) and then every new one. It is closed when ctx ends.
func (b *Broadcaster) Subscribe(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, 1)
	b.mu.Lock()
	if b.latest != nil {
		ch <- b.latest
	}
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
		close(ch)
	}()
	return ch
}

func replaceLatest(ch chan []byte, msg []byte) {
	select {
	case <-ch:
	default:
	}
	ch <- msg
}
