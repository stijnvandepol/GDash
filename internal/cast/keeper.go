package cast

import (
	"context"
	"encoding/json"
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
	// ownPageGoneGap is how long after a launch a report that the page is
	// hidden is taken for the previous page, which a restart stopped, and
	// not for someone opening an app over the new one.
	ownPageGoneGap = 30 * time.Second
)

// RestartPeriodic is why the keeper restarts a receiver that runs, for the
// log and the web interface: the Refresh interval passed.
const RestartPeriodic = "periodic"

// Keeper keeps a receiver app on screen on one Cast device and sends it
// messages. It reconnects with backoff and launches the app on a free
// screen (see ShouldRelaunch). When someone takes the TV from it, it waits
// until the TV has been in standby (see claim).
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
	// that the device can reach, deviceIP the device's own address. A
	// receiver of this kind that is found already running is restarted,
	// because it would ignore the message.
	LaunchMessage func(localIP, deviceIP string) ([]byte, error)
	// Active reports whether the display is in its active hours. Outside
	// them the keeper stops the receiver and does not launch it, so the TV
	// can go to standby. Nil means always active. It is checked at every
	// status poll, so a change takes effect within 30 seconds.
	Active func() bool
	// Refresh returns how often the receiver is stopped and started again,
	// or 0 for never. Google TV covers a running cast with its screensaver
	// after about 19 minutes while still reporting the app as running; on
	// real hardware a restart brought the dashboard back. It is read again
	// with every status, so a changed setting applies within 30 seconds.
	Refresh func(Device) time.Duration
	// Hidden delivers a report of the page on the TV that it is hidden,
	// such as by Netflix opened on a Google TV, which still reports the
	// receiver as running. The keeper then yields as when someone takes the
	// TV, and stops the periodic restart that would put the dashboard back
	// over the app, until Shown, standby or TakeOver. A report within
	// ownPageGoneGap of a launch is the previous page going away.
	Hidden <-chan struct{}
	// Shown delivers a report of the page on the TV that it is on screen
	// again, such as when the TV's screensaver ends; it ends a yield that
	// Hidden started.
	Shown <-chan struct{}
	// Restarted, if set, is called after every restart of a receiver that
	// ran, with RestartPeriodic. It must not block.
	Restarted func(reason string)
	// Status, if set, receives every state the keeper finds, for the web
	// interface. It must not block.
	Status func(State, error)
	// TakeOver, if set, delivers an admin's request to show the receiver
	// now: it ends a yield and launches over whatever is on the TV, unless
	// the TV is in standby or the display is outside its active hours.
	TakeOver <-chan struct{}
	// Now is the clock for how long the device shows its idle screen
	// during a yield; nil means time.Now.
	Now func() time.Time
	// Recall and Remember keep the session GDash launched across a server
	// restart, such as an update: a keeper that starts and finds that very
	// session on the TV takes it back instead of yielding to it. Recall
	// returns the last remembered session ID, Remember stores a change, ""
	// when GDash holds nothing. Both may be nil.
	Recall   func() string
	Remember func(sessionID string)
	Logger   *slog.Logger

	// claim is who controls the screen. Only the Run goroutine uses it.
	claim claim
	// remembered is the session last passed to Remember.
	remembered string
	// refreshFrom is when the periodic restart last launched or tried to,
	// launchedAt when the keeper last launched the receiver. They outlast a
	// connection; only the Run goroutine uses them.
	refreshFrom time.Time
	launchedAt  time.Time
}

// rememberSession passes the session GDash holds to Remember when it
// changed. A receiver that takes only its first message counts once its
// page was sent, so a restart never adopts an empty receiver.
func (k *Keeper) rememberSession() {
	if k.Remember == nil {
		return
	}
	session := k.claim.session
	if k.LaunchMessage != nil && !k.claim.pageSent {
		session = ""
	}
	if session != k.remembered {
		k.remembered = session
		k.Remember(session)
	}
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
		keeper:       k,
		client:       client,
		device:       device,
		localIP:      ReachableIP(device.Addr, client.LocalIP()),
		deviceIP:     hostOf(device.Addr),
		resend:       time.NewTimer(0),
		refresh:      time.NewTimer(time.Hour),
		refreshEvery: k.refreshInterval(device),
	}
	// The periodic restart counts on across a reconnect: a connection that
	// drops more often than the interval must not put it off for good. A
	// keeper that has not launched yet counts from now.
	if k.refreshFrom.IsZero() {
		k.refreshFrom = k.now()
	}
	if k.Messages != nil {
		s.messages = k.Messages(sessionCtx)
	}
	<-s.resend.C
	defer s.resend.Stop()
	s.armRefresh()
	defer s.refresh.Stop()

	if err := s.syncStatus(ctx); err != nil {
		return err
	}
	poll := time.NewTicker(statusPollInterval)
	defer poll.Stop()
	for {
		if err := s.step(ctx, poll.C); err != nil {
			return err
		}
	}
}

func (k *Keeper) now() time.Time {
	if k.Now == nil {
		return time.Now()
	}
	return k.Now()
}

func (k *Keeper) refreshInterval(d Device) time.Duration {
	if k.Refresh == nil {
		return 0
	}
	return k.Refresh(d)
}

// ShouldRelaunch decides whether the screen of a device that is not showing
// GDash is free: only its idle screen runs, and the TV does not report
// another input, such as a game console. Starting an app on a Cast device
// takes over the screen, and through HDMI-CEC it can switch the TV on or
// to the Cast device's input.
//
// Known gap: Google TV reports no apps while a native app such as Netflix
// runs, so a Google TV that is free here may be in use. The keeper only
// avoids that while it holds the screen: when its own app disappears, it
// waits until the TV has been in standby (see claim).
func ShouldRelaunch(status ReceiverStatus) bool {
	for _, app := range status.Applications {
		if !app.IsIdleScreen {
			return false
		}
	}
	return !status.OnOtherInput()
}

func isReadySignal(payload string) bool {
	var msg struct{ Type string }
	return json.Unmarshal([]byte(payload), &msg) == nil && msg.Type == "ready"
}

// LocalIP returns this machine's address on the connection to the device:
// the address the device can use to reach GDash.
func (c *Client) LocalIP() string {
	if addr, ok := c.conn.LocalAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// hostOf returns the host part of a host:port address, or addr itself.
func hostOf(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
