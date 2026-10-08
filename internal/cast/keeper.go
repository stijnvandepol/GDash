package cast

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

// Keeper keeps a Daycast receiver on screen on one Cast device and forwards
// display messages to it. It reconnects with backoff and relaunches the
// receiver when it disappears, within the limits of shouldRelaunch.
type Keeper struct {
	Addr      string
	AppID     string
	Namespace string
	// Subscribe returns a channel with display messages. The first message
	// must be the current state; the channel closes when ctx ends.
	Subscribe func(ctx context.Context) <-chan []byte
	Logger    *slog.Logger
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
		k.Logger.Warn("cast session ended, retrying", "device", k.Addr, "retry_in", delay, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

func (k *Keeper) session(ctx context.Context) error {
	client, err := Dial(ctx, k.Addr)
	if err != nil {
		return err
	}
	defer client.Close()

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	messages := k.Subscribe(sessionCtx)

	s := &keeperSession{keeper: k, client: client}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if err := s.reconcile(ctx, status); err != nil {
		return err
	}

	poll := time.NewTicker(statusPollInterval)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-client.Done():
			return client.Err()
		case msg, ok := <-messages:
			if !ok {
				return fmt.Errorf("display message stream closed")
			}
			s.latest = msg
			if err := s.sendLatest(); err != nil {
				return err
			}
		case status := <-client.StatusUpdates():
			if err := s.reconcile(ctx, status); err != nil {
				return err
			}
		case m := <-client.AppMessages():
			if m.Namespace == k.Namespace && isReadySignal(m.Payload) {
				if err := s.sendLatest(); err != nil {
					return err
				}
			}
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
	transport string
	latest    []byte
}

// reconcile makes sure the receiver runs and is connected, launching it when
// shouldRelaunch allows that.
func (s *keeperSession) reconcile(ctx context.Context, status ReceiverStatus) error {
	k := s.keeper
	app, running := status.App(k.AppID)
	if !running {
		s.transport = ""
		if !shouldRelaunch(status) {
			return nil
		}
		k.Logger.Info("launching receiver", "device", k.Addr, "app_id", k.AppID)
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
	s.transport = app.TransportID
	return s.sendLatest()
}

func (s *keeperSession) sendLatest() error {
	if s.transport == "" || s.latest == nil {
		return nil
	}
	return s.client.Send(s.transport, s.keeper.Namespace, s.latest)
}

// shouldRelaunch decides whether the keeper may start Daycast on a device
// that is not showing it. Starting an app on a Cast device takes over the
// screen, and through HDMI-CEC it can also switch the TV on.
//
// Current policy: only take over when nothing else is on screen, so the
// keeper never interrupts someone who casts YouTube to the kitchen TV.
func shouldRelaunch(status ReceiverStatus) bool {
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
