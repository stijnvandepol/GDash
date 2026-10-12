package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/airquality"
	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/news"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/photos"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// dataSources keeps the data hub in line with the settings.
type dataSources struct {
	hub        *datahub.Hub
	settings   *settings.Service
	weather    weather.Client
	airQuality airquality.Client
	news       *news.Client
	calendar   *calendar.Client
	home       *homeassistant.Client
	// dashboards lists every dashboard, for the Home Assistant entities
	// their widgets show.
	dashboards func(ctx context.Context) ([]display.Dashboard, error)
	logger     *slog.Logger

	// mu makes syncs run one at a time, so an older read of the settings
	// can never be applied after a newer one.
	mu sync.Mutex
}

// startData starts fetching dashboard data. The publisher turns every data
// or configuration change into new display snapshots on the broadcaster.
func startData(ctx context.Context, cfg config, db *store.Store, svc *settings.Service, album *photos.Store, logger *slog.Logger) (*display.Broadcaster, *display.Publisher, *dataSources, error) {
	cache, err := datahub.NewFileStore(filepath.Join(cfg.dataDir, "cache"))
	if err != nil {
		return nil, nil, nil, err
	}
	broadcaster := display.NewBroadcaster()
	var hub *datahub.Hub
	publisher := &display.Publisher{
		Broadcaster:   broadcaster,
		Views:         db.DisplayViews,
		Data:          func() map[string]datahub.Entry { return withPhotos(hub.Entries(), album, logger) },
		Language:      svc.Language,
		Announcements: announcement.NewBoard(time.Now),
		Now:           time.Now,
		Logger:        logger,
	}
	hub = datahub.New(cache, logger, func() { publisher.Refresh(ctx) })
	publisher.Refresh(ctx)
	go republishEveryMinute(ctx, publisher)

	sources := &dataSources{
		hub:        hub,
		settings:   svc,
		weather:    weather.Client{BaseURL: weather.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		airQuality: airquality.Client{BaseURL: airquality.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		// Feeds and calendars are addresses an admin typed in.
		news:       &news.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		calendar:   &calendar.Client{HTTP: outbound.NewClient(upstreamTimeout), Location: time.Local, Now: time.Now},
		home:       &homeassistant.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		dashboards: db.Dashboards,
		logger:     logger,
	}
	if err := sources.sync(ctx); err != nil {
		return nil, nil, nil, err
	}
	return broadcaster, publisher, sources, nil
}

// withPhotos adds the list of photo names to the fetched data, for the
// photo widget. Reading a folder of at most 50 files on every publish is
// cheaper than keeping a copy in step.
func withPhotos(entries map[string]datahub.Entry, album *photos.Store, logger *slog.Logger) map[string]datahub.Entry {
	list, err := album.List()
	if err != nil {
		logger.Error("could not list photos; photo widgets keep their last list", "error", err)
		return entries
	}
	names := make([]string, len(list))
	for i, p := range list {
		names[i] = p.Name
	}
	value, err := json.Marshal(names)
	if err != nil {
		logger.Error("could not encode the photo list", "error", err)
		return entries
	}
	now := time.Now()
	// The list is never stale: it is read from disk on every publish.
	entries[display.PhotosSource] = datahub.Entry{Value: value, FetchedAt: now, StaleAfter: now.Add(photoListValidity)}
	return entries
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
	dashboards, err := d.dashboards(ctx)
	if err != nil {
		return fmt.Errorf("read dashboards: %w", err)
	}
	d.hub.Configure(ctx, d.build(cfg, display.EntityIDs(dashboards)))
	return nil
}

// build turns settings into hub sources. A source that has nothing to fetch
// is left out, so its widgets show that it is not set up. entities are the
// Home Assistant entities that dashboards show.
func (d *dataSources) build(cfg settings.Sources, entities []string) []datahub.Source {
	var sources []datahub.Source
	if loc := cfg.Location; loc != nil {
		sources = append(sources, datahub.Source{
			Key:      display.WeatherSource,
			Interval: weatherInterval,
			Version:  d.fingerprint(*loc),
			Fetch:    func(ctx context.Context) (any, error) { return d.weather.Current(ctx, *loc) },
		}, datahub.Source{
			Key:      display.AirQualitySource,
			Interval: airQualityInterval,
			Version:  d.fingerprint(*loc),
			Fetch:    func(ctx context.Context) (any, error) { return d.airQuality.Current(ctx, *loc) },
		})
	}
	if feeds := cfg.NewsFeeds; len(feeds) > 0 {
		sources = append(sources, datahub.Source{
			Key:      "news.default",
			Interval: newsInterval,
			Timeout:  newsTimeout,
			Version:  d.fingerprint(feeds),
			Fetch:    func(ctx context.Context) (any, error) { return d.news.Headlines(ctx, feeds, newsItems) },
		})
	}
	if calendars := cfg.Calendars; len(calendars) > 0 {
		sources = append(sources, datahub.Source{
			Key:      "calendar.default",
			Interval: calendarInterval,
			Timeout:  calendarTimeout,
			Version:  d.fingerprint(calendars),
			Fetch: func(ctx context.Context) (any, error) {
				return d.calendar.Upcoming(ctx, calendars, calendarDays)
			},
		})
	}
	if conn := cfg.HomeAssistant; conn != nil && len(entities) > 0 {
		entities = entities[:min(len(entities), maxHomeAssistantEntities)]
		sources = append(sources, datahub.Source{
			Key:      display.HomeAssistantSource,
			Interval: homeAssistantInterval,
			Timeout:  homeAssistantTimeout,
			Version:  d.fingerprint(struct{ Connection, Entities any }{*conn, entities}),
			Fetch:    func(ctx context.Context) (any, error) { return d.home.States(ctx, *conn, entities) },
		})
	}
	return sources
}

// fingerprint identifies a source's configuration. It is keyed with the
// installation's secret key and written to the cache files, so secret
// calendar addresses and tokens cannot be guessed from it.
func (d *dataSources) fingerprint(v any) string {
	return d.settings.Box.Fingerprint(fmt.Appendf(nil, "%+v", v))
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
