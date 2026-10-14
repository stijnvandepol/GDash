package news

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
)

// maxFeedBytes is far above any real headline feed (the NOS feed with full
// article bodies is about 60 KB) and stops a broken server from making
// GDash read without end.
const maxFeedBytes = 2 << 20

var errFeedTooLarge = errors.New("feed is larger than the limit")

const userAgent = "GDash (self-hosted TV dashboard; +https://github.com/stijnvandepol/gdash)"

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
	ID   int64
	Name string
	URL  string
}

// SourceFeed is the newest headlines of one source, under the name the
// admin gave it.
type SourceFeed struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

// Headlines returns the newest maxItems headlines of every source, by the
// source's ID, so a screen can be given only the feeds its widgets show and
// each widget merges its own. Any failing feed fails the whole set, so the
// dashboard keeps the last complete one instead of quietly dropping a feed.
func (c *Client) Headlines(ctx context.Context, sources []Source, maxItems int) (map[string]SourceFeed, error) {
	c.forgetExcept(sources)
	// Feeds download in parallel, so a slow one does not hold up the rest
	// past the refresh's time limit. There are at most a handful.
	feeds := make([]Feed, len(sources))
	errs := make([]error, len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			feed, err := c.Fetch(ctx, src.URL, maxItems)
			if err != nil {
				errs[i] = fmt.Errorf("feed %d: %w", src.ID, err)
			}
			feeds[i] = feed
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	out := make(map[string]SourceFeed, len(sources))
	for i, src := range sources {
		out[strconv.FormatInt(src.ID, 10)] = SourceFeed{Name: src.Name, Items: feeds[i].Items}
	}
	return out, nil
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
