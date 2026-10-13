package cast

import (
	"context"
	"log/slog"
	"time"
)

// Playing is what a Cast device plays, as a Monitor last saw it. The zero
// value means nothing: no media app runs, or the device is unreachable.
type Playing struct {
	// App is the name of the media app, such as "Spotify".
	App string
	Media
	// At is when Position was read; a playing track has moved on since.
	At time.Time
}

// Monitor follows what one Cast device plays, such as a speaker, a speaker
// group or a Nest Hub. It only reads: it asks the device which apps run,
// and the media app what it plays, and follows the updates both send on
// their own. It never launches or stops an app and never sends a media
// command, so it cannot interrupt anyone. It reads the media status the
// way pychromecast's MediaController does: connect to the media app's
// transport, send GET_STATUS on the media namespace, then listen.
type Monitor struct {
	// Resolve finds the device, for every connection, so an IP address
	// that changed after a router restart is found again.
	Resolve func(ctx context.Context) (Device, error)
	// Report receives what the device plays after every change, and the
	// zero Playing when the connection is lost. It must not block.
	Report func(Playing)
	Logger *slog.Logger
}

// Run follows the device until ctx ends, reconnecting with backoff.
func (m *Monitor) Run(ctx context.Context) {
	delay := minRetryDelay
	for {
		started := time.Now()
		err := m.session(ctx)
		if ctx.Err() != nil {
			return
		}
		// While the device is away nothing is known to play on it.
		m.Report(Playing{})
		if time.Since(started) > healthySessionLength {
			delay = minRetryDelay
		}
		m.Logger.Info("lost the Cast device whose media GDash follows, retrying", "retry_in", delay, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

func (m *Monitor) session(ctx context.Context) error {
	device, err := m.Resolve(ctx)
	if err != nil {
		return err
	}
	client, err := Dial(ctx, device.Addr)
	if err != nil {
		return err
	}
	defer client.Close()

	w := &mediaWatch{client: client, report: m.Report}
	if err := w.poll(ctx); err != nil {
		return err
	}
	poll := time.NewTicker(statusPollInterval)
	defer poll.Stop()
	for {
		var err error
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-client.Done():
			return client.Err()
		case status := <-client.StatusUpdates():
			err = w.follow(status)
		case msg := <-client.AppMessages():
			w.media(msg)
		case <-poll.C:
			err = w.poll(ctx)
		}
		if err != nil {
			return err
		}
	}
}

// mediaWatch is one connection of a Monitor.
type mediaWatch struct {
	client *Client
	report func(Playing)
	// transport is the media app the watch is connected to, or "".
	transport string
	playing   Playing
}

// poll asks the device for its apps and, when a media app runs, that app
// for its status: a missed update is caught within one poll.
func (w *mediaWatch) poll(ctx context.Context) error {
	status, err := w.client.Status(ctx)
	if err != nil {
		return err
	}
	previous := w.transport
	if err := w.follow(status); err != nil {
		return err
	}
	if w.transport != "" && w.transport == previous {
		return w.client.askMediaStatus(w.transport)
	}
	return nil
}

// follow takes a receiver status: it connects to a media app that started
// and forgets one that stopped.
func (w *mediaWatch) follow(status ReceiverStatus) error {
	app, ok := mediaApp(status)
	if !ok {
		if w.transport != "" || w.playing != (Playing{}) {
			w.transport, w.playing = "", Playing{}
			w.report(Playing{})
		}
		return nil
	}
	if app.TransportID == w.transport {
		return nil
	}
	// Until the new app answers, what the old one played is gone.
	w.transport = app.TransportID
	w.playing = Playing{App: clip(app.DisplayName)}
	w.report(w.playing)
	if err := w.client.Connect(app.TransportID); err != nil {
		return err
	}
	return w.client.askMediaStatus(app.TransportID)
}

// media takes a message from an app; only a MEDIA_STATUS of the app the
// watch follows counts.
func (w *mediaWatch) media(msg Message) {
	if msg.Namespace != nsMedia || msg.SourceID != w.transport || w.transport == "" {
		return
	}
	media, ok := applyMediaStatus(msg.Payload, w.playing.Media)
	if !ok {
		return
	}
	w.playing.Media = media
	w.playing.At = time.Now()
	w.report(w.playing)
}

// mediaApp returns the app that plays media on the device, if any. The
// idle screen (Backdrop) is never one, even when it speaks the namespace.
func mediaApp(status ReceiverStatus) (Application, bool) {
	for _, app := range status.Applications {
		if !app.IsIdleScreen && app.speaks(nsMedia) && app.TransportID != "" {
			return app, true
		}
	}
	return Application{}, false
}
