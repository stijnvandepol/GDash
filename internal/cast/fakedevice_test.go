package cast

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeDevice is a minimal CASTV2 device on loopback. It understands
// heartbeats, GET_STATUS and LAUNCH, and records what senders send to
// launched apps.
type fakeDevice struct {
	t  *testing.T
	ln net.Listener

	mu       sync.Mutex
	apps     []Application
	launches []string

	appMessages chan Message
	pongs       chan struct{}
	conns       chan net.Conn
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

func (d *fakeDevice) serve() {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			return
		}
		d.conns <- conn
		go d.handle(conn)
	}
}

func (d *fakeDevice) handle(conn net.Conn) {
	defer conn.Close()
	var writeMu sync.Mutex
	reply := func(m Message) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = writeFrame(conn, m)
	}
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
				reply(Message{SourceID: receiverID, DestinationID: m.SourceID, Namespace: nsHeartbeat, Payload: `{"type":"PONG"}`})
			case "PONG":
				d.pongs <- struct{}{}
			}
		case nsReceiver:
			switch payload["type"] {
			case "STOP":
				d.mu.Lock()
				d.apps = nil
				d.mu.Unlock()
			case "LAUNCH":
				d.mu.Lock()
				appID, _ := payload["appId"].(string)
				d.launches = append(d.launches, appID)
				d.apps = []Application{{AppID: appID, SessionID: "session-1", TransportID: "web-1"}}
				d.mu.Unlock()
			}
			reply(Message{SourceID: receiverID, DestinationID: m.SourceID, Namespace: nsReceiver, Payload: d.statusPayload(payload["requestId"])})
		case nsConnection:
		default:
			d.appMessages <- m
		}
	}
}

func (d *fakeDevice) statusPayload(requestID any) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	body, _ := json.Marshal(map[string]any{
		"type":      "RECEIVER_STATUS",
		"requestId": requestID,
		"status":    map[string]any{"applications": d.apps},
	})
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
