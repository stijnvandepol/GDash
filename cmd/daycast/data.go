package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/stijnvandepol/daycast/internal/calendar"
	"github.com/stijnvandepol/daycast/internal/datahub"
	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/news"
	"github.com/stijnvandepol/daycast/internal/outbound"
	"github.com/stijnvandepol/daycast/internal/settings"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/weather"
)

// dataSources keeps the data hub in line with the settings.
type dataSources struct {
	hub      *datahub.Hub
	settings *settings.Service
	weather  weather.Client
	news     *news.Client
	calendar *calendar.Client
	logger   *slog.Logger

	// mu makes syncs run one at a time, so an older read of the settings
	// can never be applied after a newer one.
	mu sync.Mutex
}

// startData starts fetching dashboard data. The publisher turns every data
// or configuration change into new display snapshots on the broadcaster.
func startData(ctx context.Context, cfg config, db *store.Store, svc *settings.Service, logger *slog.Logger) (*display.Broadcaster, *display.Publisher, *dataSources, error) {
	cache, err := datahub.NewFileStore(filepath.Join(cfg.dataDir, "cache"))
	if err != nil {
		return nil, nil, nil, err
	}
	broadcaster := display.NewBroadcaster()
	var hub *datahub.Hub
	publisher := &display.Publisher{
		Broadcaster: broadcaster,
		Views:       db.DisplayViews,
		Data:        func() map[string]datahub.Entry { return hub.Entries() },
		Language:    svc.Language,
		Now:         time.Now,
		Logger:      logger,
	}
	hub = datahub.New(cache, logger, func() { publisher.Refresh(ctx) })
	publisher.Refresh(ctx)
	go republishEveryMinute(ctx, publisher)

	sources := &dataSources{
		hub:      hub,
		settings: svc,
		weather:  weather.Client{BaseURL: weather.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		// Feeds and calendars are addresses an admin typed in.
		news:     &news.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		calendar: &calendar.Client{HTTP: outbound.NewClient(upstreamTimeout), Location: time.Local, Now: time.Now},
		logger:   logger,
	}
	if err := sources.sync(ctx); err != nil {
		return nil, nil, nil, err
	}
	return broadcaster, publisher, sources, nil
}

// sync reads the settings and reconfigures the hub. ctx is the server's
// lifetime: the sources it starts run until it ends.
func (d *dataSources) sync(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cfg, err := d.settings.Sources(ctx)
	if err != nil {
		return fmt.Errorf("read data sources: %w", err)
	}
	d.hub.Configure(ctx, d.build(cfg))
	return nil
}

// build turns settings into hub sources. A source that has nothing to fetch
// is left out, so its widgets show that it is not set up.
func (d *dataSources) build(cfg settings.Sources) []datahub.Source {
	var sources []datahub.Source
	if loc := cfg.Location; loc != nil {
		sources = append(sources, datahub.Source{
			Key:      "weather.current",
			Interval: weatherInterval,
			Version:  fingerprint(*loc),
			Fetch:    func(ctx context.Context) (any, error) { return d.weather.Current(ctx, *loc) },
		})
	}
	if feeds := cfg.NewsFeeds; len(feeds) > 0 {
		sources = append(sources, datahub.Source{
			Key:      "news.default",
			Interval: newsInterval,
			Timeout:  newsTimeout,
			Version:  fingerprint(feeds),
			Fetch:    func(ctx context.Context) (any, error) { return d.news.Headlines(ctx, feeds, newsItems) },
		})
	}
	if calendars := cfg.Calendars; len(calendars) > 0 {
		sources = append(sources, datahub.Source{
			Key:      "calendar.default",
			Interval: calendarInterval,
			Timeout:  calendarTimeout,
			Version:  fingerprint(calendars),
			Fetch: func(ctx context.Context) (any, error) {
				return d.calendar.Upcoming(ctx, calendars, calendarDays)
			},
		})
	}
	return sources
}

// fingerprint identifies a source's configuration. It is a hash, so secret
// calendar addresses do not sit in the hub as plain text.
func fingerprint(v any) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%+v", v))
	return hex.EncodeToString(sum[:8])
}

// republishEveryMinute lets displays cross the edge of their active hours on
// time, even when no data changes.
func republishEveryMinute(ctx context.Context, publisher *display.Publisher) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publisher.Refresh(ctx)
		}
	}
}
