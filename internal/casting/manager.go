// Package casting runs one cast keeper per display that is linked to a Cast
// device, and starts or stops keepers when displays change.
package casting

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/displays"
)

// reconcileInterval catches changes made outside the API, such as an
// imported configuration. API changes trigger a reconcile at once.
const reconcileInterval = time.Minute

// Manager keeps the set of running keepers equal to the displays that have
// a Cast device, and remembers what each keeper last reported.
type Manager struct {
	List func(ctx context.Context) ([]displays.Display, error)
	// RunDisplay keeps one display on its Cast device until ctx ends,
	// passing every state it finds to report. Each value on takeOver is an
	// admin's request to show the dashboard on the TV now.
	RunDisplay func(ctx context.Context, d displays.Display, report func(cast.State, error), takeOver <-chan struct{})
	// Now is the clock for Status.Since; nil means time.Now.
	Now    func() time.Time
	Logger *slog.Logger

	triggerOnce sync.Once
	trigger     chan struct{}

	mu        sync.Mutex
	statuses  map[string]Status
	takeOvers map[string]chan struct{}
}

// Status is what the keeper of a display last reported.
type Status struct {
	State cast.State `json:"state"`
	// Since is when the display entered this state.
	Since time.Time `json:"since"`
	// Problem explains an unreachable device, in the words of the error.
	Problem string `json:"problem,omitempty"`
	// Reason sorts an unreachable device's problem for the web interface:
	// ReasonNotFound or ReasonNoAnswer. Empty in the other states.
	Reason string `json:"reason,omitempty"`
}

// Reasons a device is unreachable.
const (
	// ReasonNotFound: discovery did not find the device; it is off, asleep
	// without network standby, or on another network.
	ReasonNotFound = "not_found"
	// ReasonNoAnswer: everything else, such as a device that was found but
	// refused or dropped the connection.
	ReasonNoAnswer = "no_answer"
)

// reasonFor sorts the error of an unreachable device.
func reasonFor(state cast.State, err error) string {
	switch {
	case state != cast.StateUnreachable:
		return ""
	case errors.Is(err, cast.ErrDeviceNotFound):
		return ReasonNotFound
	default:
		return ReasonNoAnswer
	}
}

// Statuses returns the latest status by display ID, for displays with a
// running keeper.
func (m *Manager) Statuses() map[string]Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.statuses)
}

// reporter records the states of one display's keeper. Since only moves
// when the state or its problem changes, not on every status poll.
func (m *Manager) reporter(displayID string) func(cast.State, error) {
	return func(state cast.State, err error) {
		problem := ""
		if err != nil {
			problem = err.Error()
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if current, ok := m.statuses[displayID]; ok && current.State == state && current.Problem == problem {
			return
		}
		now := time.Now
		if m.Now != nil {
			now = m.Now
		}
		if m.statuses == nil {
			m.statuses = map[string]Status{}
		}
		m.statuses[displayID] = Status{State: state, Since: now(), Problem: problem, Reason: reasonFor(state, err)}
	}
}

// TakeOver asks the keeper of a display to show the dashboard on its TV now,
// also over what someone else is showing. It reports false when the display
// has no running keeper. It never blocks: a request that is still waiting
// covers a second one.
func (m *Manager) TakeOver(displayID string) bool {
	m.mu.Lock()
	request, ok := m.takeOvers[displayID]
	m.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case request <- struct{}{}:
	default:
	}
	return true
}

// track makes a display's keeper reachable for TakeOver.
func (m *Manager) track(displayID string) <-chan struct{} {
	request := make(chan struct{}, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.takeOvers == nil {
		m.takeOvers = map[string]chan struct{}{}
	}
	m.takeOvers[displayID] = request
	return request
}

func (m *Manager) forget(displayID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.statuses, displayID)
	delete(m.takeOvers, displayID)
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
			m.forget(id)
		}
	}
	for id, d := range wanted {
		if _, ok := active[id]; ok {
			continue
		}
		keeperCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		takeOver := m.track(d.ID)
		go func() {
			defer close(done)
			m.RunDisplay(keeperCtx, d, m.reporter(d.ID), takeOver)
		}()
		active[id] = running{castDeviceID: d.CastDeviceID, cancel: cancel, done: done}
	}
}
