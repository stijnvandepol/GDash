package main

import (
	"context"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/p2000"
)

const (
	// alarmeringen.nl lets its feeds be cached for about a minute and a
	// half; every two minutes shows an alert soon after it went out.
	p2000Interval = 2 * time.Minute
	// A refresh makes one request per feed, in parallel.
	p2000Timeout = time.Minute
	// maxP2000Feeds bounds those requests: a feed is one region and one
	// discipline, so this is four regions with all their disciplines.
	maxP2000Feeds = 20
	// testP2000Path stands in for alarmeringen.nl on GDASH_TEST_UPSTREAM;
	// web/e2e/upstream.ts serves it.
	testP2000Path = "/alarmeringen"
)

// p2000Source fetches the P2000 feeds that P2000 widgets show, at most
// maxP2000Feeds. The rest are published as skipped, so their widgets say
// so instead of showing a list that is quietly incomplete.
func (d *dataSources) p2000Source(feeds []p2000.Feed) (datahub.Source, bool) {
	if len(feeds) == 0 {
		return datahub.Source{}, false
	}
	fetched := feeds[:min(len(feeds), maxP2000Feeds)]
	skipped := []string{}
	for _, f := range feeds[len(fetched):] {
		skipped = append(skipped, f.Key())
	}
	if len(skipped) > 0 {
		d.logger.Warn("the dashboards show more P2000 feeds than GDash fetches; some are left out", "limit", maxP2000Feeds, "skipped", skipped)
	}
	return datahub.Source{
		Key:      display.P2000Source,
		Interval: p2000Interval,
		Timeout:  p2000Timeout,
		Version:  d.fingerprint(feeds),
		Fetch: func(ctx context.Context) (any, error) {
			alerts, err := d.p2000.Alerts(ctx, fetched)
			if err != nil {
				return nil, err
			}
			return p2000.Value{Feeds: alerts, Skipped: skipped}, nil
		},
	}, true
}
