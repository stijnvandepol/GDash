package main

import (
	"context"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/homey"
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
	// homeyPushFreshFor is how long a snapshot that a Homey sent counts as
	// current: its Flow runs every minute, so three minutes allows for two
	// late runs before the widgets show that the data is old.
	homeyPushFreshFor = 3 * time.Minute
)

// homeyPushSources returns the sources of a Homey that sends its snapshot
// every minute: the whole snapshot, which the editor lists values and
// people from, and the values and presence derived from it, which the
// screens narrow to their own widgets like polled ones. They are all kept
// whether or not a dashboard shows Homey, so the editor can list values
// before there is a widget; the snapshot holds the whole home, also on
// disk, so a restart does not blank the widgets.
func (d *dataSources) homeyPushSources() []datahub.Source {
	keys := []string{display.HomeySnapshotSource, display.HomeySource, display.HomeyUsersSource}
	sources := make([]datahub.Source, len(keys))
	for i, key := range keys {
		sources[i] = datahub.Source{
			Key: key, Interval: homeyInterval, FreshFor: homeyPushFreshFor,
			// Not the polled sources' version, so switching modes starts over.
			Version: d.fingerprint(struct{ Mode string }{settings.HomeyPush}),
		}
	}
	return sources
}

// receiveHomey stores a snapshot that a Homey sent, with the values and
// presence the screens read, as one change.
func (d *dataSources) receiveHomey(snapshot homey.Snapshot) error {
	values, err := display.HomeySnapshotData(snapshot)
	if err != nil {
		return err
	}
	return d.hub.Put(values)
}

// homeySources returns the Homey sources that dashboards need: the values
// of the capabilities that Homey widgets show and whether the people that
// Who's home widgets look for are home. Only those are kept, so the cache
// on disk holds no more of the home than the screens show. A Homey that
// sends its own snapshots is the exception, see homeyPushSources.
func (d *dataSources) homeySources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	if cfg.HomeyPushed {
		return d.homeyPushSources()
	}
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
