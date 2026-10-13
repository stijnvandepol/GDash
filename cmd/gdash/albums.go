package main

import (
	"context"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/synology"
)

// albumInterval is how often Photos widgets that show an album of Immich
// or Synology Photos get another random part of it, and new photos in it
// show up.
const albumInterval = 30 * time.Minute

// albumSources returns the sources of the albums that Photos widgets
// show. Only the IDs of the photos are kept and cached; the photos
// themselves go from Immich or the NAS to the screens through the server,
// never to disk.
func (d *dataSources) albumSources(cfg settings.Sources, dashboards []display.Dashboard) []datahub.Source {
	var sources []datahub.Source
	if album := cfg.Immich; album != nil && display.UsesAlbum(dashboards, display.ImmichSource) {
		sources = append(sources, datahub.Source{
			Key:      display.ImmichSource,
			Interval: albumInterval,
			Version:  d.fingerprint(*album),
			Fetch: func(ctx context.Context) (any, error) {
				return d.immich.Photos(ctx, album.Connection, album.AlbumID)
			},
		})
	}
	if album := cfg.Synology; album != nil && display.UsesAlbum(dashboards, display.SynologySource) {
		sources = append(sources, datahub.Source{
			Key:      display.SynologySource,
			Interval: albumInterval,
			Timeout:  synology.Timeout * 2,
			Version:  d.fingerprint(*album),
			Fetch:    func(ctx context.Context) (any, error) { return d.synology.Photos(ctx, *album) },
		})
	}
	return sources
}
