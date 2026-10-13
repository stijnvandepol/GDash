// Command gdash runs the GDash server: it fetches dashboard data, serves
// display pages and keeps a Cast receiver on a TV.
//
// Usage:
//
//	gdash            start the server
//	gdash discover   list Cast devices on the local network
//	gdash status A   show what the Cast device at address A is doing
//	gdash version    print the version
//	gdash healthcheck  exit 0 when the server on this machine answers
package main

import (
	"context"
	// The Docker image has no time zone database; calendars name zones such
	// as Europe/Amsterdam and "today" depends on TZ.
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/stijnvandepol/gdash/internal/api"
	"github.com/stijnvandepol/gdash/internal/apitoken"
	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/backup"
	"github.com/stijnvandepol/gdash/internal/camera"
	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/photos"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/update"
	"github.com/stijnvandepol/gdash/internal/weather"
	"github.com/stijnvandepol/gdash/internal/webpage"
	"github.com/stijnvandepol/gdash/web"
)

const (
	weatherInterval = 15 * time.Minute
	newsInterval    = 15 * time.Minute
	// The air quality model is updated hourly, and pollen moves slowly.
	airQualityInterval = 30 * time.Minute
	// Calendar changes should show up soon after they are made; ICS
	// servers are built for clients polling this often.
	calendarInterval = 10 * time.Minute
	calendarDays     = 7
	// Home Assistant runs in the home; once a minute keeps a sensor on the
	// TV current without a request storm. Entities are fetched one by one,
	// so their number is bounded.
	homeAssistantInterval    = time.Minute
	homeAssistantTimeout     = 30 * time.Second
	maxHomeAssistantEntities = 40
	// OVapi is run by volunteers: one request for all stops, at most every
	// 30 seconds, and only for stops that a dashboard shows. A departure
	// board counts down in minutes, so a longer wait would show old times.
	departuresInterval = 30 * time.Second
	maxDepartureStops  = 10
	// A refresh fetches all feeds or all calendars, partly in parallel;
	// each request may take upstreamTimeout.
	newsTimeout     = 30 * time.Second
	calendarTimeout = time.Minute
	// newsItems is what the server keeps; widgets show a part of it. Ten
	// short headlines keep a snapshot far below the 64 KB Cast limit.
	newsItems       = 10
	upstreamTimeout = 15 * time.Second
	// The update check is one small request a day; it may fail quietly.
	updateTimeout = 10 * time.Second
	// The Check of a web page under Settings waits this long for an answer.
	// A web page is an address an admin typed in, so it goes through
	// internal/outbound like a feed.
	webPageCheckTimeout = 10 * time.Second
	// photoListValidity only has to outlast the minute between publishes;
	// the list is read again every time.
	photoListValidity = time.Hour
	// A week of daily database copies covers a mistake noticed days later.
	backupsKept     = 7
	shutdownTimeout = 5 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch {
	case len(os.Args) == 1:
		err = serve(ctx, logger)
	case os.Args[1] == "discover":
		err = discover(ctx)
	case os.Args[1] == "status" && len(os.Args) == 3:
		err = status(ctx, os.Args[2])
	case os.Args[1] == "reset-password" && len(os.Args) == 3:
		err = resetPassword(ctx, os.Getenv, os.Args[2], os.Stdout)
	case os.Args[1] == "healthcheck":
		err = healthcheck(ctx, os.Getenv)
	case os.Args[1] == "version":
		fmt.Println(currentVersion())
	default:
		err = errors.New("usage: gdash | gdash discover | gdash status <device-address> | gdash healthcheck | gdash reset-password <user> | gdash version")
	}
	if err != nil {
		logger.Error("gdash stopped", "error", err)
		os.Exit(1)
	}
}

// newAuthService prepares login. On a fresh install the first account can be
// created from the home network in the first hour without a token; after
// that, or from elsewhere, it takes the one-time setup token in the log,
// which only the person who started GDash can read.
func newAuthService(ctx context.Context, db *store.Store, box *secret.Box, logger *slog.Logger) (*auth.Service, error) {
	token, err := auth.NewSetupToken()
	if err != nil {
		return nil, err
	}
	service, err := auth.NewService(db, box, token)
	if err != nil {
		return nil, err
	}
	needed, err := service.SetupNeeded(ctx)
	if err != nil {
		return nil, err
	}
	if needed {
		logger.Warn("GDash is not set up yet. Open the web interface within an hour to create the first account; after that, or from outside the home network, it asks for this setup token.", "setup_token", token)
	}
	return service, nil
}

func serve(ctx context.Context, logger *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	logger.Info("gdash starting", "version", currentVersion())
	if err := checkDataDir(cfg.dataDir); err != nil {
		return err
	}
	if adopted, err := adoptOldDatabase(cfg.dataDir); err != nil {
		return err
	} else if adopted != "" {
		logger.Info("renamed the database of an earlier version", "from", adopted, "to", databaseName)
	}
	db, err := store.Open(ctx, filepath.Join(cfg.dataDir, databaseName))
	if err != nil {
		return err
	}
	defer db.Close()
	go (&backup.Daily{
		Dir: filepath.Join(cfg.dataDir, "backups"), Keep: backupsKept,
		Write: db.WriteCopy, Now: time.Now, Logger: logger,
	}).Run(ctx)
	box, err := secret.LoadOrCreateKey(filepath.Join(cfg.dataDir, secretKeyName))
	if err != nil {
		return err
	}
	authService, err := newAuthService(ctx, db, box, logger)
	if err != nil {
		return err
	}
	settingsService, err := newSettingsService(ctx, cfg, db, box, logger)
	if err != nil {
		return err
	}
	album, err := photos.NewStore(filepath.Join(cfg.dataDir, "photos"))
	if err != nil {
		return err
	}
	snapshots, publisher, sources, err := startData(ctx, cfg, db, settingsService, album, logger)
	if err != nil {
		return err
	}
	pairings := pairing.New(db)
	castManager, err := newCastManager(cfg, castDeps{db: db, snapshots: snapshots, pairings: pairings, logger: logger})
	if err != nil {
		return err
	}
	go castManager.Run(ctx)

	frontend, err := web.Dist()
	if err != nil {
		return err
	}
	updates := &update.Checker{
		URL: update.DefaultURL, HTTP: outbound.NewClient(updateTimeout), UserAgent: userAgent(),
		Current: currentVersion(), Enabled: settingsService.UpdateCheck, Off: cfg.updateCheckOff,
		Now: time.Now, Logger: logger,
	}
	go updates.Run(ctx)
	handler := (&api.Server{
		Snapshots:     snapshots,
		Auth:          authService,
		Pairing:       pairings,
		APITokens:     apitoken.New(db),
		Store:         db,
		Settings:      settingsService,
		Places:        weather.Geocoder{BaseURL: weather.DefaultGeocodingURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		HomeAssistant: sources.home,
		Homey:         sources.homey,
		Immich:        sources.immich,
		Synology:      sources.synology,
		Cameras: &camera.Keeper{
			Fetcher: &camera.Fetcher{HTTP: outbound.NewClient(camera.Timeout), UserAgent: userAgent()},
			Now:     time.Now, Logger: logger,
		},
		Stops:          sources.stops,
		CustomFetcher:  sources.custom,
		WebPageChecker: &webpage.Checker{HTTP: outbound.NewClient(webPageCheckTimeout), UserAgent: userAgent()},
		ReceiveCustom:  sources.receiveCustom,
		ReceiveHomey:   sources.receiveHomey,
		TrustedProxies: cfg.trustedProxies,
		Web:            frontend,
		Discover:       cast.Discover,
		DisplaysChanged: func() {
			publisher.Refresh(ctx)
			castManager.Changed()
			if err := sources.displaysChanged(ctx); err != nil {
				logger.Error("could not follow the display change for Now playing; restart GDash", "error", err)
			}
		},
		DashboardsChanged: func() {
			// A dashboard can start or stop showing a Home Assistant entity,
			// a Homey value or user, a transit stop or a custom source.
			if err := sources.sync(ctx); err != nil {
				logger.Error("could not follow the dashboard change; restart GDash", "error", err)
			}
			publisher.Refresh(ctx)
		},
		Announcements:        publisher.Announcements,
		AnnouncementsChanged: announced(ctx, publisher),
		SettingsChanged: func() {
			if err := sources.sync(ctx); err != nil {
				logger.Error("could not apply the new settings; restart GDash", "error", err)
			}
			// The display language travels in the snapshots.
			publisher.Refresh(ctx)
		},
		SourceStatus:  sources.hub.Entries,
		Data:          publisher.Data,
		Version:       currentVersion(),
		Updates:       updates,
		CastStatus:    castManager.Statuses,
		TakeOverCast:  castManager.TakeOver,
		Photos:        album,
		Backup:        backup.Archive{DataDir: cfg.dataDir, DatabaseName: databaseName, Write: db.WriteCopy},
		PhotosChanged: func() { publisher.Refresh(ctx) },
		Artwork:       sources.nowPlaying.Artwork,
		SentData:      snapshots.Sent,
		Now:           time.Now,
		Logger:        logger,
	}).Handler()
	return listen(ctx, cfg.addr, handler, logger)
}

// secretKeyName is the key file in the data directory that seals calendar
// addresses, the Home Assistant token and two-step login secrets.
const secretKeyName = "secret.key"

// newSettingsService prepares the settings and, on the first start, fills
// them from the environment. A fresh install also gets the default
// dashboard, with a calendar when one was configured.
func newSettingsService(ctx context.Context, cfg config, db *store.Store, box *secret.Box, logger *slog.Logger) (*settings.Service, error) {
	svc := &settings.Service{Store: db, Box: box, Now: time.Now, Logger: logger}
	seeded, err := svc.SeedOnce(ctx, cfg.seed)
	if err != nil {
		return nil, err
	}
	if seeded {
		logger.Info("data sources copied from the environment; from now on, change them under Integrations in the web interface")
	}
	current, err := svc.Get(ctx)
	if err != nil {
		return nil, err
	}
	// Later edits to the default dashboard survive restarts.
	if err := db.EnsureDashboard(ctx, display.DefaultDashboard(len(current.Calendars) > 0), time.Now()); err != nil {
		return nil, err
	}
	return svc, nil
}

// listen serves HTTP until ctx ends, then shuts down gracefully.
func listen(ctx context.Context, addr string, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// A request body that drips in, a byte at a time, cannot hold a
		// connection longer than this. It does not end event streams, whose
		// request has been read (internal/api tests this).
		ReadTimeout: time.Minute,
		// No WriteTimeout: display event streams stay open for days; they
		// set a deadline per write instead.
		IdleTimeout: 2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("shutdown did not finish cleanly", "error", err)
		}
	}()
	logger.Info("gdash listening; open the web interface at / on this address", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
