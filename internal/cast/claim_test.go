package cast

import (
	"context"
	"encoding/json"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// quiet is how long a test watches for something that must not happen.
const quiet = 300 * time.Millisecond

func (r *stateRecorder) last() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.states) == 0 {
		return ""
	}
	return r.states[len(r.states)-1]
}

// startKeeper runs keeper with a state recorder until the test ends.
func startKeeper(t *testing.T, keeper *Keeper) *stateRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*testTimeout)
	t.Cleanup(cancel)
	rec := &stateRecorder{}
	keeper.Status = rec.record
	go keeper.Run(ctx)
	return rec
}

// expectLaunches checks that the launch count stays at want while the test
// watches.
func expectLaunches(t *testing.T, device *fakeDevice, want int, why string) {
	t.Helper()
	time.Sleep(quiet)
	if n := device.launchCount(); n != want {
		t.Fatalf("launches = %d, want %d: %s", n, want, why)
	}
}

func TestKeeperYieldsWhenTheDashboardIsClosedOnTheTV(t *testing.T) {
	device := newFakeDevice(t)
	rec := startKeeper(t, newRefreshingKeeper(device, 50*time.Millisecond))
	rec.waitFor(t, StateShowing)
	before := device.launchCount()

	// Back or Home on the remote ends the app; Google TV reports a native
	// app such as Netflix the same way. The device tells the keeper at once.
	device.closeApps()
	rec.waitFor(t, StateYielded)
	expectLaunches(t, device, before, "neither a relaunch nor a periodic restart while someone uses the TV")
}

func TestKeeperYieldsToAnotherApp(t *testing.T) {
	device := newFakeDevice(t)
	rec := startKeeper(t, newRefreshingKeeper(device, 50*time.Millisecond))
	rec.waitFor(t, StateShowing)
	before := device.launchCount()

	device.castFrom(Application{AppID: "233637DE", DisplayName: "YouTube", SessionID: "yt", TransportID: "yt"})
	rec.waitFor(t, StateYielded)
	// The video ends; the next one may follow, so the TV stays theirs.
	device.closeApps()
	expectLaunches(t, device, before, "the TV stays with whoever cast to it until it is switched off")
	if got := rec.last(); got != StateYielded {
		t.Fatalf("state = %s, want yielded", got)
	}
}

func TestKeeperYieldsToAnotherSendersReceiver(t *testing.T) {
	device := newFakeDevice(t)
	rec := startKeeper(t, newLaunchingKeeper(device, &launchCounter{}, 50*time.Millisecond))
	rec.waitFor(t, StateShowing)
	before, stops := device.launchCount(), device.stopCount()

	// Another GDash, or anyone with DashCast, opens its own page.
	device.castFrom(Application{AppID: DashCastAppID, SessionID: "other-server", TransportID: "web-x"})
	rec.waitFor(t, StateYielded)
	expectLaunches(t, device, before, "the other sender keeps the TV")
	if n := device.stopCount(); n != stops {
		t.Fatalf("stops went from %d to %d; the other sender's session must be left alone", stops, n)
	}
}

func TestTwoKeepersOnOneTVStopFighting(t *testing.T) {
	device := newFakeDevice(t)
	first := startKeeper(t, newLaunchingKeeper(device, &launchCounter{}, 100*time.Millisecond))
	second := startKeeper(t, newLaunchingKeeper(device, &launchCounter{}, 100*time.Millisecond))

	// Whoever launched last holds the TV and restarts its own page; the
	// other yields and stays out, also while the holder's restarts make
	// the screen free for a moment.
	time.Sleep(time.Second)
	settled := []State{first.last(), second.last()}
	slices.Sort(settled)
	if !slices.Equal(settled, []State{StateShowing, StateYielded}) {
		t.Fatalf("states = %v, want one keeper showing and one yielded", settled)
	}
	yielded := first
	if first.last() == StateShowing {
		yielded = second
	}
	yielded.mu.Lock()
	seen := len(yielded.states)
	yielded.mu.Unlock()
	time.Sleep(500 * time.Millisecond)
	yielded.mu.Lock()
	defer yielded.mu.Unlock()
	for _, s := range yielded.states[seen:] {
		if s != StateYielded {
			t.Fatalf("the yielded keeper reported %s after settling: %v", s, yielded.states[seen:])
		}
	}
}

func TestKeeperYieldsWhenTheTVSwitchesInput(t *testing.T) {
	device := newFakeDevice(t)
	device.setInput(true)
	rec := startKeeper(t, newRefreshingKeeper(device, 50*time.Millisecond))
	rec.waitFor(t, StateShowing)

	// Someone switches the TV to the game console.
	device.setInput(false)
	rec.waitFor(t, StateYielded)
	before := device.launchCount()
	expectLaunches(t, device, before, "no restart on a TV that shows another input")

	// Back to the Cast device: the dashboard is still there, and GDash
	// holds it again, with its restarts.
	device.setInput(true)
	rec.waitFor(t, StateShowing)
	waitForLaunches(t, device, before+1)
}

func TestKeeperDoesNotLaunchOnAnotherInput(t *testing.T) {
	device := newFakeDevice(t)
	device.setInput(false)
	rec := startKeeper(t, newRefreshingKeeper(device, 0))
	rec.waitFor(t, StateOtherApp)
	expectLaunches(t, device, 0, "launching would switch the TV away from what someone watches")

	device.setInput(true)
	waitForLaunches(t, device, 1)
	rec.waitFor(t, StateShowing)
}

func TestKeeperWaitsAtStartWhileTheTVIsInUse(t *testing.T) {
	// After a server restart DashCast runs. It may be GDash's page from
	// before the restart or someone else's; GDash cannot tell, so it
	// leaves it until the TV has been in standby.
	device := newFakeDevice(t, Application{AppID: DashCastAppID, SessionID: "unknown", TransportID: "web-0"})
	rec := startKeeper(t, newLaunchingKeeper(device, &launchCounter{}, 50*time.Millisecond))
	rec.waitFor(t, StateYielded)
	expectLaunches(t, device, 0, "a cast found at start is left alone")
	if n := device.stopCount(); n != 0 {
		t.Fatalf("stops = %d, want the running cast left alone", n)
	}

	device.setStandby(true, false)
	rec.waitFor(t, StateStandby)
	device.setStandby(false, false)
	waitForLaunches(t, device, 1)
	rec.waitFor(t, StateShowing)
}

func TestKeeperReturnsAfterTheTVWasSwitchedOff(t *testing.T) {
	device := newFakeDevice(t)
	rec := startKeeper(t, newRefreshingKeeper(device, 0))
	rec.waitFor(t, StateShowing)
	device.closeApps()
	rec.waitFor(t, StateYielded)

	device.setStandby(true, true)
	rec.waitFor(t, StateStandby)
	expectLaunches(t, device, 1, "a sleeping TV is not switched on")
	device.setStandby(false, false)
	waitForLaunches(t, device, 2)
	rec.waitFor(t, StateShowing)
}

func TestKeeperKeepsItsReceiverAcrossAReconnect(t *testing.T) {
	for _, tt := range []struct {
		name   string
		keeper func(*fakeDevice) *Keeper
	}{
		{"own receiver", func(d *fakeDevice) *Keeper { return newRefreshingKeeper(d, 0) }},
		{"DashCast", func(d *fakeDevice) *Keeper { return newLaunchingKeeper(d, &launchCounter{}, 0) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			device := newFakeDevice(t)
			rec := startKeeper(t, tt.keeper(device))
			rec.waitFor(t, StateShowing)
			time.Sleep(100 * time.Millisecond) // the startup resends finish

			// A network dip: the keeper reconnects and finds its own session.
			(<-device.conns).Close()
			rec.waitFor(t, StateUnreachable)
			rec.waitFor(t, StateShowing)
			expectLaunches(t, device, 1, "the receiver GDash launched stays")
			if n := device.stopCount(); n != 0 {
				t.Fatalf("stops = %d, want the page left as it is", n)
			}
		})
	}
}

func TestKeeperLaunchesAfterTheDeviceWasGone(t *testing.T) {
	// A Chromecast powered by the TV's USB port is off while the TV is, and
	// loses its apps; GDash did not see anyone take the TV.
	device := newFakeDevice(t)
	rec := startKeeper(t, newRefreshingKeeper(device, 0))
	rec.waitFor(t, StateShowing)
	(<-device.conns).Close()
	device.mu.Lock()
	device.apps = nil
	device.mu.Unlock()

	waitForLaunches(t, device, 2)
	rec.waitFor(t, StateShowing)
}

func TestKeeperStaysYieldedThroughActiveHours(t *testing.T) {
	device := newFakeDevice(t)
	var active atomic.Bool
	active.Store(true)
	keeper := newRefreshingKeeper(device, 0)
	keeper.Active = active.Load
	rec := startKeeper(t, keeper)
	rec.waitFor(t, StateShowing)
	device.closeApps()
	rec.waitFor(t, StateYielded)

	// Active hours end and start again while nobody switched the TV off.
	active.Store(false)
	device.change(func() {})
	rec.waitFor(t, StateResting)
	active.Store(true)
	device.change(func() {})
	rec.waitFor(t, StateYielded)
	expectLaunches(t, device, 1, "a new day of active hours does not end a yield")
}

func TestKeeperTakesTheTVBackOnRequest(t *testing.T) {
	device := newFakeDevice(t)
	device.setInput(true)
	takeOver := make(chan struct{}, 1)
	keeper := newRefreshingKeeper(device, 0)
	keeper.TakeOver = takeOver
	rec := startKeeper(t, keeper)
	rec.waitFor(t, StateShowing)
	device.castFrom(Application{AppID: "233637DE", DisplayName: "YouTube", SessionID: "yt", TransportID: "yt"})
	rec.waitFor(t, StateYielded)
	device.setInput(false)

	takeOver <- struct{}{}
	waitForLaunches(t, device, 2)
	rec.waitFor(t, StateShowing)
	device.mu.Lock()
	apps := append([]Application(nil), device.apps...)
	device.mu.Unlock()
	if len(apps) != 1 || apps[0].AppID != "ABCD1234" {
		t.Fatalf("apps = %+v, want GDash's receiver in place of the other app", apps)
	}
	// The TV still reports the other input for a moment after the launch:
	// that is the TV following, not someone taking it again.
	device.change(func() {})
	time.Sleep(quiet)
	if got := rec.last(); got != StateShowing {
		t.Fatalf("state = %s right after taking the TV back, want showing", got)
	}
	device.setInput(true)
	time.Sleep(100 * time.Millisecond) // the keeper sees the TV follow
	device.setInput(false)
	rec.waitFor(t, StateYielded)
}

func TestReceiverStatusTellsUnreportedFromFalse(t *testing.T) {
	var unreported, otherInput ReceiverStatus
	if err := json.Unmarshal([]byte(`{"applications":[]}`), &unreported); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"applications":[],"isActiveInput":false,"isStandBy":true}`), &otherInput); err != nil {
		t.Fatal(err)
	}
	if unreported.OnOtherInput() || unreported.OnThisInput() || unreported.InStandby() {
		t.Fatalf("a device without HDMI-CEC reads as %+v", unreported)
	}
	if !otherInput.OnOtherInput() || !otherInput.InStandby() {
		t.Fatalf("reported values were lost: %+v", otherInput)
	}
}
