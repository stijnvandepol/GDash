package traffic

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	// MinInterval is the shortest time between two requests to the ANWB,
	// whatever asks: the website itself refreshes every few minutes.
	MinInterval = 3 * time.Minute
	// maxBackoff is the longest wait after failures in a row.
	maxBackoff = 30 * time.Minute
	// stopAfterRefusals is how many refusals in a row (see
	// StatusError.Refused) make GDash stop asking until the admin turns
	// traffic info off and on again.
	stopAfterRefusals = 3
)

var (
	// ErrStopped is returned after the ANWB refused several requests in a
	// row; GDash no longer asks.
	ErrStopped = errors.New("anwb traffic info stopped after repeated refusals; turn it off and on under Settings to try again")
	// ErrWaiting is returned while GDash waits before asking again after a
	// failure.
	ErrWaiting = errors.New("waiting before asking the anwb again")
)

// The states of a Guard, for the Settings page.
const (
	// StateIdle is before the first request.
	StateIdle    = "idle"
	StateOK      = "ok"
	StateFailing = "failing"
	StateStopped = "stopped"
)

// Status is how the requests to the ANWB go.
type Status struct {
	State string `json:"state"`
	// HTTPStatus is the code of the last refusal, while failing or stopped.
	HTTPStatus int `json:"httpStatus,omitempty"`
	// NextAttemptAt is when GDash asks again after a failure.
	NextAttemptAt *time.Time `json:"nextAttemptAt,omitempty"`
}

// Guard puts the rules for asking the ANWB around a Client: at most one
// request per MinInterval, shared by every caller; a doubling wait after
// failures; and no more requests at all after stopAfterRefusals refusals in
// a row, until Reset.
type Guard struct {
	Client interface {
		Fetch(ctx context.Context) (Report, error)
	}
	Now func() time.Time

	mu          sync.Mutex
	lastAttempt time.Time
	last        *Report
	failures    int
	refusals    int
	lastCode    int
	stopped     bool
}

// Fetch returns the national report. Within MinInterval of a good answer it
// returns that answer again without asking, so a widget that changes does
// not cost a request.
func (g *Guard) Fetch(ctx context.Context) (Report, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped {
		return Report{}, ErrStopped
	}
	now := g.Now()
	if !g.lastAttempt.IsZero() && now.Before(g.lastAttempt.Add(g.wait())) {
		if g.failures == 0 && g.last != nil {
			return *g.last, nil
		}
		return Report{}, ErrWaiting
	}
	g.lastAttempt = now
	report, err := g.Client.Fetch(ctx)
	if err != nil {
		g.failed(err)
		return Report{}, err
	}
	g.failures, g.refusals, g.lastCode = 0, 0, 0
	g.last = &report
	return report, nil
}

func (g *Guard) failed(err error) {
	g.failures++
	var status *StatusError
	if !errors.As(err, &status) {
		g.refusals = 0
		return
	}
	g.lastCode = status.Code
	if !status.Refused() {
		g.refusals = 0
		return
	}
	g.refusals++
	if g.refusals >= stopAfterRefusals {
		g.stopped = true
	}
}

// wait is MinInterval, doubled for every failure in a row, up to maxBackoff.
func (g *Guard) wait() time.Duration {
	wait := MinInterval
	for range g.failures {
		wait *= 2
		if wait >= maxBackoff {
			return maxBackoff
		}
	}
	return wait
}

// Reset forgets failures and a stop, for when the admin turns traffic info
// on again.
func (g *Guard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastAttempt, g.last = time.Time{}, nil
	g.failures, g.refusals, g.lastCode, g.stopped = 0, 0, 0, false
}

// Status reports how the requests go.
func (g *Guard) Status() Status {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case g.stopped:
		return Status{State: StateStopped, HTTPStatus: g.lastCode}
	case g.failures > 0:
		next := g.lastAttempt.Add(g.wait())
		return Status{State: StateFailing, HTTPStatus: g.lastCode, NextAttemptAt: &next}
	case g.last != nil:
		return Status{State: StateOK}
	default:
		return Status{State: StateIdle}
	}
}
