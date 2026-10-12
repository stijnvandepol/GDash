package cast

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// startupResends are the delays, after connecting to a freshly launched
// receiver, at which the latest message is sent again. A receiver page
// that is still loading drops messages; on real hardware the first one
// often arrived too early.
var startupResends = []time.Duration{2 * time.Second, 5 * time.Second}

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
	case <-s.keeper.TakeOver:
		s.keeper.claim.force = true
		return s.syncStatus(ctx)
	case <-s.resend.C:
		last := s.resendIdx >= len(startupResends)
		s.scheduleNextResend()
		if err := s.sendLatest(); err != nil {
			return err
		}
		if last && s.latest != nil {
			s.keeper.claim.pageSent = true
		}
		return nil
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
	deviceIP  string
	app       Application
	transport string
	standby   bool
	// otherInput: the last status said the TV shows another input.
	otherInput bool
	// seen is set by the session's first status. Before it the keeper was
	// not connected, so it cannot tell what happened on the TV meanwhile.
	seen      bool
	latest    []byte
	resend    *time.Timer
	resendIdx int
}

// reconcile follows a fresh status of the device: it notices when someone
// took the TV from GDash, and launches the receiver when the screen is
// GDash's to take.
func (s *keeperSession) reconcile(ctx context.Context, status ReceiverStatus) error {
	k := s.keeper
	c := &k.claim
	fresh, afterGap := !c.started, !s.seen
	c.started, s.seen = true, true
	s.standby, s.otherInput = status.InStandby(), status.OnOtherInput()
	if s.standby {
		c.endYield()
	}
	// Only a change seen while connected says that someone took the TV. A
	// Chromecast powered by the TV's USB port is gone while the TV is off,
	// and its session with it.
	if reason := c.observe(status, k.AppID); reason != "" && !s.standby && !afterGap && !c.yielded {
		k.Logger.Info("someone else is using the TV; waiting until it is switched off", "reason", reason)
		c.yielded = true
	}
	if c.yielded && c.idleLongEnough(status, k.now()) {
		k.Logger.Info("the TV showed its idle screen long enough; it is free again", "after", idleYieldTimeout)
		c.yielded = false
	}
	app, running := status.App(k.AppID)
	holds := c.holds(app, running)
	if !holds {
		s.transport = ""
	}
	if k.Active != nil && !k.Active() {
		k.report(StateResting, nil)
		return s.rest(ctx, app, running)
	}
	switch {
	case c.force:
		return s.takeOver(ctx, app, running)
	case holds && c.yielded && !c.mayResume(status, k.AppID):
		k.report(StateYielded, nil)
		return nil
	case holds:
		c.yielded = false
		return s.keep(ctx, app)
	case s.standby:
		// Launching could switch the TV on through HDMI-CEC, in the middle
		// of the night. The first status after someone switches it on
		// launches the receiver.
		k.report(StateStandby, nil)
		return nil
	case c.yielded:
		k.report(StateYielded, nil)
		return nil
	}
	return s.claimScreen(ctx, status, app, running, fresh)
}

// claimScreen launches the receiver on a screen that GDash does not hold,
// when that screen is free; see ShouldRelaunch.
func (s *keeperSession) claimScreen(ctx context.Context, status ReceiverStatus, app Application, running, fresh bool) error {
	k := s.keeper
	c := &k.claim
	switch {
	case running && c.replace && k.LaunchMessage != nil:
		k.Logger.Info("restarting receiver that was already running, so it takes the current page")
		if err := s.stop(ctx, app.SessionID); err != nil {
			return err
		}
		return s.launch(ctx)
	case running && c.replace:
		c.launched(app.SessionID, s.otherInput)
		return s.keep(ctx, app)
	case fresh && (running || status.hasOtherApp(k.AppID)):
		// Started with the server: GDash cannot tell whether it launched
		// what runs before the restart, or someone else did.
		k.Logger.Info("the TV is in use; waiting until it is switched off")
		c.yielded = true
		k.report(StateYielded, nil)
		return nil
	case running || !ShouldRelaunch(status):
		k.report(StateOtherApp, nil)
		return nil
	}
	return s.launch(ctx)
}

// takeOver puts the receiver on screen at an admin's request, over whatever
// runs. Launching also makes the TV switch to the Cast device's input
// through HDMI-CEC.
func (s *keeperSession) takeOver(ctx context.Context, app Application, running bool) error {
	s.keeper.Logger.Info("taking the TV back at an admin's request")
	if running {
		if err := s.stop(ctx, app.SessionID); err != nil {
			return err
		}
	}
	return s.launch(ctx)
}

// rest stops the receiver outside active hours, so the TV can go to
// standby. It never stops another app, nor a session of the receiver that
// someone else started after GDash yielded to them.
func (s *keeperSession) rest(ctx context.Context, app Application, running bool) error {
	c := &s.keeper.claim
	c.force = false
	if !running || (c.yielded && !c.holds(app, running)) {
		return nil
	}
	s.keeper.Logger.Info("outside active hours, stopping receiver")
	return s.stop(ctx, app.SessionID)
}

// restart stops the receiver and launches it again, against the Google TV
// screensaver. It asks the device first, because the last status poll can
// be 30 seconds old, and restarts only GDash's own session while nothing
// else is on the TV (see claim.mayRestart): launching while the TV sleeps
// would switch it on through HDMI-CEC.
func (s *keeperSession) restart(ctx context.Context) error {
	if s.transport == "" {
		return nil
	}
	status, err := s.client.Status(ctx)
	if err != nil {
		return err
	}
	if err := s.reconcile(ctx, status); err != nil {
		return err
	}
	k := s.keeper
	if s.transport == "" || !k.claim.mayRestart(status, k.AppID) {
		return nil
	}
	k.Logger.Info("restarting receiver to keep it in front of the screensaver")
	if err := s.stop(ctx, k.claim.session); err != nil {
		return err
	}
	return s.launch(ctx)
}

// stop ends an app session on the device. A session GDash holds is marked
// first, so that its disappearance does not count as someone taking the
// TV, also when the reply to STOP is lost.
func (s *keeperSession) stop(ctx context.Context, sessionID string) error {
	c := &s.keeper.claim
	if sessionID == c.session {
		c.stopping = sessionID
	}
	if err := s.client.Stop(ctx, sessionID); err != nil {
		return err
	}
	if sessionID == c.session {
		c.session, c.stopping = "", ""
	}
	s.transport = ""
	return nil
}

// launch starts the receiver and connects to it.
func (s *keeperSession) launch(ctx context.Context) error {
	k := s.keeper
	k.Logger.Info("launching receiver", "app_id", k.AppID)
	app, err := s.client.Launch(ctx, k.AppID)
	if err != nil {
		return err
	}
	k.claim.launched(app.SessionID, s.otherInput)
	return s.connect(app, true)
}

// keep stays with GDash's own session. A receiver that takes only its first
// message is restarted when that message may not have arrived, because it
// would ignore a new one.
func (s *keeperSession) keep(ctx context.Context, app Application) error {
	k := s.keeper
	if k.LaunchMessage != nil && !k.claim.pageSent && app.TransportID != s.transport && !s.standby {
		k.Logger.Info("restarting receiver that may not have received its page")
		if err := s.stop(ctx, app.SessionID); err != nil {
			return err
		}
		return s.launch(ctx)
	}
	return s.connect(app, false)
}

// connect opens a connection to app, unless it is open already, and sends
// it the latest message. After a launch, a receiver that takes only its
// first message gets a fresh one.
func (s *keeperSession) connect(app Application, launched bool) error {
	k := s.keeper
	if app.TransportID == s.transport {
		s.reportShowing()
		return nil
	}
	if err := s.client.Connect(app.TransportID); err != nil {
		return err
	}
	s.app = app
	s.transport = app.TransportID
	s.reportShowing()
	if launched && k.LaunchMessage != nil {
		msg, err := k.LaunchMessage(s.localIP, s.deviceIP)
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
