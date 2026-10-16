package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/casting"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/store"
)

// newCastManager keeps the dashboard on every display that is linked to a
// Cast device. Without an own receiver app ID it uses DashCast, which opens
// the display page on the TV; with one it pushes snapshots to that receiver.
// castDeps are what the cast manager needs from the rest of the server.
type castDeps struct {
	db        *store.Store
	snapshots *display.Broadcaster
	pairings  *pairing.Service
	logger    *slog.Logger
}

func newCastManager(cfg config, deps castDeps) (*casting.Manager, error) {
	db, snapshots, pairings, logger := deps.db, deps.snapshots, deps.pairings, deps.logger
	_, port, err := net.SplitHostPort(cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("GDASH_ADDR %q has no port: %w", cfg.addr, err)
	}
	sessions := loadCastSessions(cfg.dataDir, logger)
	runDisplay := func(ctx context.Context, d displays.Display, keep casting.Keep) {
		keeper := &cast.Keeper{
			Resolve:   resolveDevice(d.CastDeviceID),
			Refresh:   refreshInterval(ctx, db, d.ID, logger),
			Active:    activeNow(ctx, db, d.ID, logger),
			Status:    keep.Report,
			Restarted: keep.Restarted,
			TakeOver:  keep.TakeOver,
			Hidden:    keep.Hidden,
			Shown:     keep.Shown,
			Recall:    func() string { return sessions.recall(d.ID) },
			Remember:  func(id string) { sessions.remember(d.ID, id) },
			Logger:    logger.With("display", d.ID),
		}
		if cfg.castAppID != "" {
			keeper.AppID, keeper.Namespace = cfg.castAppID, display.CastNamespace
			keeper.Messages = func(ctx context.Context) <-chan []byte { return snapshots.Subscribe(ctx, d.ID) }
		} else {
			keeper.AppID, keeper.Namespace = cast.DashCastAppID, cast.DashCastNamespace
			keeper.LaunchMessage = func(localIP, deviceIP string) ([]byte, error) {
				device, err := netip.ParseAddr(deviceIP)
				if err != nil {
					return nil, fmt.Errorf("cast device address %q: %w", deviceIP, err)
				}
				launchToken, err := pairings.NewLaunchToken(d.ID, device)
				if err != nil {
					return nil, err
				}
				return cast.DashCastLoad(displayURL(localIP, port, d.ID, launchToken)), nil
			}
		}
		keeper.Run(ctx)
	}
	return &casting.Manager{List: db.Displays, RunDisplay: runDisplay, Logger: logger}, nil
}

// activeNow reads the display's active hours on every call, so an edit in
// the web interface applies without restarting the keeper. When the display
// cannot be read the screen stays on: a database hiccup must not switch a TV
// off.
func activeNow(ctx context.Context, db *store.Store, displayID string, logger *slog.Logger) func() bool {
	return func() bool {
		d, err := db.Display(ctx, displayID)
		if err != nil {
			logger.Warn("could not read active hours; keeping the screen on", "display", displayID, "error", err)
			return true
		}
		return d.Hours.Contains(time.Now())
	}
}

// resolveDevice finds a Cast device by its device ID.
func resolveDevice(deviceID string) func(ctx context.Context) (cast.Device, error) {
	return func(ctx context.Context) (cast.Device, error) {
		devices, err := cast.Discover(ctx)
		if err != nil {
			return cast.Device{}, err
		}
		for _, d := range devices {
			if d.ID == deviceID {
				return d, nil
			}
		}
		return cast.Device{}, fmt.Errorf("%w: %s; is it switched on?", cast.ErrDeviceNotFound, deviceID)
	}
}

// Nest Hubs close a receiver without media after about 10 minutes, whatever
// the display's screensaver setting.
const nestHubRefreshInterval = 9 * time.Minute

// refreshInterval reads, on every call, how often the display's receiver is
// restarted to stay in front of the screensaver (displays.Display
// ScreensaverMinutes), so a change applies without restarting the keeper.
// When the display cannot be read the last value stays: a database hiccup
// must not turn the restarts off.
func refreshInterval(ctx context.Context, db *store.Store, displayID string, logger *slog.Logger) func(cast.Device) time.Duration {
	last := displays.DefaultScreensaverMinutes
	return func(device cast.Device) time.Duration {
		d, err := db.Display(ctx, displayID)
		if err != nil {
			logger.Warn("could not read the screensaver setting; keeping the last one", "display", displayID, "error", err)
		} else {
			last = d.ScreensaverMinutes
		}
		return deviceRefreshInterval(device, last)
	}
}

// deviceRefreshInterval is how often the receiver is restarted on a device
// for a setting in minutes, 0 being never. Each restart reloads the screen
// for a few seconds.
func deviceRefreshInterval(device cast.Device, minutes int) time.Duration {
	every := time.Duration(minutes) * time.Minute
	if strings.Contains(device.Model, "Nest Hub") && (every == 0 || every > nestHubRefreshInterval) {
		return nestHubRefreshInterval
	}
	return every
}

// displayURL is the display page as the TV reaches it: through the address
// this server has on the connection to that TV. The single-use launch token
// lets the page pair itself; the page removes it from the address bar.
func displayURL(localIP, port, displayID, launchToken string) string {
	return "http://" + net.JoinHostPort(localIP, port) + "/d/" + url.PathEscape(displayID) +
		"?" + url.Values{"launch": {launchToken}}.Encode()
}
