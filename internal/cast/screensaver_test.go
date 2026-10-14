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
// clock the test moves and a channel for the page's hidden reports.
type screensaverTest struct {
	device   *fakeDevice
	clock    *fakeClock
	hidden   chan struct{}
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
		hidden:   make(chan struct{}, 1),
		restarts: &restartLog{},
	}
	keeper := newRefreshingKeeper(device, 0)
	keeper.Now = st.clock.Now
	keeper.Hidden = st.hidden
	keeper.Restarted = st.restarts.add
	st.rec = startKeeper(t, keeper)
	st.rec.waitFor(t, StateShowing)
	waitForLaunches(t, device, 1)
	return st
}

// reportHidden passes a hidden report and waits until the keeper took it.
func (st *screensaverTest) reportHidden(t *testing.T) {
	t.Helper()
	st.hidden <- struct{}{}
	deadline := time.Now().Add(testTimeout)
	for len(st.hidden) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the keeper did not take the hidden report")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHiddenScreenRestartsTheReceiver(t *testing.T) {
	st := newScreensaverTest(t)

	// Right after the launch: the receiver's own page going away, or a TV
	// that covers it again at once. Not restarted.
	st.reportHidden(t)
	expectLaunches(t, st.device, 1, "a hidden report right after a launch")

	st.clock.advance(minHiddenRestartGap + time.Second)
	st.reportHidden(t)
	waitForLaunches(t, st.device, 2)
	if got := st.restarts.list(); !slices.Equal(got, []string{RestartHidden}) {
		t.Fatalf("restarts = %v, want one for the hidden screen", got)
	}

	// At most one restart per gap, however often the screen reports.
	st.reportHidden(t)
	expectLaunches(t, st.device, 2, "a second hidden report within the gap")
}

func TestHiddenScreenDoesNotRestartATVThatSleepsOrIsInUse(t *testing.T) {
	t.Run("standby", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.setStandby(true, false)
		st.rec.waitFor(t, StateStandby)
		st.clock.advance(minHiddenRestartGap + time.Second)
		st.reportHidden(t)
		expectLaunches(t, st.device, 1, "launching could switch a sleeping TV on")
	})
	t.Run("another input", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.setInput(false)
		st.rec.waitFor(t, StateYielded)
		st.clock.advance(minHiddenRestartGap + time.Second)
		st.reportHidden(t)
		expectLaunches(t, st.device, 1, "the TV shows a game console")
	})
	t.Run("another app", func(t *testing.T) {
		st := newScreensaverTest(t)
		st.device.castFrom(radio)
		st.rec.waitFor(t, StateYielded)
		st.clock.advance(minHiddenRestartGap + time.Second)
		st.reportHidden(t)
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
