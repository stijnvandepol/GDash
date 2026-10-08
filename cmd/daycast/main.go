// Command daycast runs the Daycast server: it fetches dashboard data, serves
// display pages and keeps a Cast receiver on a TV.
//
// Usage:
//
//	daycast            start the server
//	daycast discover   list Cast devices on the local network
//	daycast status A   show what the Cast device at address A is doing
package main

import (
	"context"
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

	"github.com/stijnvandepol/daycast/internal/api"
	"github.com/stijnvandepol/daycast/internal/auth"
	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/datahub"
	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/weather"
	"github.com/stijnvandepol/daycast/web"
)

const (
	weatherInterval = 15 * time.Minute
	upstreamTimeout = 15 * time.Second
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
	default:
		err = errors.New("usage: daycast | daycast discover | daycast status <device-address>")
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
	addr, err := withDefaultPort(device)
	if err != nil {
		return err
	}
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
	frontend, err := web.Dist()
	if err != nil {
		return err
	}
	cache, err := datahub.NewFileStore(filepath.Join(cfg.dataDir, "cache"))
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, filepath.Join(cfg.dataDir, "daycast.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	authService, err := newAuthService(ctx, db, logger)
	if err != nil {
		return err
	}

	dashboard := display.DefaultDashboard()
	broadcaster := display.NewBroadcaster()
	var hub *datahub.Hub
	publish := func() {
		if err := broadcaster.Publish(time.Now(), dashboard, hub.Entries()); err != nil {
			logger.Error("could not publish display state", "error", err)
		}
	}
	hub = datahub.New(cache, logger, publish)
	publish()

	weatherClient := weather.Client{BaseURL: weather.DefaultBaseURL, HTTP: &http.Client{Timeout: upstreamTimeout}}
	go hub.Run(ctx, []datahub.Source{{
		Key:      "weather.current",
		Interval: weatherInterval,
		Fetch:    func(ctx context.Context) (any, error) { return weatherClient.Current(ctx, cfg.location) },
	}})

	if cfg.castDevice != "" {
		keeper := &cast.Keeper{
			Addr:      cfg.castDevice,
			AppID:     cfg.castAppID,
			Namespace: display.CastNamespace,
			Subscribe: broadcaster.Subscribe,
			Logger:    logger,
		}
		go keeper.Run(ctx)
	}

	srv := &http.Server{
		Addr: cfg.addr,
		Handler: (&api.Server{
			Snapshots: broadcaster,
			Auth:      authService,
			Store:     db,
			Web:       frontend,
			Discover:  cast.Discover,
			Now:       time.Now,
			Logger:    logger,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: display event streams stay open for days.
		IdleTimeout: 2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("daycast listening; open /d/<name> on a browser display", "addr", cfg.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
