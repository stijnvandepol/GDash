// Command daycast runs the Daycast server: it fetches dashboard data, serves
// display pages and keeps a Cast receiver on a TV.
//
// Usage:
//
//	daycast            start the server
//	daycast discover   list Cast devices on the local network
//	daycast status A   show what the Cast device at address A is doing
//	daycast version    print the version
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
	"text/tabwriter"
	"time"
	_ "time/tzdata"

	"github.com/stijnvandepol/daycast/internal/api"
	"github.com/stijnvandepol/daycast/internal/auth"
	"github.com/stijnvandepol/daycast/internal/backup"
	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/pairing"
	"github.com/stijnvandepol/daycast/internal/secret"
	"github.com/stijnvandepol/daycast/internal/settings"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/weather"
	"github.com/stijnvandepol/daycast/web"
)

const (
	weatherInterval = 15 * time.Minute
	newsInterval    = 15 * time.Minute
	// Calendar changes should show up soon after they are made; ICS
	// servers are built for clients polling this often.
	calendarInterval = 10 * time.Minute
	calendarDays     = 7
	// newsItems is what the server keeps; widgets show a part of it. Ten
	// short headlines keep a snapshot far below the 64 KB Cast limit.
	newsItems       = 10
	upstreamTimeout = 15 * time.Second
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
	case os.Args[1] == "version":
		fmt.Println(currentVersion())
	default:
		err = errors.New("usage: daycast | daycast discover | daycast status <device-address> | daycast version")
	}
	if err != nil {
		logger.Error("daycast stopped", "error", err)
		os.Exit(1)
	}
}

func discover(ctx context.Context) error {
	devices, err := cast.Discover(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No Cast devices found. Is this machine on the same network and VLAN as the TV?")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODEL\tSCREEN\tADDRESS\tID")
	for _, d := range devices {
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n", d.Name, d.Model, d.HasScreen, d.Addr, d.ID)
	}
	return w.Flush()
}

// status shows what a Cast device is doing without changing anything on it,
// to check the CASTV2 connection and to see how a device reports its state.
func status(ctx context.Context, device string) error {
	addr := withDefaultPort(device)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := cast.Dial(ctx, addr)
	if err != nil {
		return err
	}
	defer client.Close()
	st, err := client.Status(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("standby: %v  active input: %v  daycast may launch: %v\n", st.IsStandBy, st.IsActiveInput, cast.ShouldRelaunch(st))
	if len(st.Applications) == 0 {
		fmt.Println("no apps running")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "APP ID\tNAME\tIDLE SCREEN")
	for _, app := range st.Applications {
		fmt.Fprintf(w, "%s\t%s\t%v\n", app.AppID, app.DisplayName, app.IsIdleScreen)
	}
	return w.Flush()
}

// newAuthService prepares login. On a fresh install it prints a one-time
// setup token to the log: whoever can read the log (the person who started
// Daycast) can create the first account, nobody else on the network.
func newAuthService(ctx context.Context, db *store.Store, logger *slog.Logger) (*auth.Service, error) {
	token, err := auth.NewSetupToken()
	if err != nil {
		return nil, err
	}
	service, err := auth.NewService(db, token)
	if err != nil {
		return nil, err
	}
	needed, err := service.SetupNeeded(ctx)
	if err != nil {
		return nil, err
	}
	if needed {
		logger.Warn("Daycast is not set up yet. Open the web interface and enter this setup token.", "setup_token", token)
	}
	return service, nil
}

func serve(ctx context.Context, logger *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	logger.Info("daycast starting", "version", currentVersion())
	db, err := store.Open(ctx, filepath.Join(cfg.dataDir, "daycast.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	go (&backup.Daily{
		Dir: filepath.Join(cfg.dataDir, "backups"), Keep: backupsKept,
		Write: db.WriteCopy, Now: time.Now, Logger: logger,
	}).Run(ctx)
	authService, err := newAuthService(ctx, db, logger)
	if err != nil {
		return err
	}
	settingsService, err := newSettingsService(ctx, cfg, db, logger)
	if err != nil {
		return err
	}
	snapshots, publisher, sources, err := startData(ctx, cfg, db, settingsService, logger)
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
	handler := (&api.Server{
		Snapshots: snapshots,
		Auth:      authService,
		Pairing:   pairings,
		Store:     db,
		Settings:  settingsService,
		Places:    weather.Geocoder{BaseURL: weather.DefaultGeocodingURL, HTTP: &http.Client{Timeout: upstreamTimeout}},
		Web:       frontend,
		Discover:  cast.Discover,
		DisplaysChanged: func() {
			publisher.Refresh(ctx)
			castManager.Changed()
		},
		DashboardsChanged: func() { publisher.Refresh(ctx) },
		SettingsChanged: func() {
			if err := sources.sync(ctx); err != nil {
				logger.Error("could not apply the new settings; restart Daycast", "error", err)
			}
			// The display language travels in the snapshots.
			publisher.Refresh(ctx)
		},
		SourceStatus: sources.hub.Entries,
		Version:      currentVersion(),
		CastStatus:   castManager.Statuses,
		Now:          time.Now,
		Logger:       logger,
	}).Handler()
	return listen(ctx, cfg.addr, handler, logger)
}

// newSettingsService opens the secret key and, on the first start, fills
// the settings from the environment. A fresh install also gets the default
// dashboard, with a calendar when one was configured.
func newSettingsService(ctx context.Context, cfg config, db *store.Store, logger *slog.Logger) (*settings.Service, error) {
	box, err := secret.LoadOrCreateKey(filepath.Join(cfg.dataDir, "secret.key"))
	if err != nil {
		return nil, err
	}
	svc := &settings.Service{Store: db, Box: box, Now: time.Now, Logger: logger}
	seeded, err := svc.SeedOnce(ctx, cfg.seed)
	if err != nil {
		return nil, err
	}
	if seeded {
		logger.Info("data sources copied from the environment; from now on, change them under Settings in the web interface")
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
		// No WriteTimeout: display event streams stay open for days.
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
	logger.Info("daycast listening; open the web interface at / on this address", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
