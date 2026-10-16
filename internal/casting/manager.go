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
	// RunDisplay keeps one display on its Cast device until ctx ends; see
	// Keep for what it reports and hears.
	RunDisplay func(ctx context.Context, d displays.Display, keep Keep)
	// Now is the clock for Status.Since; nil means time.Now.
	Now    func() time.Time
	Logger *slog.Logger

	triggerOnce sync.Once
	trigger     chan struct{}

	mu        sync.Mutex
	statuses  map[string]Status
	takeOvers map[string]chan struct{}
	hiddens   map[string]chan struct{}
	showns    map[string]chan struct{}
}

// Keep connects one display's keeper to the manager.
type Keep struct {
	// Report receives every state the keeper finds.
	Report func(cast.State, error)
	// Restarted is called after every restart of a running receiver, with
	// cast.RestartPeriodic.
	Restarted func(reason string)
	// TakeOver delivers an admin's request to show the dashboard on the TV
	// now.
	TakeOver <-chan struct{}
	// Hidden delivers a report of the page on the TV that it is hidden, and
	// Shown one that it is on screen again.
	Hidden <-chan struct{}
	Shown  <-chan struct{}
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
	// RestartedAt is when the keeper last restarted the receiver to keep it
	// in front of the screensaver, and RestartReason why: cast.RestartPeriodic.
	// Absent until it first did.
	RestartedAt   *time.Time `json:"restartedAt,omitempty"`
	RestartReason string     `json:"restartReason,omitempty"`
	// HiddenAt is when the page on the TV last reported that it is hidden,
	// such as by the TV's screensaver.
	HiddenAt *time.Time `json:"hiddenAt,omitempty"`
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
		current, ok := m.statuses[displayID]
		if ok && current.State == state && current.Problem == problem {
			return
		}
		if m.statuses == nil {
			m.statuses = map[string]Status{}
		}
		m.statuses[displayID] = Status{
			State: state, Since: m.now(), Problem: problem, Reason: reasonFor(state, err),
			RestartedAt: current.RestartedAt, RestartReason: current.RestartReason, HiddenAt: current.HiddenAt,
		}
	}
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// restarted records a restart of one display's receiver.
func (m *Manager) restarted(displayID string) func(reason string) {
	return func(reason string) {
		m.mu.Lock()
		defer m.mu.Unlock()
		status, ok := m.statuses[displayID]
		if !ok {
			return
		}
		at := m.now()
		status.RestartedAt, status.RestartReason = &at, reason
		m.statuses[displayID] = status
	}
}

// ScreenHidden passes a report of the page on a display's TV that it is
// hidden to the display's keeper, which then leaves the TV to whoever
// covered the dashboard (see cast.Keeper.Hidden). It reports false when the
// display has no running keeper. It never blocks.
func (m *Manager) ScreenHidden(displayID string) bool {
	m.mu.Lock()
	hidden, ok := m.hiddens[displayID]
	if status, known := m.statuses[displayID]; ok && known {
		at := m.now()
		status.HiddenAt = &at
		m.statuses[displayID] = status
	}
	m.mu.Unlock()
	return ok && signal(hidden)
}

// ScreenShown passes a report of the page on a display's TV that it is on
// screen again to the display's keeper (see cast.Keeper.Shown). It reports
// false when the display has no running keeper. It never blocks.
func (m *Manager) ScreenShown(displayID string) bool {
	m.mu.Lock()
	shown, ok := m.showns[displayID]
	m.mu.Unlock()
	return ok && signal(shown)
}

// signal sends on a channel with room for one, without blocking: a signal
// that is still waiting covers a second one. It always reports true.
func signal(ch chan struct{}) bool {
	select {
	case ch <- struct{}{}:
	default:
	}
	return true
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

// track makes a display's keeper reachable for TakeOver, ScreenHidden and
// ScreenShown.
func (m *Manager) track(displayID string) Keep {
	takeOver, hidden, shown := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.takeOvers == nil {
		m.takeOvers, m.hiddens, m.showns = map[string]chan struct{}{}, map[string]chan struct{}{}, map[string]chan struct{}{}
	}
	m.takeOvers[displayID], m.hiddens[displayID], m.showns[displayID] = takeOver, hidden, shown
	return Keep{Report: m.reporter(displayID), Restarted: m.restarted(displayID), TakeOver: takeOver, Hidden: hidden, Shown: shown}
}

func (m *Manager) forget(displayID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.statuses, displayID)
	delete(m.takeOvers, displayID)
	delete(m.hiddens, displayID)
	delete(m.showns, displayID)
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
		keep := m.track(d.ID)
		go func() {
			defer close(done)
			m.RunDisplay(keeperCtx, d, keep)
		}()
		active[id] = running{castDeviceID: d.CastDeviceID, cancel: cancel, done: done}
	}
}
