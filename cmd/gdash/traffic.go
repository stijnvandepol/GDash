package main

import (
	"context"
	"fmt"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/traffic"
	"github.com/stijnvandepol/gdash/internal/weather"
)

const (
	// The paths on GDASH_TEST_UPSTREAM that stand in for the ANWB, the
	// Overpass API and the KNMI; web/e2e/upstream.ts serves them.
	testANWBTrafficPath = "/anwb/traffic"
	testOverpassPath    = "/overpass/api/interpreter"
	testKNMIPath        = "/knmi/edr"

	// The hub asks the traffic source this often; the guard makes sure the
	// ANWB is asked at most once per traffic.MinInterval whatever happens.
	trafficInterval = traffic.MinInterval
	// maxTrafficQueries bounds the work and the snapshot for a screen full
	// of widgets; all queries share one request.
	maxTrafficQueries = 10
	// The camera list is a week old at most (speedcameras.MaxAge); asking
	// the store a few times a day finds out when it is due, without a
	// request.
	camerasInterval = 6 * time.Hour
	// One Overpass query of the whole country may take its full 90 seconds.
	camerasTimeout = 2 * time.Minute
	overpassHTTP   = 2 * time.Minute
	// KNMI stations report every 10 minutes.
	knmiInterval = 10 * time.Minute
	// A refresh may ask a few stations after the station list.
	knmiTimeout = time.Minute
)

// trafficSources returns the sources of the Traffic widgets: their data,
// and the camera list when a widget shows cameras. Nothing is fetched from
// the ANWB unless the admin turned traffic info on and acknowledged its
// terms; a widget then still shows fixed cameras.
func (d *dataSources) trafficSources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	queries := display.TrafficQueries(dashboards)
	var home *traffic.Point
	if loc := cfg.Location; loc != nil {
		home = &traffic.Point{Latitude: loc.Latitude, Longitude: loc.Longitude}
	}
	var resolved []traffic.Query
	for _, q := range queries {
		// A widget that watches a radius needs the weather location; without
		// one its screen says to set one up.
		if q.NearHome() && home == nil {
			continue
		}
		resolved = append(resolved, q)
	}
	resolved = resolved[:min(len(resolved), maxTrafficQueries)]
	showsCameras := display.TrafficShows(dashboards, traffic.ShowCameras)
	if len(resolved) == 0 || (!cfg.TrafficInfo && !showsCameras) {
		return nil
	}
	anwb := cfg.TrafficInfo
	trafficData := datahub.Source{
		Key:      display.TrafficSource,
		Interval: trafficInterval,
		Version:  d.fingerprint(struct{ Queries, Home, ANWB, Cameras any }{resolved, home, anwb, showsCameras}),
		Fetch: func(ctx context.Context) (any, error) {
			return d.trafficResults(ctx, anwb, showsCameras, resolved, home)
		},
	}
	if showsCameras {
		// The first refresh may wait for the first camera list.
		trafficData.Timeout = camerasTimeout
	}
	sources := []datahub.Source{trafficData}
	if showsCameras {
		sources = append(sources, datahub.Source{
			Key:      display.CamerasSource,
			Interval: camerasInterval,
			Timeout:  camerasTimeout,
			Version:  "openstreetmap",
			Fetch:    func(ctx context.Context) (any, error) { return d.cameras.Refresh(ctx) },
		})
	}
	return sources
}

// trafficResults builds what each query shows from one national report and
// the camera list. A failing ANWB request fails the refresh, so screens keep
// the last good answer and mark it as old. Without a camera list yet it asks
// for one first, so a new widget does not wait hours for its cameras; a
// failing Overpass only leaves the cameras out.
func (d *dataSources) trafficResults(ctx context.Context, anwb, cameras bool, queries []traffic.Query, home *traffic.Point) (map[string]traffic.Result, error) {
	var src traffic.Sources
	if anwb {
		report, err := d.traffic.Fetch(ctx)
		if err != nil {
			return nil, fmt.Errorf("anwb traffic info: %w", err)
		}
		src.Report = &report
	}
	src.Cameras, src.CamerasLoaded = d.cameras.Cameras()
	if cameras && !src.CamerasLoaded {
		if _, err := d.cameras.Refresh(ctx); err != nil {
			d.logger.Warn("could not fetch the fixed camera list; traffic widgets show it later", "error", err)
		}
		src.Cameras, src.CamerasLoaded = d.cameras.Cameras()
	}
	now := time.Now()
	out := make(map[string]traffic.Result, len(queries))
	for _, q := range queries {
		out[q.Key()] = traffic.Select(src, q, home, now)
	}
	return out, nil
}

// knmiSource fetches what the KNMI station nearest to the weather location
// measured, while a Weather widget shows measured values and a key is set.
func (d *dataSources) knmiSource(cfg settings.Sources, dashboards []display.Dashboard) (datahub.Source, bool) {
	loc, key := cfg.Location, cfg.KNMIKey
	if loc == nil || key == "" || !display.ShowsMeasuredWeather(dashboards) {
		return datahub.Source{}, false
	}
	return datahub.Source{
		Key:      display.KNMISource,
		Interval: knmiInterval,
		Timeout:  knmiTimeout,
		// Keyed, so the key cannot be read from the cache file.
		Version: d.fingerprint(struct {
			Location weather.Location
			Key      string
		}{*loc, key}),
		Fetch: func(ctx context.Context) (any, error) {
			return d.knmi.Latest(ctx, key, loc.Latitude, loc.Longitude)
		},
	}, true
}
