package p2000

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
)

// DefaultBaseURL is alarmeringen.nl, which publishes the feeds under CC
// BY-NC-ND 3.0: the widget credits it and shows its texts unchanged.
const DefaultBaseURL = "https://alarmeringen.nl"

// maxFeedBytes is far above a real feed (100 alerts are about 40 KB) and
// stops a broken server from making GDash read without end.
const maxFeedBytes = 2 << 20

// Errors of the place test.
var (
	// ErrUnknownPlace: alarmeringen.nl has no place by that name.
	ErrUnknownPlace = errors.New("alarmeringen.nl knows no place by that name")
	// ErrPlaceWithoutAlerts: the place exists but has no recent alerts, so
	// its region cannot be read from them.
	ErrPlaceWithoutAlerts = errors.New("the place has no recent alerts to read its region from")
	errNotFound           = errors.New("not found")
	errFeedTooLarge       = errors.New("feed is larger than the limit")
)

// Client fetches feeds with conditional requests: an unchanged feed costs
// alarmeringen.nl a 304 instead of the whole document.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP      *http.Client
	UserAgent string

	mu    sync.Mutex
	cache map[string]cachedFeed
}

type cachedFeed struct {
	etag, lastModified string
	alerts             []Alert
}

// Alerts returns the alerts of every feed, by Feed.Key. The feeds download
// in parallel; any failing feed fails the whole set, so the screens keep the
// last complete one instead of quietly losing a region.
func (c *Client) Alerts(ctx context.Context, feeds []Feed) (map[string][]Alert, error) {
	c.forgetExcept(feeds)
	alerts := make([][]Alert, len(feeds))
	errs := make([]error, len(feeds))
	var wg sync.WaitGroup
	for i, f := range feeds {
		wg.Go(func() {
			alerts[i], errs[i] = c.feed(ctx, f)
			if errs[i] != nil {
				errs[i] = fmt.Errorf("p2000 feed %s: %w", f.Key(), errs[i])
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	out := make(map[string][]Alert, len(feeds))
	for i, f := range feeds {
		out[f.Key()] = alerts[i]
	}
	return out, nil
}

func (c *Client) feed(ctx context.Context, f Feed) ([]Alert, error) {
	if !ValidRegion(f.Region) || !ValidDiscipline(f.Discipline) {
		return nil, fmt.Errorf("unknown region or discipline")
	}
	path := "/feeds/region/" + f.Region + "/" + f.Discipline + ".rss"
	cached, hasCache := c.cached(path)
	body, resp, err := c.get(ctx, path, cached, hasCache)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return cached.alerts, nil
	}
	alerts, err := Parse(bytes.NewReader(body), f.Discipline)
	if err != nil {
		return nil, err
	}
	c.store(path, cachedFeed{etag: resp.Header.Get("ETag"), lastModified: resp.Header.Get("Last-Modified"), alerts: alerts})
	return alerts, nil
}

// FindPlace looks a place up by its name: it reads the place's own feed
// once and takes the place's slug and region from the links of its alerts.
// It returns ErrUnknownPlace or ErrPlaceWithoutAlerts when it cannot.
func (c *Client) FindPlace(ctx context.Context, name string) (Place, error) {
	slug := SlugOf(name)
	if !ValidSlug(slug) {
		return Place{}, ErrUnknownPlace
	}
	body, _, err := c.get(ctx, "/feeds/city/"+slug+".rss", cachedFeed{}, false)
	if errors.Is(err, errNotFound) {
		return Place{}, ErrUnknownPlace
	}
	if err != nil {
		return Place{}, err
	}
	items, err := parseItems(bytes.NewReader(body))
	if err != nil {
		return Place{}, err
	}
	// The feed of "Den Haag" links to den-haag, and so does that of
	// "'s-Gravenhage": the links say how alerts name the place.
	for _, it := range items {
		if region, place := regionAndPlace(it.Link); region != "" && place != "" {
			// One line of plain spaces: a pasted tab or line break is no name.
			return Place{Name: strings.Join(strings.Fields(name), " "), Slug: place, Region: region}, nil
		}
	}
	return Place{}, ErrPlaceWithoutAlerts
}

// get fetches path. It returns a nil body for a feed that has not changed
// since cached.
func (c *Client) get(ctx context.Context, path string, cached cachedFeed, hasCache bool) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
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
		return nil, nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified && hasCache:
		return nil, resp, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, resp, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, resp, fmt.Errorf("alarmeringen.nl answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, resp, fmt.Errorf("read: %w", err)
	}
	if len(body) > maxFeedBytes {
		return nil, resp, errFeedTooLarge
	}
	return body, resp, nil
}

func (c *Client) cached(path string) (cachedFeed, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.cache[path]
	return f, ok
}

func (c *Client) store(path string, f cachedFeed) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = map[string]cachedFeed{}
	}
	c.cache[path] = f
}

// forgetExcept drops the conditional-request state of feeds no longer asked
// for.
func (c *Client) forgetExcept(feeds []Feed) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for path := range c.cache {
		if !slices.ContainsFunc(feeds, func(f Feed) bool { return strings.Contains(path, "/"+f.Key()+".rss") }) {
			delete(c.cache, path)
		}
	}
}
