package cast

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// stateRecorder collects the states a keeper reports.
type stateRecorder struct {
	mu     sync.Mutex
	states []State
	err    error
}

func (r *stateRecorder) record(s State, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = append(r.states, s)
	r.err = err
}

func (r *stateRecorder) waitFor(t *testing.T, want State) error {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		n := len(r.states)
		last, err := State(""), r.err
		if n > 0 {
			last = r.states[n-1]
		}
		r.mu.Unlock()
		if last == want {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("states = %v, want it to reach %s", r.states, want)
	return nil
}

func TestKeeperReportsState(t *testing.T) {
	otherInput := false
	tests := []struct {
		name    string
		apps    []Application
		standby bool
		input   *bool
		active  bool
		want    State
	}{
		{"idle device gets the dashboard", []Application{{AppID: "E8C28D3C", IsIdleScreen: true}}, false, nil, true, StateShowing},
		{"TV in standby", nil, true, nil, true, StateStandby},
		{"someone casts YouTube when GDash starts", []Application{{AppID: "233637DE", DisplayName: "YouTube", SessionID: "yt", TransportID: "yt"}}, false, nil, true, StateYielded},
		{"TV on another input", nil, false, &otherInput, true, StateOtherApp},
		{"outside active hours", nil, false, nil, false, StateResting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()
			device := newFakeDevice(t, tt.apps...)
			device.standby = tt.standby
			device.activeInput = tt.input
			rec := &stateRecorder{}
			keeper := newRefreshingKeeper(device, 0)
			keeper.Active = func() bool { return tt.active }
			keeper.Status = rec.record
			go keeper.Run(ctx)
			rec.waitFor(t, tt.want)
		})
	}
}

func TestKeeperReportsUnreachableDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	rec := &stateRecorder{}
	keeper := newRefreshingKeeper(nil, 0)
	keeper.Resolve = func(context.Context) (Device, error) { return Device{}, errors.New("device not found on the network") }
	keeper.Status = rec.record
	go keeper.Run(ctx)
	if err := rec.waitFor(t, StateUnreachable); err == nil || err.Error() != "device not found on the network" {
		t.Fatalf("unreachable error = %v", err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.states[0] != StateConnecting {
		t.Fatalf("first state = %s, want connecting", rec.states[0])
	}
}
