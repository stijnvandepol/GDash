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
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/departures"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/energyprice"
	"github.com/stijnvandepol/gdash/internal/fuel"
	"github.com/stijnvandepol/gdash/internal/holidays"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/immich"
	"github.com/stijnvandepol/gdash/internal/knmi"
	"github.com/stijnvandepol/gdash/internal/news"
	"github.com/stijnvandepol/gdash/internal/nowplaying"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/photos"
	"github.com/stijnvandepol/gdash/internal/rain"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/solar"
	"github.com/stijnvandepol/gdash/internal/speedcameras"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/synology"
	"github.com/stijnvandepol/gdash/internal/traffic"
	"github.com/stijnvandepol/gdash/internal/warnings"
	"github.com/stijnvandepol/gdash/internal/weather"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

// dataSources keeps the data hub in line with the settings.
type dataSources struct {
	hub        *datahub.Hub
	settings   *settings.Service
	weather    weather.Client
	airQuality airquality.Client
	news       *news.Client
	// newsStandIn is GDASH_TEST_UPSTREAM's address for the feeds of the
	// news catalogue, or empty outside the end-to-end tests.
	newsStandIn string
	calendar    *calendar.Client
	home        *homeassistant.Client
	homey       *homey.Client
	immich      *immich.Client
	synology    *synology.Client
	energy      energyprice.Client
	solar       solar.Client
	departures  *departures.Client
	fuel        *fuel.Client
	holidays    holidays.Client
	rain        *rain.Client
	warnings    *warnings.Client
	// traffic guards the requests to the ANWB; cameras keeps the fixed
	// camera list; knmi reads measured weather.
	traffic *traffic.Guard
	cameras *speedcameras.Store
	knmi    *knmi.Client
	custom  *custom.Fetcher
	// stops searches the stop areas for the widget editor.
	stops *departures.Directory
	// dashboards lists every dashboard, for the Home Assistant entities,
	// transit stops and custom sources their widgets show.
	dashboards func(ctx context.Context) ([]display.Dashboard, error)
	// displays lists every display, for the TVs the Now playing watcher
	// must leave alone.
	displays func(ctx context.Context) ([]displays.Display, error)
	// nowPlaying follows what plays on the Cast devices that Now playing
	// widgets show.
	nowPlaying *nowplaying.Watcher
	// pages are the web pages under Integrations, read at every sync.
	pages  *webPageList
	logger *slog.Logger

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
	var rules *customRules
	pages := &webPageList{}
	publisher := &display.Publisher{
		Broadcaster: broadcaster,
		Views:       db.DisplayViews,
		Data: func() map[string]datahub.Entry {
			return withWebPages(withPhotos(hub.Entries(), album, logger), pages.get(), logger)
		},
		Language:      svc.Language,
		Announcements: announcement.NewBoard(time.Now),
		Now:           time.Now,
		Logger:        logger,
	}
	var warned *warningRules
	refresh := func() {
		rules.check(ctx)
		warned.check(ctx)
		publisher.Refresh(ctx)
	}
	rules = &customRules{
		announcer:  custom.Announcer{Gap: custom.AnnounceGap},
		dashboards: db.Dashboards,
		displays:   db.Displays,
		data:       func() map[string]datahub.Entry { return hub.Entries() },
		board:      publisher.Announcements,
		announced:  announced(ctx, publisher),
		audit:      db.AddAuditEvent,
		now:        time.Now,
		logger:     logger,
	}
	warned = &warningRules{
		dashboards: db.Dashboards,
		displays:   db.Displays,
		data:       func() map[string]datahub.Entry { return hub.Entries() },
		language:   svc.Language,
		board:      publisher.Announcements,
		announced:  announced(ctx, publisher),
		audit:      db.AddAuditEvent,
		now:        time.Now,
		logger:     logger,
	}
	hub = datahub.New(cache, logger, refresh)
	publisher.Refresh(ctx)
	go everyMinute(ctx, refresh)

	dutch, err := time.LoadLocation(dutchTimeZone)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load the Dutch time zone: %w", err)
	}
	userAgent := userAgent()
	transit := &departures.Client{
		BaseURL:   departures.DefaultBaseURL,
		HTTP:      outbound.NewClient(upstreamTimeout),
		UserAgent: userAgent,
		Now:       time.Now,
	}
	anwbURL, overpassURL, knmiURL := traffic.DefaultBaseURL, speedcameras.DefaultBaseURL, knmi.DefaultBaseURL
	if cfg.testUpstream != "" {
		anwbURL, overpassURL, knmiURL = cfg.testUpstream+testANWBTrafficPath, cfg.testUpstream+testOverpassPath, cfg.testUpstream+testKNMIPath
	}
	sources := &dataSources{
		hub:        hub,
		settings:   svc,
		weather:    weather.Client{BaseURL: weather.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		airQuality: airquality.Client{BaseURL: airquality.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		// Feeds and calendars are addresses an admin typed in.
		news:       &news.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		calendar:   &calendar.Client{HTTP: outbound.NewClient(upstreamTimeout), Location: time.Local, Now: time.Now},
		home:       &homeassistant.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		homey:      &homey.Client{HTTP: outbound.NewClient(upstreamTimeout)},
		immich:     &immich.Client{HTTP: outbound.NewClient(immich.Timeout)},
		synology:   &synology.Client{},
		departures: transit,
		fuel:       &fuel.Client{BaseURL: fuel.DefaultBaseURL, HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent},
		traffic: &traffic.Guard{
			Client: traffic.Client{BaseURL: anwbURL, HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent},
			Now:    time.Now,
		},
		cameras: &speedcameras.Store{
			Client: speedcameras.Client{BaseURL: overpassURL, HTTP: outbound.NewClient(overpassHTTP), UserAgent: userAgent},
			Path:   filepath.Join(cfg.dataDir, "speedcameras.json"),
			Now:    time.Now,
			Logger: logger,
		},
		knmi:       &knmi.Client{BaseURL: knmiURL, HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent, Now: time.Now},
		custom:     &custom.Fetcher{HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent, Box: svc.Box, Now: time.Now},
		stops:      &departures.Directory{Source: transit, Now: time.Now, Logger: logger},
		dashboards: db.Dashboards,
		displays:   db.Displays,
		pages:      pages,
		logger:     logger,
		energy:     energyprice.Client{BaseURL: energyprice.DefaultBaseURL, HTTP: outbound.NewClient(upstreamTimeout), Location: dutch, Now: time.Now},
		holidays: holidays.Client{
			SchoolURL: cfg.schoolHolidaysURL, HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent, Location: dutch, Now: time.Now,
		},
		rain: &rain.Client{
			BuienradarURL: rain.DefaultBuienradarURL, OpenMeteoURL: rain.DefaultOpenMeteoURL,
			HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent, Location: dutch, Now: time.Now,
		},
		warnings: &warnings.Client{BaseURL: warnings.DefaultBaseURL, HTTP: outbound.NewClient(upstreamTimeout), UserAgent: userAgent, Now: time.Now},
		solar: solar.Client{
			OpenMeteo:     solar.OpenMeteo{BaseURL: solar.DefaultOpenMeteoURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
			ForecastSolar: solar.ForecastSolar{BaseURL: solar.DefaultForecastSolarURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		},
	}
	if cfg.testUpstream != "" {
		logger.Warn("GDASH_TEST_UPSTREAM is set: rain, weather warnings, the solar forecast, traffic, fixed cameras, KNMI measurements and the feeds of the news catalogue come from a stand-in, not from Buienradar, Open-Meteo, MeteoAlarm, the ANWB, Overpass, the KNMI or the publishers", "address", cfg.testUpstream)
		sources.rain.BuienradarURL = cfg.testUpstream + testBuienradarPath
		sources.rain.OpenMeteoURL = cfg.testUpstream + testOpenMeteoPath
		sources.warnings.BaseURL = cfg.testUpstream + testMeteoAlarmPath
		sources.solar.OpenMeteo.BaseURL = cfg.testUpstream + testSolarPath
		sources.newsStandIn = cfg.testUpstream + testNewsPath
	}
	sources.nowPlaying = newNowPlaying(hub, sources)
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

// webPageList holds the web pages under Integrations from one sync to the
// next, so a publish does not read the database.
type webPageList struct {
	mu    sync.Mutex
	pages []webpage.Page
}

func (l *webPageList) set(pages []webpage.Page) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pages = pages
}

func (l *webPageList) get() []webpage.Page {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pages
}

// withWebPages adds the web pages under Integrations to the data, for the Web
// page widgets; each screen only receives those its own widgets show.
func withWebPages(entries map[string]datahub.Entry, pages []webpage.Page, logger *slog.Logger) map[string]datahub.Entry {
	if len(pages) == 0 {
		return entries
	}
	entry, err := display.WebPageEntry(pages, time.Now())
	if err != nil {
		logger.Error("could not encode the web pages", "error", err)
		return entries
	}
	entries[display.WebPageSource] = entry
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
	customs, err := d.settings.FetchableCustomSources(ctx)
	if err != nil {
		return fmt.Errorf("read custom sources: %w", err)
	}
	webPages, err := d.settings.WebPages(ctx)
	if err != nil {
		return fmt.Errorf("read web pages: %w", err)
	}
	d.pages.set(webPages)
	sources := d.build(cfg, display.EntityIDs(dashboards), display.StopCodes(dashboards))
	sources = append(sources, d.dashboardSources(cfg, dashboards)...)
	sources = append(sources, d.homeySources(cfg, dashboards)...)
	sources = append(sources, d.albumSources(cfg, dashboards)...)
	sources = append(sources, nowPlayingSource(dashboards)...)
	d.hub.Configure(ctx, append(sources, d.customSources(customs, display.CustomSourceIDs(dashboards))...))
	// After the hub has the source, so the watcher's first value lands.
	return d.followNowPlaying(ctx, dashboards)
}

// build turns settings into hub sources. A source that has nothing to fetch
// is left out, so its widgets show that it is not set up. entities are the
// Home Assistant entities and stops the transit stop areas that dashboards
// show.
func (d *dataSources) build(cfg settings.Sources, entities, stops []string) []datahub.Source {
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
	if feeds := d.newsFeeds(cfg.NewsFeeds); len(feeds) > 0 {
		sources = append(sources, datahub.Source{
			Key:      display.NewsSource,
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
	if len(stops) > 0 {
		// One request covers all stops; the cap keeps the address short.
		stops = stops[:min(len(stops), maxDepartureStops)]
		sources = append(sources, datahub.Source{
			Key:      display.DeparturesSource,
			Interval: departuresInterval,
			Version:  d.fingerprint(stops),
			Fetch:    func(ctx context.Context) (any, error) { return d.departures.Boards(ctx, stops) },
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

// everyMinute runs refresh every minute, so displays cross the edge of
// their active hours on time and time-based rules apply, even when no data
// changes.
func everyMinute(ctx context.Context, refresh func()) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

// announced returns what to do after an announcement is posted: show it
// now, and take it out of the snapshot a screen gets when it connects
// later, once it expires (the screen drops it by itself).
func announced(ctx context.Context, publisher *display.Publisher) func(expiresAt time.Time) {
	return func(expiresAt time.Time) {
		publisher.Refresh(ctx)
		time.AfterFunc(time.Until(expiresAt), func() { publisher.Refresh(ctx) })
	}
}
