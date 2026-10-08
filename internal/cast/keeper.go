package cast

import (
	"context"
	"encoding/json"
	"errors"
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
	// Messages is for receivers that keep listening: it returns the current
	// message at once, then every change. The channel closes when ctx ends.
	// Set Messages or LaunchMessage, not both.
	Messages func(ctx context.Context) <-chan []byte
	// LaunchMessage is for receivers that handle only the first message
	// after they start, such as DashCast, which replaces itself with the
	// page it opens. It is called after every launch, so the message can
	// carry a fresh single-use token. localIP is an address of this server
	// that the device can reach. A receiver of this kind that is found
	// already running is restarted, because it would ignore the message.
	LaunchMessage func(localIP string) ([]byte, error)
	// Active reports whether the display is in its active hours. Outside
	// them the keeper stops the receiver and does not launch it, so the TV
	// can go to standby. Nil means always active. It is checked at every
	// status poll, so a change takes effect within 30 seconds.
	Active func() bool
	// Refresh returns how often the receiver is stopped and started again,
	// or 0 for never. Google TV covers a running cast with its screensaver
	// after about 19 minutes while still reporting the app as running; on
	// real hardware a restart brought the dashboard back.
	Refresh func(Device) time.Duration
	// Status, if set, receives every state the keeper finds, for the web
	// interface. It must not block.
	Status func(State, error)
	Logger *slog.Logger
}

// Run keeps the receiver alive until ctx is cancelled.
func (k *Keeper) Run(ctx context.Context) {
	delay := minRetryDelay
	k.report(StateConnecting, nil)
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
		k.report(StateUnreachable, err)
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
	s := &keeperSession{
		keeper:  k,
		client:  client,
		localIP: ReachableIP(device.Addr, client.LocalIP()),
		resend:  time.NewTimer(0),
	}
	if k.Messages != nil {
		s.messages = k.Messages(sessionCtx)
	}
	<-s.resend.C
	defer s.resend.Stop()

	if err := s.syncStatus(ctx); err != nil {
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
		if err := s.step(ctx, poll.C, refresh); err != nil {
			return err
		}
	}
}

// step waits for the next event of the session and handles it.
func (s *keeperSession) step(ctx context.Context, poll, refresh <-chan time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.client.Done():
		return s.client.Err()
	case msg, ok := <-s.messages:
		if !ok {
			return errors.New("receiver message stream closed")
		}
		s.latest = msg
		return s.sendLatest()
	case <-s.client.StatusUpdates():
		// An unsolicited update can be stale: after a STOP the device
		// reports "no apps" even when the keeper has already launched
		// again. On real hardware that caused a second launch. The update
		// is only a trigger to ask for the current state.
		return s.syncStatus(ctx)
	case m := <-s.client.AppMessages():
		if m.Namespace == s.keeper.Namespace && isReadySignal(m.Payload) {
			return s.sendLatest()
		}
		return nil
	case <-refresh:
		return s.restart(ctx)
	case <-s.resend.C:
		s.scheduleNextResend()
		return s.sendLatest()
	case <-poll:
		if err := s.syncStatus(ctx); err != nil {
			return err
		}
		return s.sendLatest()
	}
}

// syncStatus asks the device for its current state and reconciles.
func (s *keeperSession) syncStatus(ctx context.Context) error {
	status, err := s.client.Status(ctx)
	if err != nil {
		return err
	}
	return s.reconcile(ctx, status)
}

type keeperSession struct {
	keeper    *Keeper
	client    *Client
	messages  <-chan []byte
	localIP   string
	app       Application
	transport string
	standby   bool
	latest    []byte
	resend    *time.Timer
	resendIdx int
}

// rest stops the receiver outside active hours. It stops it only while it
// is the keeper's own app, never another app on the TV.
func (s *keeperSession) rest(ctx context.Context, app Application, running bool) error {
	s.transport = ""
	if !running {
		return nil
	}
	s.keeper.Logger.Info("outside active hours, stopping receiver")
	return s.client.Stop(ctx, app.SessionID)
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
	if k.Active != nil && !k.Active() {
		k.report(StateResting, nil)
		return s.rest(ctx, app, running)
	}
	if running && k.LaunchMessage != nil && s.transport == "" {
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
			k.report(StateOtherApp, nil)
			return nil
		}
		k.Logger.Info("launching receiver", "app_id", k.AppID)
		launched, err := s.client.Launch(ctx, k.AppID)
		if err != nil {
			return err
		}
		app = launched
	}
	s.reportShowing()
	if app.TransportID == s.transport {
		return nil
	}
	if err := s.client.Connect(app.TransportID); err != nil {
		return err
	}
	s.app = app
	s.transport = app.TransportID
	if k.LaunchMessage != nil {
		msg, err := k.LaunchMessage(s.localIP)
		if err != nil {
			return fmt.Errorf("build launch message: %w", err)
		}
		s.latest = msg
	}
	s.resendIdx = 0
	s.scheduleNextResend()
	return s.sendLatest()
}

// reportShowing reports that the receiver runs, or that the TV sleeps.
func (s *keeperSession) reportShowing() {
	if s.standby {
		s.keeper.report(StateStandby, nil)
	} else {
		s.keeper.report(StateShowing, nil)
	}
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
