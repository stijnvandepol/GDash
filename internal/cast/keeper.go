package cast

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"
)

const (
	// statusPollInterval catches state changes the device does not report
	// on its own, and doubles as a periodic resend of the latest state.
	statusPollInterval = 30 * time.Second
	minRetryDelay      = time.Second
	maxRetryDelay      = time.Minute
	// A session that lasted this long was healthy, so the next failure
	// starts again at the minimum retry delay.
	healthySessionLength = time.Minute
)

// startupResends are the delays, after connecting to a freshly launched
// receiver, at which the latest message is sent again. A receiver page
// that is still loading drops messages; on real hardware the first one
// often arrived too early.
var startupResends = []time.Duration{2 * time.Second, 5 * time.Second}

// Keeper keeps a receiver app on screen on one Cast device and sends it
// messages. It reconnects with backoff and relaunches the app when it
// disappears, within the limits of ShouldRelaunch.
type Keeper struct {
	// Resolve finds the device. Looking it up for every session survives an
	// IP change after a router restart.
	Resolve   func(ctx context.Context) (Device, error)
	AppID     string
	Namespace string
	// Messages returns the messages for the receiver: the current one at
	// once, then every change. localIP is an address of this server that
	// the device can reach, for receivers that need a URL back to it.
	// The channel closes when ctx ends.
	Messages func(ctx context.Context, localIP string) <-chan []byte
	// Refresh returns how often the receiver is stopped and started again,
	// or 0 for never. Google TV covers a running cast with its screensaver
	// after about 19 minutes while still reporting the app as running; on
	// real hardware a restart brought the dashboard back.
	Refresh func(Device) time.Duration
	// FirstMessageOnly marks a receiver that handles only the first message
	// after it starts. DashCast replaces itself with the page it opens, so
	// a DashCast session found already running cannot take a new URL and
	// is restarted instead.
	FirstMessageOnly bool
	Logger           *slog.Logger
}

// Run keeps the receiver alive until ctx is cancelled.
func (k *Keeper) Run(ctx context.Context) {
	delay := minRetryDelay
	for {
		started := time.Now()
		err := k.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > healthySessionLength {
			delay = minRetryDelay
		}
		k.Logger.Warn("cast session ended, retrying", "retry_in", delay, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

func (k *Keeper) session(ctx context.Context) error {
	device, err := k.Resolve(ctx)
	if err != nil {
		return err
	}
	client, err := Dial(ctx, device.Addr)
	if err != nil {
		return err
	}
	defer client.Close()

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	messages := k.Messages(sessionCtx, ReachableIP(device.Addr, client.LocalIP()))

	s := &keeperSession{keeper: k, client: client, resend: time.NewTimer(0)}
	<-s.resend.C
	defer s.resend.Stop()

	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if err := s.reconcile(ctx, status); err != nil {
		return err
	}

	poll := time.NewTicker(statusPollInterval)
	defer poll.Stop()
	var refresh <-chan time.Time
	if every := k.refreshInterval(device); every > 0 {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		refresh = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-client.Done():
			return client.Err()
		case msg, ok := <-messages:
			if !ok {
				return fmt.Errorf("receiver message stream closed")
			}
			s.latest = msg
			if err := s.sendLatest(); err != nil {
				return err
			}
		case <-client.StatusUpdates():
			// An unsolicited update can be stale: after a STOP the device
			// reports "no apps" even when the keeper has already launched
			// again. On real hardware that caused a second launch. The
			// update is only a trigger to ask for the current state.
			status, err := client.Status(ctx)
			if err != nil {
				return err
			}
			if err := s.reconcile(ctx, status); err != nil {
				return err
			}
		case m := <-client.AppMessages():
			if m.Namespace == k.Namespace && isReadySignal(m.Payload) {
				if err := s.sendLatest(); err != nil {
					return err
				}
			}
		case <-refresh:
			if err := s.restart(ctx); err != nil {
				return err
			}
		case <-s.resend.C:
			if err := s.sendLatest(); err != nil {
				return err
			}
			s.scheduleNextResend()
		case <-poll.C:
			status, err := client.Status(ctx)
			if err != nil {
				return err
			}
			if err := s.reconcile(ctx, status); err != nil {
				return err
			}
			if err := s.sendLatest(); err != nil {
				return err
			}
		}
	}
}

type keeperSession struct {
	keeper    *Keeper
	client    *Client
	app       Application
	transport string
	standby   bool
	latest    []byte
	resend    *time.Timer
	resendIdx int
}

func (k *Keeper) refreshInterval(d Device) time.Duration {
	if k.Refresh == nil {
		return 0
	}
	return k.Refresh(d)
}

// restart stops the receiver and launches it again. It does nothing while
// another app is on screen, and nothing while the TV is in standby, because
// launching would switch the TV on through HDMI-CEC.
func (s *keeperSession) restart(ctx context.Context) error {
	if s.transport == "" || s.standby {
		return nil
	}
	s.keeper.Logger.Info("restarting receiver to keep it in front of the screensaver")
	if err := s.client.Stop(ctx, s.app.SessionID); err != nil {
		return err
	}
	s.transport = ""
	return s.reconcile(ctx, ReceiverStatus{})
}

// reconcile makes sure the receiver runs and is connected, launching it when
// ShouldRelaunch allows that.
func (s *keeperSession) reconcile(ctx context.Context, status ReceiverStatus) error {
	k := s.keeper
	s.standby = status.IsStandBy
	app, running := status.App(k.AppID)
	if running && k.FirstMessageOnly && s.transport == "" {
		k.Logger.Info("restarting receiver that was already running, so it takes the current page")
		if err := s.client.Stop(ctx, app.SessionID); err != nil {
			return err
		}
		running = false
		status = ReceiverStatus{}
	}
	if !running {
		s.transport = ""
		if !ShouldRelaunch(status) {
			return nil
		}
		k.Logger.Info("launching receiver", "app_id", k.AppID)
		launched, err := s.client.Launch(ctx, k.AppID)
		if err != nil {
			return err
		}
		app = launched
	}
	if app.TransportID == s.transport {
		return nil
	}
	if err := s.client.Connect(app.TransportID); err != nil {
		return err
	}
	s.app = app
	s.transport = app.TransportID
	s.resendIdx = 0
	s.scheduleNextResend()
	return s.sendLatest()
}

func (s *keeperSession) scheduleNextResend() {
	if s.resendIdx >= len(startupResends) {
		return
	}
	previous := time.Duration(0)
	if s.resendIdx > 0 {
		previous = startupResends[s.resendIdx-1]
	}
	s.resend.Reset(startupResends[s.resendIdx] - previous)
	s.resendIdx++
}

func (s *keeperSession) sendLatest() error {
	if s.transport == "" || s.latest == nil {
		return nil
	}
	return s.client.Send(s.transport, s.keeper.Namespace, s.latest)
}

// ShouldRelaunch decides whether the keeper may start Daycast on a device
// that is not showing it. Starting an app on a Cast device takes over the
// screen, and through HDMI-CEC it can also switch the TV on.
//
// Current policy: only take over when nothing else is on screen, so the
// keeper never interrupts someone who casts YouTube to the kitchen TV.
// Known gap: Google TV devices report no apps while a native app such as
// Netflix runs, so the keeper would start over it.
func ShouldRelaunch(status ReceiverStatus) bool {
	for _, app := range status.Applications {
		if !app.IsIdleScreen {
			return false
		}
	}
	return true
}

func isReadySignal(payload string) bool {
	var msg struct{ Type string }
	return json.Unmarshal([]byte(payload), &msg) == nil && msg.Type == "ready"
}

// LocalIP returns this machine's address on the connection to the device:
// the address the device can use to reach Daycast.
func (c *Client) LocalIP() string {
	if addr, ok := c.conn.LocalAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return ""
}
