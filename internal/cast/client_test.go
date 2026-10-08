package cast

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

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
	if err := c.Send(app.TransportID, "urn:x-cast:app.daycast.display", []byte(`{"type":"snapshot"}`)); err != nil {
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
		Addr:      device.addr(),
		AppID:     "ABCD1234",
		Namespace: "urn:x-cast:app.daycast.display",
		Subscribe: func(ctx context.Context) <-chan []byte {
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
	if n := device.launchCount(); n != 1 {
		t.Fatalf("launches = %d, want 1", n)
	}
}

func TestShouldRelaunch(t *testing.T) {
	tests := []struct {
		name   string
		status ReceiverStatus
		want   bool
	}{
		{"nothing running", ReceiverStatus{}, true},
		{"only the idle screen", ReceiverStatus{Applications: []Application{{AppID: "E8C28D3C", IsIdleScreen: true}}}, true},
		{"someone is casting", ReceiverStatus{Applications: []Application{{AppID: "233637DE", DisplayName: "YouTube"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRelaunch(tt.status); got != tt.want {
				t.Fatalf("shouldRelaunch = %v, want %v", got, tt.want)
			}
		})
	}
}
