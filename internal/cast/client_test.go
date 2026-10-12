package cast

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func TestMain(m *testing.M) {
	// A receiver page on a TV needs seconds to load; the fake device's
	// receivers take messages at once. Set before any keeper runs.
	startupResends = []time.Duration{20 * time.Millisecond, 50 * time.Millisecond}
	os.Exit(m.Run())
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

func TestClientLaunchesAppAndSendsMessage(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)

	c, err := Dial(ctx, device.addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(status.Applications) != 0 {
		t.Fatalf("idle device reports apps: %+v", status.Applications)
	}

	app, err := c.Launch(ctx, "ABCD1234")
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if app.TransportID != "web-1" {
		t.Fatalf("transport = %q, want web-1", app.TransportID)
	}
	if err := c.Connect(app.TransportID); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := c.Send(app.TransportID, "urn:x-cast:app.gdash.display", []byte(`{"type":"snapshot"}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case m := <-device.appMessages:
		if m.DestinationID != "web-1" || m.Payload != `{"type":"snapshot"}` {
			t.Fatalf("device received %+v", m)
		}
	case <-ctx.Done():
		t.Fatal("device never received the app message")
	}
}

func TestClientStopsSession(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	c, err := Dial(ctx, device.addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	app, err := c.Launch(ctx, "ABCD1234")
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := c.Stop(ctx, app.SessionID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if _, running := status.App("ABCD1234"); running {
		t.Fatal("app still running after Stop")
	}
}

func TestClientAnswersDevicePing(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	c, err := Dial(ctx, device.addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	conn := <-device.conns
	if err := writeFrame(conn, Message{SourceID: receiverID, DestinationID: senderID, Namespace: nsHeartbeat, Payload: `{"type":"PING"}`}); err != nil {
		t.Fatalf("send PING: %v", err)
	}
	select {
	case <-device.pongs:
	case <-ctx.Done():
		t.Fatal("client did not answer PING with PONG")
	}
}

func TestClientReportsClosedConnection(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	c, err := Dial(ctx, device.addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	(<-device.conns).Close()

	select {
	case <-c.Done():
	case <-ctx.Done():
		t.Fatal("client did not notice the closed connection")
	}
	if c.Err() == nil {
		t.Fatal("Err() = nil after the device dropped the connection")
	}
	if _, err := c.Status(ctx); err == nil {
		t.Fatal("Status succeeded on a closed connection")
	}
}

func TestKeeperLaunchesOnIdleDeviceAndForwardsState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t, Application{AppID: "E8C28D3C", DisplayName: "Backdrop", IsIdleScreen: true})

	keeper := &Keeper{
		Resolve:   func(context.Context) (Device, error) { return Device{Addr: device.addr()}, nil },
		AppID:     "ABCD1234",
		Namespace: "urn:x-cast:app.gdash.display",
		Messages: func(ctx context.Context) <-chan []byte {
			ch := make(chan []byte, 1)
			ch <- []byte(`{"type":"snapshot","version":1}`)
			return ch
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	go keeper.Run(ctx)

	select {
	case m := <-device.appMessages:
		if m.Payload != `{"type":"snapshot","version":1}` {
			t.Fatalf("receiver got %q", m.Payload)
		}
	case <-ctx.Done():
		t.Fatal("keeper never delivered the state to the receiver")
	}
	// The startup resends deliver the same state again while the receiver
	// page may still be loading.
	select {
	case m := <-device.appMessages:
		if m.Payload != `{"type":"snapshot","version":1}` {
			t.Fatalf("resend carried %q", m.Payload)
		}
	case <-ctx.Done():
		t.Fatal("keeper did not resend the state after startup")
	}
	if n := device.launchCount(); n != 1 {
		t.Fatalf("launches = %d, want 1", n)
	}
}

func newRefreshingKeeper(device *fakeDevice, every time.Duration) *Keeper {
	return &Keeper{
		Resolve:   func(context.Context) (Device, error) { return Device{Addr: device.addr()}, nil },
		AppID:     "ABCD1234",
		Namespace: "urn:x-cast:app.gdash.display",
		Messages: func(ctx context.Context) <-chan []byte {
			ch := make(chan []byte, 1)
			ch <- []byte(`{"type":"snapshot"}`)
			return ch
		},
		Refresh: func(Device) time.Duration { return every },
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func waitForLaunches(t *testing.T, device *fakeDevice, want int) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for device.launchCount() < want {
		if time.Now().After(deadline) {
			t.Fatalf("launches = %d, want at least %d", device.launchCount(), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestKeeperRestartsReceiverPeriodically(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t)
	go newRefreshingKeeper(device, 100*time.Millisecond).Run(ctx)
	waitForLaunches(t, device, 3)
}

func TestKeeperDoesNotRestartDuringStandby(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t)
	go newRefreshingKeeper(device, 50*time.Millisecond).Run(ctx)
	waitForLaunches(t, device, 1)

	// The TV goes to standby while the receiver runs: no more restarts.
	device.mu.Lock()
	device.standby = true
	device.mu.Unlock()
	time.Sleep(100 * time.Millisecond) // let a restart that already started finish
	before := device.launchCount()
	time.Sleep(300 * time.Millisecond)
	if n := device.launchCount(); n != before {
		t.Fatalf("launches went from %d to %d during standby, want no restarts", before, n)
	}
}

func TestKeeperDoesNotWakeATVInStandby(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t)
	device.standby = true
	go newRefreshingKeeper(device, 0).Run(ctx)

	time.Sleep(300 * time.Millisecond)
	if n := device.launchCount(); n != 0 {
		t.Fatalf("launches = %d while the TV sleeps, want 0: a launch can switch the TV on", n)
	}

	// Someone switches the TV on; the device reports its new state.
	device.setStandby(false, false)
	waitForLaunches(t, device, 1)
}

func TestKeeperLeavesARunningReceiverAloneInStandby(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	// After a server restart at night: DashCast still runs, the TV sleeps.
	device := newFakeDevice(t, Application{AppID: DashCastAppID, SessionID: "old", TransportID: "web-0"})
	device.standby = true
	go newLaunchingKeeper(device, &launchCounter{}, 0).Run(ctx)

	time.Sleep(300 * time.Millisecond)
	device.mu.Lock()
	defer device.mu.Unlock()
	if len(device.apps) != 1 || len(device.launches) != 0 {
		t.Fatalf("apps = %v, launches = %v; want the sleeping TV left alone", device.apps, device.launches)
	}
}

// launchCounter builds numbered launch messages and records the address
// the keeper passed.
type launchCounter struct {
	mu       sync.Mutex
	n        int
	localIPs []string
}

func (c *launchCounter) message(localIP, _ string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	c.localIPs = append(c.localIPs, localIP)
	return []byte(fmt.Sprintf(`{"launch":%d}`, c.n)), nil
}

func newLaunchingKeeper(device *fakeDevice, launches *launchCounter, refresh time.Duration) *Keeper {
	return &Keeper{
		Resolve:       func(context.Context) (Device, error) { return Device{Addr: device.addr()}, nil },
		AppID:         DashCastAppID,
		Namespace:     DashCastNamespace,
		LaunchMessage: launches.message,
		Refresh:       func(Device) time.Duration { return refresh },
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestKeeperReplacesALeftoverReceiverAfterStandby(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	// After a server restart at night: DashCast still runs, the TV sleeps.
	device := newFakeDevice(t, Application{AppID: DashCastAppID, SessionID: "old", TransportID: "web-0"})
	device.standby = true
	launches := &launchCounter{}
	keeper := newLaunchingKeeper(device, launches, 0)
	rec := &stateRecorder{}
	keeper.Status = rec.record
	go keeper.Run(ctx)
	rec.waitFor(t, StateStandby)

	// In the morning someone switches the TV on: nobody watches the
	// leftover receiver, which would ignore a new page, so it is replaced.
	device.setStandby(false, false)
	waitForLaunches(t, device, 1)
	select {
	case m := <-device.appMessages:
		if m.DestinationID != "web-1" || m.Payload != `{"launch":1}` {
			t.Fatalf("message %q went to %s, want the first launch message on the relaunched web-1", m.Payload, m.DestinationID)
		}
	case <-ctx.Done():
		t.Fatal("relaunched receiver never got the message")
	}
	if stops := device.stopped(); len(stops) != 1 || stops[0] != "old" {
		t.Fatalf("stops = %v, want the leftover session stopped once", stops)
	}
	launches.mu.Lock()
	defer launches.mu.Unlock()
	if launches.localIPs[0] != "127.0.0.1" {
		t.Fatalf("localIP = %q, want the server's address towards the device", launches.localIPs[0])
	}
}

func TestKeeperBuildsAFreshLaunchMessageOnEveryRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t)
	launches := &launchCounter{}
	go newLaunchingKeeper(device, launches, 100*time.Millisecond).Run(ctx)

	waitForLaunches(t, device, 3)
	seen := map[string]bool{}
	for len(seen) < 3 {
		select {
		case m := <-device.appMessages:
			seen[m.Payload] = true
		case <-ctx.Done():
			t.Fatalf("saw launch messages %v, want three different ones", seen)
		}
	}
}

func TestKeeperIgnoresStaleStatusUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t)
	go newRefreshingKeeper(device, 0).Run(ctx)
	waitForLaunches(t, device, 1)

	// The device reports "no apps" on its own while the app still runs,
	// as a late update after a STOP does.
	device.push(`{"type":"RECEIVER_STATUS","requestId":0,"status":{"applications":[]}}`)
	time.Sleep(300 * time.Millisecond)
	if n := device.launchCount(); n != 1 {
		t.Fatalf("launches = %d after a stale update, want 1", n)
	}
}

func TestKeeperRestsOutsideActiveHours(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	device := newFakeDevice(t, Application{AppID: "ABCD1234", SessionID: "s", TransportID: "web-0"})
	keeper := newRefreshingKeeper(device, 50*time.Millisecond)
	keeper.Active = func() bool { return false }
	go keeper.Run(ctx)

	deadline := time.Now().Add(testTimeout)
	for {
		device.mu.Lock()
		stopped := len(device.apps) == 0
		device.mu.Unlock()
		if stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("keeper did not stop its receiver outside active hours")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Refresh ticks pass, but nothing is launched while inactive.
	time.Sleep(300 * time.Millisecond)
	if n := device.launchCount(); n != 0 {
		t.Fatalf("launches = %d outside active hours, want 0", n)
	}
}

func TestShouldRelaunch(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name   string
		status ReceiverStatus
		want   bool
	}{
		{"nothing running", ReceiverStatus{}, true},
		{"only the idle screen", ReceiverStatus{Applications: []Application{{AppID: "E8C28D3C", IsIdleScreen: true}}}, true},
		{"someone is casting", ReceiverStatus{Applications: []Application{{AppID: "233637DE", DisplayName: "YouTube"}}}, false},
		{"the TV shows another input", ReceiverStatus{IsActiveInput: &no}, false},
		{"the TV shows the Cast device", ReceiverStatus{IsActiveInput: &yes}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRelaunch(tt.status); got != tt.want {
				t.Fatalf("ShouldRelaunch = %v, want %v", got, tt.want)
			}
		})
	}
}
