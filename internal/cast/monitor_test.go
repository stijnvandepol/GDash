package cast

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var spotifyApp = Application{
	AppID: "CC32E753", DisplayName: "Spotify", SessionID: "spotify-session", TransportID: "spotify-transport",
	StatusText: "Spotify", Namespaces: []Namespace{{Name: nsMedia}, {Name: "urn:x-cast:com.spotify.chromecast.secure.v1"}},
}

var youtubeApp = Application{
	AppID: "233637DE", DisplayName: "YouTube", SessionID: "youtube-session", TransportID: "youtube-transport",
	Namespaces: []Namespace{{Name: nsMedia}},
}

// statusList returns the "status" list of a fixture.
func statusList(t *testing.T, fixture string) string {
	t.Helper()
	payload := readFixture(t, fixture)
	start := strings.Index(payload, `"status": [`)
	end := strings.LastIndex(payload, `],`)
	if start < 0 || end < start {
		t.Fatalf("%s has no status list", fixture)
	}
	return payload[start+len(`"status": `) : end+1]
}

// runMonitor follows device until the test ends and returns what it reports
// and how often it resolved the device.
func runMonitor(t *testing.T, ctx context.Context, device *fakeDevice) (<-chan Playing, *atomic.Int32) {
	t.Helper()
	reports := make(chan Playing, 64)
	var resolved atomic.Int32
	m := &Monitor{
		Resolve: func(context.Context) (Device, error) {
			resolved.Add(1)
			return Device{ID: "speaker", Addr: device.addr()}, nil
		},
		Report: func(p Playing) { reports <- p },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Run(runCtx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return reports, &resolved
}

// nextReport waits for a report that matches.
func nextReport(t *testing.T, ctx context.Context, reports <-chan Playing, what string, match func(Playing) bool) Playing {
	t.Helper()
	for {
		select {
		case p := <-reports:
			if match(p) {
				return p
			}
		case <-ctx.Done():
			t.Fatalf("never reported %s", what)
		}
	}
}

func TestMonitorReportsWhatAMediaAppPlays(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	device.play(spotifyApp, statusList(t, "media-spotify.json"))
	before := time.Now()
	reports, _ := runMonitor(t, ctx, device)

	got := nextReport(t, ctx, reports, "the Spotify track", func(p Playing) bool { return p.State == PlayerPlaying })
	if got.App != "Spotify" || got.Title != "One More Time" || got.Artist != "Daft Punk" || got.Album != "Discovery" || got.Position != 63.514 {
		t.Fatalf("reported %+v", got)
	}
	if got.At.Before(before) {
		t.Fatalf("At = %v, before the monitor started", got.At)
	}

	// The app reports a pause on its own, without the media.
	device.pushMedia(spotifyApp.TransportID, `[{"mediaSessionId":1,"playerState":"PAUSED","currentTime":80}]`)
	got = nextReport(t, ctx, reports, "the pause", func(p Playing) bool { return p.State == PlayerPaused })
	if got.Title != "One More Time" || got.Position != 80 {
		t.Fatalf("after the pause reported %+v", got)
	}

	// Another sender casts YouTube: the monitor follows the new app.
	device.play(youtubeApp, statusList(t, "media-youtube.json"))
	got = nextReport(t, ctx, reports, "the YouTube video", func(p Playing) bool { return p.App == "YouTube" && p.State != "" })
	if got.Artist != "Blender" || got.State != PlayerPaused {
		t.Fatalf("reported %+v", got)
	}

	device.closeApps()
	nextReport(t, ctx, reports, "that nothing plays", func(p Playing) bool { return p == Playing{} })
}

func TestMonitorNeverControlsTheDevice(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	reports, _ := runMonitor(t, ctx, device)

	device.play(spotifyApp, statusList(t, "media-spotify.json"))
	nextReport(t, ctx, reports, "the track", func(p Playing) bool { return p.State == PlayerPlaying })
	device.pushMedia(spotifyApp.TransportID, `[{"playerState":"IDLE","idleReason":"FINISHED"}]`)
	nextReport(t, ctx, reports, "the end of the track", func(p Playing) bool { return p.State == PlayerIdle })
	device.closeApps()
	nextReport(t, ctx, reports, "that nothing plays", func(p Playing) bool { return p == Playing{} })
	device.setStandby(true, false)

	if n := device.launchCount(); n != 0 {
		t.Fatalf("the monitor launched %d apps", n)
	}
	if n := device.stopCount(); n != 0 {
		t.Fatalf("the monitor stopped %d sessions", n)
	}
	for _, kind := range device.mediaCommandsSent() {
		if kind != "GET_STATUS" {
			t.Fatalf("the monitor sent media command %q", kind)
		}
	}
	select {
	case m := <-device.appMessages:
		t.Fatalf("the monitor sent an app %+v", m)
	default:
	}
}

func TestMonitorLeavesIdleScreensAndOtherAppsAlone(t *testing.T) {
	ctx := testContext(t)
	backdrop := Application{AppID: "E8C28D3C", DisplayName: "Backdrop", TransportID: "backdrop", IsIdleScreen: true, Namespaces: []Namespace{{Name: nsMedia}}}
	game := Application{AppID: "12345678", DisplayName: "A game", TransportID: "game", Namespaces: []Namespace{{Name: "urn:x-cast:com.example.game"}}}
	device := newFakeDevice(t, backdrop, game)
	reports, _ := runMonitor(t, ctx, device)

	// The monitor polled once: wait for the device to have answered it.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case p := <-reports:
			t.Fatalf("reported %+v for a device without media", p)
		case <-time.After(20 * time.Millisecond):
		}
	}
	if sent := device.mediaCommandsSent(); len(sent) != 0 {
		t.Fatalf("asked an app without media for its status: %v", sent)
	}
}

func TestMonitorReconnectsAfterTheDeviceDropsIt(t *testing.T) {
	ctx := testContext(t)
	device := newFakeDevice(t)
	device.play(spotifyApp, statusList(t, "media-spotify.json"))
	reports, resolved := runMonitor(t, ctx, device)
	nextReport(t, ctx, reports, "the track", func(p Playing) bool { return p.State == PlayerPlaying })

	device.dropConnections()
	nextReport(t, ctx, reports, "that nothing is known", func(p Playing) bool { return p == Playing{} })
	got := nextReport(t, ctx, reports, "the track again", func(p Playing) bool { return p.State == PlayerPlaying })
	if got.Title != "One More Time" {
		t.Fatalf("after reconnecting reported %+v", got)
	}
	if n := resolved.Load(); n < 2 {
		t.Fatalf("resolved the device %d times, want a new lookup for the new connection", n)
	}
}
