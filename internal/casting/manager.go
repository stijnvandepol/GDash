// Package casting runs one cast keeper per display that is linked to a Cast
// device, and starts or stops keepers when displays change.
package casting

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stijnvandepol/daycast/internal/displays"
)

// reconcileInterval catches changes made outside the API, such as an
// imported configuration. API changes trigger a reconcile at once.
const reconcileInterval = time.Minute

// Manager keeps the set of running keepers equal to the displays that have
// a Cast device.
type Manager struct {
	List func(ctx context.Context) ([]displays.Display, error)
	// RunDisplay keeps one display on its Cast device until ctx ends.
	RunDisplay func(ctx context.Context, d displays.Display)
	Logger     *slog.Logger

	triggerOnce sync.Once
	trigger     chan struct{}
}

type running struct {
	castDeviceID string
	cancel       context.CancelFunc
	done         chan struct{}
}

func (m *Manager) init() {
	m.triggerOnce.Do(func() { m.trigger = make(chan struct{}, 1) })
}

// Changed asks the manager to reconcile soon. It never blocks.
func (m *Manager) Changed() {
	m.init()
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

// Run reconciles until ctx ends, then stops every keeper and waits for them.
func (m *Manager) Run(ctx context.Context) {
	m.init()
	active := map[string]running{}
	defer func() {
		for _, r := range active {
			r.cancel()
			<-r.done
		}
	}()

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		m.reconcile(ctx, active)
		select {
		case <-ctx.Done():
			return
		case <-m.trigger:
		case <-ticker.C:
		}
	}
}

func (m *Manager) reconcile(ctx context.Context, active map[string]running) {
	list, err := m.List(ctx)
	if err != nil {
		// Keep the current keepers; a failed read says nothing about the
		// displays themselves.
		m.Logger.Error("could not read displays", "error", err)
		return
	}
	wanted := map[string]displays.Display{}
	for _, d := range list {
		if d.CastDeviceID != "" {
			wanted[d.ID] = d
		}
	}
	for id, r := range active {
		if d, ok := wanted[id]; !ok || d.CastDeviceID != r.castDeviceID {
			r.cancel()
			<-r.done
			delete(active, id)
		}
	}
	for id, d := range wanted {
		if _, ok := active[id]; ok {
			continue
		}
		keeperCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			m.RunDisplay(keeperCtx, d)
		}()
		active[id] = running{castDeviceID: d.CastDeviceID, cancel: cancel, done: done}
	}
}
