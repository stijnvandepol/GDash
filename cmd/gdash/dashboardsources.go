package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/fuel"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/waste"
	"github.com/stijnvandepol/gdash/internal/weather"
)

const (
	// A pickup calendar changes a few times a year; this is plenty to see a
	// moved pickup the evening before.
	wasteInterval = 30 * time.Minute
	wasteTimeout  = time.Minute
	// wasteDays is the days after today that pickups are fetched for. The
	// widget shows the next 14; the extra day covers the hours until the
	// next refresh crosses midnight.
	wasteDays = 15
	// EnergyZero publishes the prices of tomorrow in the afternoon; an
	// hourly refresh shows them within the hour, and the price of the
	// current hour is read from the list, not fetched again.
	energyPriceInterval = time.Hour
	// dutchTimeZone is where the price market's days start and end.
	dutchTimeZone = "Europe/Amsterdam"
	// Station prices change a few times a day, and the ANWB's own apps
	// show them as they are; a quarter of an hour is plenty and polite.
	fuelInterval = 15 * time.Minute
	// A refresh makes one request per distinct place and radius.
	fuelTimeout = 2 * time.Minute
	// maxFuelQueries bounds those requests for a screen full of widgets.
	maxFuelQueries = 10
)

// dashboardSources returns the sources that exist because a dashboard has
// a widget for them, rather than because something is set up under
// Settings. A service is only asked for data that a screen shows.
func (d *dataSources) dashboardSources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	var sources []datahub.Source
	if ids := display.WasteCalendarIDs(dashboards); len(ids) > 0 && len(cfg.Calendars) > 0 {
		sources = append(sources, d.wasteSource(cfg.Calendars, ids))
	}
	if display.UsesWidget(dashboards, "energyprice") {
		sources = append(sources, datahub.Source{
			Key:      display.EnergyPriceSource,
			Interval: energyPriceInterval,
			Version:  "energyzero",
			Fetch:    func(ctx context.Context) (any, error) { return d.energy.Prices(ctx) },
		})
	}
	if source, ok := d.fuelSource(cfg.Location, display.FuelQueries(dashboards)); ok {
		sources = append(sources, source)
	}
	return sources
}

// fuelSource fetches the cheapest stations for the places that Fuel widgets
// show. A widget without a place follows the weather location; without one
// it has nothing to fetch, and its screen says to set one up.
func (d *dataSources) fuelSource(location *weather.Location, queries []fuel.Query) (datahub.Source, bool) {
	var resolved []fuel.Query
	for _, q := range queries {
		if q.Here {
			if location == nil {
				continue
			}
			q.Area.Latitude, q.Area.Longitude = location.Latitude, location.Longitude
		}
		resolved = append(resolved, q)
	}
	if len(resolved) == 0 {
		return datahub.Source{}, false
	}
	resolved = resolved[:min(len(resolved), maxFuelQueries)]
	return datahub.Source{
		Key:      display.FuelSource,
		Interval: fuelInterval,
		Timeout:  fuelTimeout,
		Version:  d.fingerprint(resolved),
		Fetch:    func(ctx context.Context) (any, error) { return d.fuel.Nearby(ctx, resolved) },
	}, true
}

// wasteSource fetches the pickups of the calendars that waste widgets name.
// A calendar that is gone (deleted, or unreadable) is left out of the
// answer, which the widget shows as a calendar it cannot find.
func (d *dataSources) wasteSource(all []calendar.Source, ids []int64) datahub.Source {
	var chosen []calendar.Source
	for _, c := range all {
		if slices.Contains(ids, c.ID) {
			chosen = append(chosen, c)
		}
	}
	return datahub.Source{
		Key:      display.WasteSource,
		Interval: wasteInterval,
		Timeout:  wasteTimeout,
		Version:  d.fingerprint(chosen),
		Fetch: func(ctx context.Context) (any, error) {
			return d.wastePickups(ctx, chosen)
		},
	}
}

// wastePickups reads each calendar on its own, so the pickups can be told
// apart by calendar. It fails when any calendar fails, so the widgets keep
// the last complete answer instead of quietly showing half of it.
func (d *dataSources) wastePickups(ctx context.Context, calendars []calendar.Source) (waste.Report, error) {
	report := waste.Report{}
	var errs []error
	for _, c := range calendars {
		events, err := d.calendar.Upcoming(ctx, []calendar.Source{c}, wasteDays)
		if err != nil {
			// The error never holds the address; see calendar.withoutURL.
			errs = append(errs, fmt.Errorf("waste calendar %d: %w", c.ID, err))
			continue
		}
		report[strconv.FormatInt(c.ID, 10)] = waste.Pickups(events)
	}
	return report, errors.Join(errs...)
}
