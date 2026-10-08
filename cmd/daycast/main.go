// Command daycast runs the Daycast server: it fetches dashboard data, serves
// display pages and keeps a Cast receiver on a TV.
//
// Usage:
//
//	daycast            start the server
//	daycast discover   list Cast devices on the local network
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
	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/datahub"
	"github.com/stijnvandepol/daycast/internal/display"
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
	if len(os.Args) > 1 && os.Args[1] == "discover" {
		err = discover(ctx)
	} else if len(os.Args) > 1 {
		err = fmt.Errorf("unknown command %q; use no command to start the server, or \"discover\"", os.Args[1])
	} else {
		err = serve(ctx, logger)
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
	fmt.Fprintln(w, "NAME\tMODEL\tADDRESS\tID")
	for _, d := range devices {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.Name, d.Model, d.Addr, d.ID)
	}
	return w.Flush()
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
	store, err := datahub.NewFileStore(filepath.Join(cfg.dataDir, "cache"))
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
	hub = datahub.New(store, logger, publish)
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
			Displays: broadcaster,
			Web:      frontend,
			Discover: cast.Discover,
			Logger:   logger,
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
