package main

import (
	"context"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/settings"
)

// immichInterval is how often Photos widgets that show the Immich album get
// another random part of it, and new photos in it show up.
const immichInterval = 30 * time.Minute

// immichSources returns the Immich album's source when a Photos widget
// shows it. Only the IDs of the photos are kept and cached; the photos
// themselves go from Immich to the screens through the server, never to
// disk.
func (d *dataSources) immichSources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	album := cfg.Immich
	if album == nil || !display.UsesImmich(dashboards) {
		return nil
	}
	return []datahub.Source{{
		Key:      display.ImmichSource,
		Interval: immichInterval,
		Version:  d.fingerprint(*album),
		Fetch: func(ctx context.Context) (any, error) {
			return d.immich.Photos(ctx, album.Connection, album.AlbumID)
		},
	}}
}
