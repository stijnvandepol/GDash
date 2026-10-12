package main

import (
	"context"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/settings"
)

const (
	// Homey runs in the home, like Home Assistant: once a minute keeps a
	// sensor and who is home current without a request storm. All
	// devices come in one request, and all users in another.
	homeyInterval = time.Minute
	homeyTimeout  = 30 * time.Second
	// The caps bound what is kept and cached, not the requests.
	maxHomeyCapabilities = 40
	maxHomeyUsers        = 20
)

// homeySources returns the Homey sources that dashboards need: the values
// of the capabilities that Homey widgets show and whether the people that
// Who's home widgets look for are home. Only those are kept, so the cache
// on disk holds no more of the home than the screens show.
func (d *dataSources) homeySources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	conn := cfg.Homey
	if conn == nil {
		return nil
	}
	var sources []datahub.Source
	if keys := display.HomeyCapabilityKeys(dashboards); len(keys) > 0 {
		keys = keys[:min(len(keys), maxHomeyCapabilities)]
		sources = append(sources, datahub.Source{
			Key:      display.HomeySource,
			Interval: homeyInterval,
			Timeout:  homeyTimeout,
			Version:  d.fingerprint(struct{ Connection, Keys any }{*conn, keys}),
			Fetch:    func(ctx context.Context) (any, error) { return d.homey.States(ctx, *conn, keys) },
		})
	}
	if users := display.HomeyUserIDs(dashboards); len(users) > 0 {
		users = users[:min(len(users), maxHomeyUsers)]
		sources = append(sources, datahub.Source{
			Key:      display.HomeyUsersSource,
			Interval: homeyInterval,
			Timeout:  homeyTimeout,
			Version:  d.fingerprint(struct{ Connection, Users any }{*conn, users}),
			Fetch:    func(ctx context.Context) (any, error) { return d.homey.Presence(ctx, *conn, users) },
		})
	}
	return sources
}
