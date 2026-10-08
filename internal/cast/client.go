package cast

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	nsConnection = "urn:x-cast:com.google.cast.tp.connection"
	nsHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	nsReceiver   = "urn:x-cast:com.google.cast.receiver"

	senderID   = "sender-0"
	receiverID = "receiver-0"

	heartbeatInterval = 5 * time.Second
	dialTimeout       = 10 * time.Second
	// A device answers every PING within a few seconds; three missed
	// intervals means the connection is dead even if TCP has not noticed.
	readTimeout  = 3 * heartbeatInterval
	writeTimeout = 10 * time.Second
)

type receiverMessage struct {
	Type      string         `json:"type"`
	RequestID int64          `json:"requestId"`
	Status    ReceiverStatus `json:"status"`
	Reason    string         `json:"reason"`
}

// Client is a CASTV2 connection to one Cast device. It answers heartbeats on
// its own and closes itself when the device stops responding.
type Client struct {
	conn      net.Conn
	writeMu   sync.Mutex
	requestID atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan receiverMessage

	statusUpdates chan ReceiverStatus
	appMessages   chan Message

	done      chan struct{}
	closeOnce sync.Once
	err       error
}

// Dial opens a CASTV2 connection to addr (host:port, normally port 8009).
func Dial(ctx context.Context, addr string) (*Client, error) {
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout}, Config: &tls.Config{
		// Cast devices present a certificate issued by Google's device CA,
		// which is not in any system trust store. Verifying it requires the
		// device-auth challenge, which is out of scope for the prototype.
		// The connection stays on the local network.
		InsecureSkipVerify: true, //nolint:gosec // see comment above
	}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect to cast device %s: %w", addr, err)
	}
	c := &Client{
		conn:          conn,
		pending:       make(map[int64]chan receiverMessage),
		statusUpdates: make(chan ReceiverStatus, 1),
		appMessages:   make(chan Message, 8),
		done:          make(chan struct{}),
	}
	if err := c.sendJSON(receiverID, nsConnection, map[string]any{"type": "CONNECT"}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open virtual connection to %s: %w", addr, err)
	}
	go c.readLoop()
	go c.heartbeatLoop()
	return c, nil
}

// Close ends the connection. It does not stop apps running on the device.
func (c *Client) Close() error {
	c.shutdown(nil)
	return nil
}

// Done is closed when the connection has ended. Err then reports why.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err returns the reason the connection ended, or nil if Close was called.
func (c *Client) Err() error {
	<-c.done
	return c.err
}

// StatusUpdates delivers RECEIVER_STATUS messages the device sends on its
// own, for example when another app takes over the screen. Only the latest
// unread status is kept.
func (c *Client) StatusUpdates() <-chan ReceiverStatus { return c.statusUpdates }

// AppMessages delivers messages that apps on the device send to this sender
// on their own namespaces.
func (c *Client) AppMessages() <-chan Message { return c.appMessages }

func (c *Client) request(ctx context.Context, payload map[string]any) (receiverMessage, error) {
	id := c.requestID.Add(1)
	payload["requestId"] = id
	reply := make(chan receiverMessage, 1)

	c.pendingMu.Lock()
	c.pending[id] = reply
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if err := c.sendJSON(receiverID, nsReceiver, payload); err != nil {
		return receiverMessage{}, err
	}
	select {
	case resp := <-reply:
		return resp, nil
	case <-ctx.Done():
		return receiverMessage{}, ctx.Err()
	case <-c.done:
		return receiverMessage{}, c.closedErr()
	}
}

func (c *Client) sendJSON(destination, namespace string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s message: %w", namespace, err)
	}
	return c.write(Message{SourceID: senderID, DestinationID: destination, Namespace: namespace, Payload: string(body)})
}

func (c *Client) write(m Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	if err := writeFrame(c.conn, m); err != nil {
		c.shutdown(err)
		return fmt.Errorf("write %s message: %w", m.Namespace, err)
	}
	return nil
}

func (c *Client) readLoop() {
	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			c.shutdown(err)
			return
		}
		m, err := readFrame(c.conn)
		if err != nil {
			c.shutdown(fmt.Errorf("read from cast device: %w", err))
			return
		}
		c.dispatch(m)
	}
}

func (c *Client) dispatch(m Message) {
	switch m.Namespace {
	case nsHeartbeat:
		var hb struct{ Type string }
		if json.Unmarshal([]byte(m.Payload), &hb) == nil && hb.Type == "PING" {
			// A failed PONG closes the client through write(), so the
			// error needs no further handling here.
			_ = c.sendJSON(m.SourceID, nsHeartbeat, map[string]any{"type": "PONG"})
		}
	case nsConnection:
		var msg struct{ Type string }
		if json.Unmarshal([]byte(m.Payload), &msg) == nil && msg.Type == "CLOSE" && m.SourceID == receiverID {
			c.shutdown(errors.New("cast device closed the connection"))
		}
	case nsReceiver:
		var msg receiverMessage
		if json.Unmarshal([]byte(m.Payload), &msg) != nil {
			return
		}
		if msg.RequestID != 0 {
			c.deliverReply(msg)
			return
		}
		if msg.Type == "RECEIVER_STATUS" {
			c.publishStatus(msg.Status)
		}
	default:
		select {
		case c.appMessages <- m:
		default:
			// Nobody is reading app messages fast enough; dropping is safe
			// because the keeper resends the full state periodically.
		}
	}
}

func (c *Client) deliverReply(msg receiverMessage) {
	c.pendingMu.Lock()
	reply, ok := c.pending[msg.RequestID]
	c.pendingMu.Unlock()
	if ok {
		reply <- msg
	}
}

func (c *Client) publishStatus(status ReceiverStatus) {
	select {
	case <-c.statusUpdates:
	default:
	}
	c.statusUpdates <- status
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.sendJSON(receiverID, nsHeartbeat, map[string]any{"type": "PING"}); err != nil {
				return
			}
		}
	}
}

func (c *Client) shutdown(err error) {
	c.closeOnce.Do(func() {
		c.err = err
		close(c.done)
		c.conn.Close()
	})
}

func (c *Client) closedErr() error {
	if c.err != nil {
		return c.err
	}
	return errors.New("cast connection closed")
}
