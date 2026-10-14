package main

import (
	"github.com/stijnvandepol/gdash/internal/news"
)

// testNewsPath is where GDASH_TEST_UPSTREAM serves a stand-in for every
// feed of the news catalogue, as <path><catalogue id>; web/e2e/upstream.ts
// serves it.
const testNewsPath = "/news/"

// newsFeeds returns the feeds to fetch. In the end-to-end tests a feed of
// the catalogue is read from the stand-in instead of its publisher, so the
// tests choose sources from the real catalogue without the internet; an
// address of one's own is fetched as it is.
func (d *dataSources) newsFeeds(feeds []news.Source) []news.Source {
	if d.newsStandIn == "" {
		return feeds
	}
	out := make([]news.Source, len(feeds))
	for i, f := range feeds {
		if entry, ok := news.CatalogueEntryByURL(f.URL); ok {
			f.URL = d.newsStandIn + entry.ID
		}
		out[i] = f
	}
	return out
}
