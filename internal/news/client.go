package news

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
)

// maxFeedBytes is far above any real headline feed (the NOS feed with full
// article bodies is about 60 KB) and stops a broken server from making
// Daycast read without end.
const maxFeedBytes = 2 << 20

var errFeedTooLarge = errors.New("feed is larger than the limit")

const userAgent = "Daycast (self-hosted TV dashboard; +https://github.com/stijnvandepol/daycast)"

// Client fetches feeds with conditional requests: when a server sends an
// ETag or Last-Modified, an unchanged feed costs it a 304 instead of the
// whole document.
type Client struct {
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client

	mu    sync.Mutex
	cache map[string]cachedFeed
}

type cachedFeed struct {
	etag         string
	lastModified string
	feed         Feed
}

// Source is a feed as the admin added it.
type Source struct {
	Name string
	URL  string
}

// mergedTitle heads a list with headlines from more than one feed.
const mergedTitle = "News"

// Headlines returns the newest maxItems headlines of all sources. One
// source keeps its own title; several are merged newest first, each item
// naming its feed. Any failing feed fails the whole list, so the dashboard
// keeps the last complete one instead of quietly dropping a feed.
func (c *Client) Headlines(ctx context.Context, sources []Source, maxItems int) (Feed, error) {
	c.forgetExcept(sources)
	if len(sources) == 1 {
		return c.Fetch(ctx, sources[0].URL, maxItems)
	}
	merged := Feed{Title: mergedTitle, Items: []Item{}}
	for i, src := range sources {
		feed, err := c.Fetch(ctx, src.URL, maxItems)
		if err != nil {
			return Feed{}, fmt.Errorf("feed %d: %w", i+1, err)
		}
		for _, item := range feed.Items {
			item.Source = src.Name
			merged.Items = append(merged.Items, item)
		}
	}
	// Stable, so items without a date keep their feed's order at the end.
	slices.SortStableFunc(merged.Items, func(a, b Item) int { return b.Published.Compare(a.Published) })
	if len(merged.Items) > maxItems {
		merged.Items = merged.Items[:maxItems]
	}
	return merged, nil
}

// forgetExcept drops the conditional-request state of removed feeds.
func (c *Client) forgetExcept(sources []Source) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for url := range c.cache {
		if !slices.ContainsFunc(sources, func(s Source) bool { return s.URL == url }) {
			delete(c.cache, url)
		}
	}
}

// Fetch returns the newest maxItems headlines of the feed at url.
func (c *Client) Fetch(ctx context.Context, url string, maxItems int) (Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Feed{}, fmt.Errorf("build feed request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")
	cached, hasCache := c.cached(url)
	if hasCache {
		if cached.etag != "" {
			req.Header.Set("If-None-Match", cached.etag)
		}
		if cached.lastModified != "" {
			req.Header.Set("If-Modified-Since", cached.lastModified)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Feed{}, fmt.Errorf("request feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && hasCache {
		return cached.feed, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Feed{}, fmt.Errorf("feed server answered %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return Feed{}, fmt.Errorf("read feed: %w", err)
	}
	if len(body) > maxFeedBytes {
		return Feed{}, errFeedTooLarge
	}
	feed, err := Parse(bytes.NewReader(body), maxItems)
	if err != nil {
		return Feed{}, err
	}
	c.store(url, cachedFeed{etag: resp.Header.Get("ETag"), lastModified: resp.Header.Get("Last-Modified"), feed: feed})
	return feed, nil
}

func (c *Client) cached(url string) (cachedFeed, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.cache[url]
	return f, ok
}

func (c *Client) store(url string, f cachedFeed) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = map[string]cachedFeed{}
	}
	c.cache[url] = f
}
