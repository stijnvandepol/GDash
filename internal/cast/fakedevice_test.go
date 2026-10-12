package cast

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeDevice is a minimal CASTV2 device on loopback. It understands
// heartbeats, GET_STATUS, LAUNCH and STOP, records what senders send to
// launched apps, and can play what people do with a TV: close the app,
// cast from another sender, switch input or switch the TV off. Each of
// those reaches the connected senders as an unsolicited status, as on
// real hardware.
type fakeDevice struct {
	t  *testing.T
	ln net.Listener

	mu       sync.Mutex
	apps     []Application
	launches []string
	stops    []string
	standby  bool
	// activeInput is left out of the status while nil, as by a device
	// without HDMI-CEC.
	activeInput *bool
	open        []*fakeConn

	appMessages chan Message
	pongs       chan struct{}
	conns       chan net.Conn
}

// fakeConn is one sender's connection; replies and pushed statuses share
// its write lock.
type fakeConn struct {
	net.Conn
	writeMu sync.Mutex
}

func (c *fakeConn) send(m Message) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = writeFrame(c.Conn, m)
}

func newFakeDevice(t *testing.T, apps ...Application) *fakeDevice {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSignedCert(t)}})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	d := &fakeDevice{
		t:           t,
		ln:          ln,
		apps:        apps,
		appMessages: make(chan Message, 16),
		pongs:       make(chan struct{}, 16),
		conns:       make(chan net.Conn, 4),
	}
	t.Cleanup(func() { ln.Close() })
	go d.serve()
	return d
}

func (d *fakeDevice) addr() string { return d.ln.Addr().String() }

func (d *fakeDevice) launchCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.launches)
}

func (d *fakeDevice) stopCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.stops)
}

// stopped returns the sessions senders stopped.
func (d *fakeDevice) stopped() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.stops...)
}

// closeApps ends every app, as someone pressing Back or Home on the remote
// does, and as Google TV reports a native app such as Netflix.
func (d *fakeDevice) closeApps() {
	d.change(func() { d.apps = nil })
}

// castFrom puts app on screen as another sender's cast, replacing what ran.
func (d *fakeDevice) castFrom(app Application) {
	d.change(func() { d.apps = []Application{app} })
}

// setInput reports whether the TV shows the Cast device's input.
func (d *fakeDevice) setInput(active bool) {
	d.change(func() { d.activeInput = &active })
}

// setStandby switches the TV off or on; off also ends every app when
// endApps is set, as some devices do.
func (d *fakeDevice) setStandby(on, endApps bool) {
	d.change(func() {
		d.standby = on
		if endApps {
			d.apps = nil
		}
	})
}

// change applies a change and pushes the new status to every sender.
func (d *fakeDevice) change(apply func()) {
	d.mu.Lock()
	apply()
	d.mu.Unlock()
	d.push(d.statusPayload(0))
}

// push sends a RECEIVER_STATUS payload to every sender on its own, also one
// that no longer matches the device, as a late update after a STOP.
func (d *fakeDevice) push(payload string) {
	d.mu.Lock()
	open := append([]*fakeConn(nil), d.open...)
	d.mu.Unlock()
	for _, c := range open {
		c.send(Message{SourceID: receiverID, DestinationID: "*", Namespace: nsReceiver, Payload: payload})
	}
}

func (d *fakeDevice) serve() {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			return
		}
		d.conns <- conn
		c := &fakeConn{Conn: conn}
		d.mu.Lock()
		d.open = append(d.open, c)
		d.mu.Unlock()
		go d.handle(c)
	}
}

func (d *fakeDevice) handle(conn *fakeConn) {
	defer d.forget(conn)
	for {
		m, err := readFrame(conn)
		if err != nil {
			return
		}
		var payload map[string]any
		_ = json.Unmarshal([]byte(m.Payload), &payload)
		switch m.Namespace {
		case nsHeartbeat:
			switch payload["type"] {
			case "PING":
				conn.send(Message{SourceID: receiverID, DestinationID: m.SourceID, Namespace: nsHeartbeat, Payload: `{"type":"PONG"}`})
			case "PONG":
				d.pongs <- struct{}{}
			}
		case nsReceiver:
			changed := d.receiverRequest(payload)
			conn.send(Message{SourceID: receiverID, DestinationID: m.SourceID, Namespace: nsReceiver, Payload: d.statusPayload(payload["requestId"])})
			if changed {
				// A real device tells every sender when an app starts or
				// stops, also the one that asked.
				d.push(d.statusPayload(0))
			}
		case nsConnection:
		default:
			d.appMessages <- m
		}
	}
}

func (d *fakeDevice) forget(conn *fakeConn) {
	conn.Close()
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, c := range d.open {
		if c == conn {
			d.open = append(d.open[:i], d.open[i+1:]...)
			return
		}
	}
}

// receiverRequest carries out a request and reports whether it changed the
// running apps.
func (d *fakeDevice) receiverRequest(payload map[string]any) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch payload["type"] {
	case "STOP":
		sessionID, _ := payload["sessionId"].(string)
		d.stops = append(d.stops, sessionID)
		kept := d.apps[:0]
		for _, app := range d.apps {
			if app.SessionID != sessionID {
				kept = append(kept, app)
			}
		}
		d.apps = kept
	case "LAUNCH":
		appID, _ := payload["appId"].(string)
		d.launches = append(d.launches, appID)
		n := len(d.launches)
		d.apps = []Application{{AppID: appID, SessionID: fmt.Sprintf("session-%d", n), TransportID: fmt.Sprintf("web-%d", n)}}
	default:
		return false
	}
	return true
}

func (d *fakeDevice) statusPayload(requestID any) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	status := map[string]any{"applications": d.apps, "isStandBy": d.standby}
	if d.activeInput != nil {
		status["isActiveInput"] = *d.activeInput
	}
	body, _ := json.Marshal(map[string]any{"type": "RECEIVER_STATUS", "requestId": requestID, "status": status})
	return string(body)
}

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
