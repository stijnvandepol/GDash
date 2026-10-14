package calendar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// maxEvents keeps a snapshot well below the 64 KB Cast message limit.
	maxEvents = 50
	// parallelFetches bounds how many calendars download at once: each may
	// keep up to maxKeptBytes, and a home server has little memory.
	parallelFetches = 4
	userAgent       = "GDash (self-hosted TV dashboard; +https://github.com/stijnvandepol/gdash)"
)

// Source is one calendar feed. Its URL is a secret: the iCal address of a
// Google, Outlook or iCloud calendar gives read access to everyone who has
// it, so it must never appear in logs or error messages.
type Source struct {
	// ID is the calendar's number under Integrations; widgets that read one
	// calendar, such as waste collection, name it by this.
	ID   int64
	Name string
	URL  string
	// Color is copied onto every event of the calendar, for the TV.
	Color string
}

// ParseSources reads a comma-separated list of ICS URLs, as configured in
// GDASH_CALENDAR_URLS. webcal:// is accepted and fetched over https, as
// calendar apps do.
func ParseSources(list string) ([]Source, error) {
	var sources []Source
	for i, raw := range strings.Split(list, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("calendar %d is not a URL", i+1)
		}
		switch u.Scheme {
		case "webcal":
			u.Scheme = "https"
		case "http", "https":
		default:
			return nil, fmt.Errorf("calendar %d must be an http, https or webcal URL", i+1)
		}
		sources = append(sources, Source{Name: fmt.Sprintf("Calendar %d", i+1), URL: u.String()})
	}
	return sources, nil
}

// Client fetches calendar feeds.
type Client struct {
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// Location is the server's time zone, for "today" and floating times.
	Location *time.Location
	Now      func() time.Time
}

// Upcoming returns the events from the start of today for the given number
// of days, merged from all sources and sorted. It fails when any source
// fails, so the dashboard keeps the last complete list instead of quietly
// showing half of it.
func (c *Client) Upcoming(ctx context.Context, sources []Source, days int) ([]Event, error) {
	w := c.window(days)

	// Calendars download in parallel, so a few slow ones do not add up to
	// more than the refresh may take.
	found := make([][]Event, len(sources))
	errs := make([]error, len(sources))
	slots := make(chan struct{}, parallelFetches)
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			var err error
			found[i], err = c.read(ctx, src, w)
			if err != nil {
				errs[i] = fmt.Errorf("calendar %d: %w", i+1, err)
			}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	events := []Event{}
	for _, f := range found {
		events = append(events, f...)
	}
	return sortedFirst(events), nil
}

// Read returns the events of one calendar as Upcoming does, with an error
// that does not say which calendar, for a test of an address before it is
// stored.
func (c *Client) Read(ctx context.Context, src Source, days int) ([]Event, error) {
	events, err := c.read(ctx, src, c.window(days))
	if err != nil {
		return nil, err
	}
	return sortedFirst(events), nil
}

// window runs from the start of today for the given number of days.
func (c *Client) window(days int) Window {
	now := c.Now().In(c.Location)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.Location)
	return Window{From: from, To: from.AddDate(0, 0, days), Location: c.Location}
}

// read fetches and parses one calendar, with its colour on every event.
// The feed is parsed while it downloads; see trim.
func (c *Client) read(ctx context.Context, src Source, w Window) ([]Event, error) {
	body, err := c.open(ctx, src)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	events, err := Parse(body, w, src.Name)
	if err != nil {
		return nil, withoutURL(err)
	}
	for i := range events {
		events[i].Color = src.Color
	}
	return events, nil
}

// sortedFirst sorts events and keeps the first maxEvents.
func sortedFirst(events []Event) []Event {
	Sort(events)
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}
	return events
}

// StatusError is an answer other than 200 OK from a calendar server, such
// as 404 for the public address of a calendar that is not public.
type StatusError struct {
	Code   int
	Status string
}

func (e *StatusError) Error() string { return "calendar server answered " + e.Status }

// open requests a calendar and returns its body. The content type is not
// checked: servers send ICS as text/calendar, text/plain or a download, so
// Parse tells a calendar from a web page by its first line.
func (c *Client) open(ctx context.Context, src Source) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return nil, errors.New("invalid calendar URL")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, withoutURL(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &StatusError{Code: resp.StatusCode, Status: resp.Status}
	}
	return resp.Body, nil
}

// withoutURL drops the request URL that net/http puts in its errors: for a
// calendar that URL is the secret.
func withoutURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("request failed: %w", urlErr.Err)
	}
	return err
}
