package cast

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// restartLog records the reasons a keeper passes to Restarted.
type restartLog struct {
	mu      sync.Mutex
	reasons []string
}

func (l *restartLog) add(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reasons = append(l.reasons, reason)
}

func (l *restartLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.reasons)
}

// screensaverTest is a keeper on a TV that shows its receiver, with a
// clock the test moves, a restart interval it sets, and channels for the
// page's hidden and visible reports.
type screensaverTest struct {
	device   *fakeDevice
	clock    *fakeClock
	every    *atomic.Int64
	hidden   chan struct{}
	shown    chan struct{}
	restarts *restartLog
	rec      *stateRecorder
}

func newScreensaverTest(t *testing.T) *screensaverTest {
	t.Helper()
	device := newFakeDevice(t)
	device.setInput(true)
	st := &screensaverTest{
		device:   device,
		clock:    &fakeClock{now: time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)},
		every:    &atomic.Int64{},
		hidden:   make(chan struct{}, 1),
		shown:    make(chan struct{}, 1),
		restarts: &restartLog{},
	}
	keeper := newRefreshingKeeper(device, 0)
	keeper.Refresh = func(Device) time.Duration { return time.Duration(st.every.Load()) }
	keeper.Now = st.clock.Now
	keeper.Hidden = st.hidden
	keeper.Shown = st.shown
	keeper.Restarted = st.restarts.add
	st.rec = startKeeper(t, keeper)
	st.rec.waitFor(t, StateShowing)
	waitForLaunches(t, device, 1)
	return st
}

// send passes a report of the page and waits until the keeper took it.
func (st *screensaverTest) send(t *testing.T, ch chan struct{}) {
	t.Helper()
	ch <- struct{}{}
	deadline := time.Now().Add(testTimeout)
	for len(ch) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the keeper did not take the report")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// restartEvery sets the restart interval and lets the keeper read it.
func (st *screensaverTest) restartEvery(d time.Duration) {
	st.every.Store(int64(d))
	st.device.change(func() {})
}

func TestHiddenScreenYieldsToTheAppOverIt(t *testing.T) {
	st := newScreensaverTest(t)

	// Right after a launch: the previous page going away. Nothing changes.
	st.send(t, st.hidden)
	time.Sleep(quiet)
	if got := st.rec.last(); got != StateShowing {
		t.Fatalf("state after a hidden report right after a launch = %s, want showing", got)
	}

	// Netflix opened on a Google TV: the device still reports only the
	// receiver, so only the page can tell. GDash leaves the TV alone, also
	// when the next status looks like its own receiver on a free screen.
	st.clock.advance(ownPageGoneGap + time.Second)
	st.send(t, st.hidden)
	st.rec.waitFor(t, StateYielded)
	st.device.change(func() {})
	time.Sleep(quiet)
	if got := st.rec.last(); got != StateYielded {
		t.Fatalf("state after a status = %s, want yielded while the page is hidden", got)
	}
	expectLaunches(t, st.device, 1, "nothing is launched over the app")

	// The dashboard is on screen again, such as after the screensaver.
	st.send(t, st.shown)
	st.rec.waitFor(t, StateShowing)
	expectLaunches(t, st.device, 1, "the receiver that is on screen is kept")
}

func TestPeriodicRestartWaitsWhileTheScreenIsHidden(t *testing.T) {
	st := newScreensaverTest(t)
	st.clock.advance(ownPageGoneGap + time.Second)
	st.send(t, st.hidden)
	st.rec.waitFor(t, StateYielded)

	st.restartEvery(80 * time.Millisecond)
	expectLaunches(t, st.device, 1, "a restart would put the dashboard back over the app")
	if got := st.restarts.list(); len(got) != 0 {
		t.Fatalf("restarts = %v, want none while the screen is hidden", got)
	}

	st.send(t, st.shown)
	waitForLaunches(t, st.device, 2)
	if got := st.restarts.list(); len(got) == 0 || got[0] != RestartPeriodic {
		t.Fatalf("restarts = %v, want periodic ones once the screen is shown", got)
	}
}

func TestHiddenScreenEndsWithStandby(t *testing.T) {
	st := newScreensaverTest(t)
	st.clock.advance(ownPageGoneGap + time.Second)
	st.send(t, st.hidden)
	st.rec.waitFor(t, StateYielded)

	// Switched off and on again: the screen is GDash's once more.
	st.device.setStandby(true, false)
	st.rec.waitFor(t, StateStandby)
	st.device.setStandby(false, false)
	st.rec.waitFor(t, StateShowing)
}

func TestHiddenScreenIsLeftAloneOnATVThatSleepsOrIsInUse(t *testing.T) {
	t.Run("standby", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.setStandby(true, false)
		st.rec.waitFor(t, StateStandby)
		st.clock.advance(ownPageGoneGap + time.Second)
		st.send(t, st.hidden)
		expectLaunches(t, st.device, 1, "launching could switch a sleeping TV on")
	})
	t.Run("another input", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.setInput(false)
		st.rec.waitFor(t, StateYielded)
		st.clock.advance(ownPageGoneGap + time.Second)
		st.send(t, st.hidden)
		st.send(t, st.shown)
		expectLaunches(t, st.device, 1, "the TV shows a game console")
	})
	t.Run("another app", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.castFrom(radio)
		st.rec.waitFor(t, StateYielded)
		st.clock.advance(ownPageGoneGap + time.Second)
		st.send(t, st.hidden)
		st.send(t, st.shown)
		expectLaunches(t, st.device, 1, "someone casts radio to the TV")
	})
}

func TestRestartIntervalFollowsTheSetting(t *testing.T) {
	device := newFakeDevice(t)
	var every atomic.Int64
	restarts := &restartLog{}
	keeper := newRefreshingKeeper(device, 0)
	keeper.Refresh = func(Device) time.Duration { return time.Duration(every.Load()) }
	keeper.Restarted = restarts.add
	rec := startKeeper(t, keeper)
	rec.waitFor(t, StateShowing)
	expectLaunches(t, device, 1, "restarts are off")

	// The admin chooses an interval; the next status applies it.
	every.Store(int64(80 * time.Millisecond))
	device.change(func() {})
	waitForLaunches(t, device, 3)
	// Restarted follows the launch, so it may still be on its way.
	deadline := time.Now().Add(testTimeout)
	for len(restarts.list()) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := restarts.list(); len(got) < 2 || got[0] != RestartPeriodic {
		t.Fatalf("restarts = %v, want periodic ones", got)
	}

	// And turns them off again.
	every.Store(0)
	device.change(func() {})
	time.Sleep(100 * time.Millisecond) // a restart that already started finishes
	before := device.launchCount()
	expectLaunches(t, device, before, "restarts are off again")
}

func TestPeriodicRestartCountsOnAcrossAReconnect(t *testing.T) {
	device := newFakeDevice(t)
	clock := &fakeClock{now: time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)}
	restarts := &restartLog{}
	keeper := newRefreshingKeeper(device, 10*time.Minute)
	keeper.Now = clock.Now
	keeper.Restarted = restarts.add
	rec := startKeeper(t, keeper)
	rec.waitFor(t, StateShowing)

	// The connection drops after eleven minutes. Counting from the new
	// connection would put the restart off again, and a connection that
	// keeps dropping would put it off for good.
	clock.advance(11 * time.Minute)
	device.dropConnections()
	waitForLaunches(t, device, 2)
	if got := restarts.list(); !slices.Equal(got, []string{RestartPeriodic}) {
		t.Fatalf("restarts = %v, want the periodic one that was due", got)
	}
}
