package fuel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// DefaultBaseURL is the ANWB points-of-interest service, which the ANWB
// route planner and its apps read.
const DefaultBaseURL = "https://api.anwb.nl/routing/points-of-interest/v3/all"

// maxAnswerBytes bounds what is read. A box of 25 km radius around a town
// lists a few hundred stations at about 2 KB each; this stops a broken
// server without ever cutting a real one.
const maxAnswerBytes = 8 << 20

// Client reads petrol stations from the ANWB.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash to the ANWB.
	UserAgent string
}

// Nearby returns the cheapest stations of each query, by the query's Key.
// Queries that share an area share one request. Every key is in the answer,
// with an empty list when no station in the circle sells the fuel. One
// failing request fails the call, so the dashboard keeps the last complete
// answer instead of showing prices for some places and none for others.
func (c Client) Nearby(ctx context.Context, queries []Query) (map[string][]Station, error) {
	answers := map[Area][]poi{}
	out := make(map[string][]Station, len(queries))
	for _, q := range queries {
		if !q.Area.valid() || !ValidType(q.Fuel) {
			return nil, fmt.Errorf("fuel query %q is not valid", q.Key())
		}
		if _, done := answers[q.Area]; !done {
			stations, err := c.stations(ctx, q.Area)
			if err != nil {
				return nil, err
			}
			answers[q.Area] = stations
		}
		out[q.Key()] = pick(answers[q.Area], q.Area, q.Fuel)
	}
	return out, nil
}

// stations fetches every station in the box around the area.
func (c Client) stations(ctx context.Context, a Area) ([]poi, error) {
	minLat, minLon, maxLat, maxLon := a.boundingBox()
	box := make([]string, 0, 4)
	for _, degrees := range []float64{minLat, minLon, maxLat, maxLon} {
		box = append(box, strconv.FormatFloat(degrees, 'f', 6, 64))
	}
	// Digits, points, minus signs and commas need no escaping.
	address := c.BaseURL + "?type-filter=FUEL_STATION&bounding-box-filter=" + strings.Join(box, ",")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build anwb request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request anwb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anwb answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read anwb answer: %w", err)
	}
	if len(body) > maxAnswerBytes {
		return nil, fmt.Errorf("anwb answer is larger than %d bytes", maxAnswerBytes)
	}
	return decode(body)
}
