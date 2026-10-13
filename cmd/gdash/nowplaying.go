package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/nowplaying"
	"github.com/stijnvandepol/gdash/internal/outbound"
)

const (
	// nowPlayingFreshFor is how long what plays counts as current. The
	// monitors send a change at once and screens move a track on by
	// themselves, so a quiet hour of paused music is not stale data.
	nowPlayingFreshFor = 24 * time.Hour
	// nowPlayingInterval only names the pushed source's rhythm: monitors
	// poll every 30 seconds.
	nowPlayingInterval = 30 * time.Second
)

// newNowPlaying returns the watcher of what plays on Cast devices, which
// puts its value in the hub. Covers are fetched through the address guard,
// as an app on the network chooses their address.
func newNowPlaying(hub *datahub.Hub, sources *dataSources) *nowplaying.Watcher {
	artwork := &nowplaying.Artwork{HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent(), Now: time.Now}
	return &nowplaying.Watcher{
		Discover: cast.Discover,
		Publish:  func(value map[string]nowplaying.Item) { sources.putNowPlaying(hub, value) },
		Artwork:  artwork,
		Logger:   sources.logger,
	}
}

// putNowPlaying stores what plays. A hub without the source, because the
// last widget was just removed, is no error.
func (d *dataSources) putNowPlaying(hub *datahub.Hub, value map[string]nowplaying.Item) {
	raw, err := json.Marshal(value)
	if err != nil {
		d.logger.Error("could not encode what plays on Cast devices", "error", err)
		return
	}
	err = hub.Put(map[string]json.RawMessage{display.NowPlayingSource: raw})
	if err != nil && !errors.Is(err, datahub.ErrNotPushed) {
		d.logger.Warn("could not store what plays on Cast devices", "error", err)
	}
}

// nowPlayingSource is the pushed source that Now playing widgets read,
// while a dashboard has one.
func nowPlayingSource(dashboards []display.Dashboard) []datahub.Source {
	if !display.UsesWidget(dashboards, display.NowPlayingWidget) {
		return nil
	}
	return []datahub.Source{{
		Key: display.NowPlayingSource, Interval: nowPlayingInterval, FreshFor: nowPlayingFreshFor, Version: "cast",
	}}
}

// nowPlayingWant is what the watcher follows: the devices that widgets
// name, or any device, but never a TV that a display shows a dashboard on.
func nowPlayingWant(dashboards []display.Dashboard, list []displays.Display) nowplaying.Want {
	if !display.UsesWidget(dashboards, display.NowPlayingWidget) {
		return nowplaying.Want{}
	}
	ids, anyDevice := display.NowPlayingDevices(dashboards)
	want := nowplaying.Want{IDs: ids, Any: anyDevice}
	for _, d := range list {
		if d.CastDeviceID != "" {
			want.Exclude = append(want.Exclude, d.CastDeviceID)
		}
	}
	return want
}

// followNowPlaying points the watcher at what the dashboards show.
func (d *dataSources) followNowPlaying(ctx context.Context, dashboards []display.Dashboard) error {
	list, err := d.displays(ctx)
	if err != nil {
		return fmt.Errorf("read displays: %w", err)
	}
	d.nowPlaying.Configure(ctx, nowPlayingWant(dashboards, list))
	return nil
}

// displaysChanged follows a display that was linked to another TV, or
// unlinked: the watcher must leave the TVs that the keepers own alone.
func (d *dataSources) displaysChanged(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	dashboards, err := d.dashboards(ctx)
	if err != nil {
		return fmt.Errorf("read dashboards: %w", err)
	}
	return d.followNowPlaying(ctx, dashboards)
}
