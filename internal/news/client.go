package news

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
