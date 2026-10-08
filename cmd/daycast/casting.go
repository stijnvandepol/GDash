package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"

	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/casting"
	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/displays"
	"github.com/stijnvandepol/daycast/internal/store"
)

// newCastManager keeps the dashboard on every display that is linked to a
// Cast device. Without an own receiver app ID it uses DashCast, which opens
// the display page on the TV; with one it pushes snapshots to that receiver.
func newCastManager(cfg config, db *store.Store, snapshots *display.Broadcaster, logger *slog.Logger) (*casting.Manager, error) {
	_, port, err := net.SplitHostPort(cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("DAYCAST_ADDR %q has no port: %w", cfg.addr, err)
	}
	runDisplay := func(ctx context.Context, d displays.Display) {
		keeper := &cast.Keeper{Resolve: resolveDevice(d.CastDeviceID), Logger: logger.With("display", d.ID)}
		if cfg.castAppID != "" {
			keeper.AppID, keeper.Namespace = cfg.castAppID, display.CastNamespace
			keeper.Messages = func(ctx context.Context, _ string) <-chan []byte { return snapshots.Subscribe(ctx) }
		} else {
			keeper.AppID, keeper.Namespace = cast.DashCastAppID, cast.DashCastNamespace
			keeper.Messages = func(ctx context.Context, localIP string) <-chan []byte {
				return single(ctx, cast.DashCastLoad(displayURL(localIP, port, d.ID)))
			}
		}
		keeper.Run(ctx)
	}
	return &casting.Manager{List: db.Displays, RunDisplay: runDisplay, Logger: logger}, nil
}

// resolveDevice finds a Cast device's current address by its device ID.
func resolveDevice(deviceID string) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		devices, err := cast.Discover(ctx)
		if err != nil {
			return "", err
		}
		for _, d := range devices {
			if d.ID == deviceID {
				return d.Addr, nil
			}
		}
		return "", fmt.Errorf("cast device %s not found on the network; is it switched on?", deviceID)
	}
}

// displayURL is the display page as the TV reaches it: through the address
// this server has on the connection to that TV.
func displayURL(localIP, port, displayID string) string {
	return "http://" + net.JoinHostPort(localIP, port) + "/d/" + url.PathEscape(displayID)
}

// single returns a channel that holds msg until ctx ends.
func single(ctx context.Context, msg []byte) <-chan []byte {
	ch := make(chan []byte, 1)
	ch <- msg
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}
