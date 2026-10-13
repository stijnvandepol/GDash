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
	runDisplay := func(ctx context.Context, d displays.Display, report func(cast.State, error), takeOver <-chan struct{}) {
		keeper := &cast.Keeper{
			Resolve:  resolveDevice(d.CastDeviceID),
			Refresh:  refreshInterval,
			Active:   activeNow(ctx, db, d.ID, logger),
			Status:   report,
			TakeOver: takeOver,
			Recall:   func() string { return sessions.recall(d.ID) },
			Remember: func(id string) { sessions.remember(d.ID, id) },
			Logger:   logger.With("display", d.ID),
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

const (
	// Google TV put its screensaver over a running cast after 19 minutes
	// in a test at home; 15 minutes stays ahead of it.
	defaultRefreshInterval = 15 * time.Minute
	// Nest Hubs close a receiver without media after about 10 minutes.
	nestHubRefreshInterval = 9 * time.Minute
)

// refreshInterval is how often the receiver is restarted on a device. Each
// restart reloads the screen for a few seconds.
func refreshInterval(d cast.Device) time.Duration {
	if strings.Contains(d.Model, "Nest Hub") {
		return nestHubRefreshInterval
	}
	return defaultRefreshInterval
}

// displayURL is the display page as the TV reaches it: through the address
// this server has on the connection to that TV. The single-use launch token
// lets the page pair itself; the page removes it from the address bar.
func displayURL(localIP, port, displayID, launchToken string) string {
	return "http://" + net.JoinHostPort(localIP, port) + "/d/" + url.PathEscape(displayID) +
		"?" + url.Values{"launch": {launchToken}}.Encode()
}
