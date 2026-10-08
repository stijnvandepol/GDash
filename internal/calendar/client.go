package calendar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxCalendarBytes is far above a family calendar with years of history
	// and stops a broken server from making Daycast read without end.
	maxCalendarBytes = 10 << 20
	// maxEvents keeps a snapshot well below the 64 KB Cast message limit.
	maxEvents = 50
	userAgent = "Daycast (self-hosted TV dashboard; +https://github.com/stijnvandepol/daycast)"
)

// Source is one calendar feed. Its URL is a secret: the iCal address of a
// Google, Outlook or iCloud calendar gives read access to everyone who has
// it, so it must never appear in logs or error messages.
type Source struct {
	Name string
	URL  string
}

// ParseSources reads a comma-separated list of ICS URLs, as configured in
// DAYCAST_CALENDAR_URLS. webcal:// is accepted and fetched over https, as
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
	now := c.Now().In(c.Location)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.Location)
	w := Window{From: from, To: from.AddDate(0, 0, days), Location: c.Location}

	events := []Event{}
	for i, src := range sources {
		body, err := c.fetch(ctx, src)
		if err != nil {
			return nil, fmt.Errorf("calendar %d: %w", i+1, err)
		}
		found, err := Parse(bytes.NewReader(body), w, src.Name)
		if err != nil {
			return nil, fmt.Errorf("calendar %d: %w", i+1, err)
		}
		events = append(events, found...)
	}
	Sort(events)
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}
	return events, nil
}

func (c *Client) fetch(ctx context.Context, src Source) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return nil, errors.New("invalid calendar URL")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/calendar")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, withoutURL(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar server answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCalendarBytes+1))
	if err != nil {
		return nil, withoutURL(err)
	}
	if len(body) > maxCalendarBytes {
		return nil, errors.New("calendar is larger than the limit")
	}
	return body, nil
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
