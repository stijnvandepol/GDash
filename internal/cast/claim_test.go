package cast

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
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

// fakeClock is a keeper's clock that the test moves, and that counts how
// often the keeper read it: a yielded keeper reads it for every status.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	reads int
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads++
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *fakeClock) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

var (
	backdrop = Application{AppID: "E8C28D3C", DisplayName: "Backdrop", IsIdleScreen: true}
	radio    = Application{AppID: "radio", DisplayName: "Radio", SessionID: "radio", TransportID: "radio"}
)

// idleTest is a keeper that yielded to radio cast to the device, with a
// clock the test moves.
type idleTest struct {
	device *fakeDevice
	clock  *fakeClock
	rec    *stateRecorder
}

func newIdleTest(t *testing.T) *idleTest {
	t.Helper()
	device := newFakeDevice(t)
	clock := &fakeClock{now: time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)}
	keeper := newRefreshingKeeper(device, 0)
	keeper.Now = clock.Now
	rec := startKeeper(t, keeper)
	rec.waitFor(t, StateShowing)
	device.castFrom(radio)
	rec.waitFor(t, StateYielded)
	return &idleTest{device: device, clock: clock, rec: rec}
}

// status changes the device, tells the keeper, and waits until the keeper
// has looked at the new status.
func (it *idleTest) status(t *testing.T, change func(d *fakeDevice)) {
	t.Helper()
	before := it.clock.readCount()
	it.device.change(func() { change(it.device) })
	deadline := time.Now().Add(testTimeout)
	for it.clock.readCount() == before {
		if time.Now().After(deadline) {
			t.Fatal("the keeper did not look at the new status")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func showIdleScreen(d *fakeDevice) { d.apps = []Application{backdrop} }
func playRadio(d *fakeDevice)      { d.apps = []Application{radio} }
func unchanged(*fakeDevice)        {}

func TestYieldOutlastsAnIdleScreenThatIsInterrupted(t *testing.T) {
	it := newIdleTest(t)
	it.status(t, showIdleScreen)
	it.clock.advance(9 * time.Minute)
	it.status(t, unchanged)
	// The radio plays again for a moment: the idle time starts over.
	it.status(t, playRadio)
	it.status(t, showIdleScreen)
	it.clock.advance(9 * time.Minute)
	it.status(t, unchanged)

	expectLaunches(t, it.device, 1, "nine minutes of idle screen at a time do not end the yield")
	if got := it.rec.last(); got != StateYielded {
		t.Fatalf("state = %s, want yielded", got)
	}
}

func TestYieldEndsAfterTenMinutesOfIdleScreen(t *testing.T) {
	// A Nest Hub never reports standby; its own screen after the radio
	// stopped says that nobody uses it any more.
	it := newIdleTest(t)
	it.status(t, showIdleScreen)
	it.clock.advance(idleYieldTimeout)
	it.status(t, unchanged)

	waitForLaunches(t, it.device, 2)
	it.rec.waitFor(t, StateShowing)
}

func TestYieldOutlastsWhatOnlyLooksIdle(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(d *fakeDevice)
	}{
		// Google TV reports no apps while Netflix plays.
		{"no apps", func(d *fakeDevice) { d.apps = nil }},
		{"idle screen while the TV shows another input", func(d *fakeDevice) {
			otherInput := false
			d.apps, d.activeInput = []Application{backdrop}, &otherInput
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			it := newIdleTest(t)
			it.status(t, tt.change)
			for range 2 {
				it.clock.advance(15 * time.Minute)
				it.status(t, unchanged)
			}
			expectLaunches(t, it.device, 1, "30 minutes of this do not end the yield")
			if got := it.rec.last(); got != StateYielded {
				t.Fatalf("state = %s, want yielded", got)
			}
		})
	}
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
