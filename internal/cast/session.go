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
	deviceIP  string
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

// restart stops the receiver and launches it again. It does nothing while
// another app is on screen, and nothing while the TV is in standby, because
// launching would switch the TV on through HDMI-CEC. It asks the device
// first: the last status poll can be 30 seconds old.
func (s *keeperSession) restart(ctx context.Context) error {
	if s.transport == "" {
		return nil
	}
	if err := s.syncStatus(ctx); err != nil {
		return err
	}
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
	if status.IsStandBy && s.transport == "" {
		// The TV sleeps. Launching or restarting the receiver could switch
		// it on through HDMI-CEC, in the middle of the night after a server
		// restart. The next status poll launches it once someone switches
		// the TV on.
		k.report(StateStandby, nil)
		return nil
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
	if k.LaunchMessage != nil {
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
