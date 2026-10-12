package departures

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// DefaultBaseURL is OVapi. It is plain HTTP on purpose: the HTTPS address
// of v0.ovapi.nl answers with a certificate for another domain (1313.nl),
// so no verifying client can use it. The feed is public timetable data
// and no secret travels with the request. Change this to https if OVapi
// ever fixes its certificate.
const DefaultBaseURL = "http://v0.ovapi.nl"

// maxAnswerBytes bounds what is read from OVapi. The list of all stop
// areas is about 650 KB and the departures of ten busy stops are a few
// hundred, so this stops a broken server without ever cutting a real one.
const maxAnswerBytes = 8 << 20

// Client reads stop areas and departures from OVapi.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash to OVapi, which is run by volunteers and
	// asks to know who is calling.
	UserAgent string
	// Now is the clock that tells which departures are in the past.
	Now func() time.Time
}

// Boards returns the next departures of each stop area in codes, using one
// request for all of them. Every code is in the answer, with an empty list
// when OVapi has no departures for it. A code that is not a valid stop area
// code fails the call, because only validated codes may reach the address.
func (c Client) Boards(ctx context.Context, codes []string) (map[string][]Departure, error) {
	if len(codes) == 0 {
		return map[string][]Departure{}, nil
	}
	for _, code := range codes {
		if !ValidCode(code) {
			return nil, fmt.Errorf("stop area code %q is not valid", code)
		}
	}
	body, err := c.get(ctx, "/stopareacode/"+strings.Join(codes, ","))
	if err != nil {
		return nil, err
	}
	return parseBoards(body, codes, c.Now())
}

// apiStopArea is one entry of the list of all stop areas.
type apiStopArea struct {
	Town string `json:"TimingPointTown"`
	Name string `json:"TimingPointName"`
}

// Stops returns every stop area OVapi lists, sorted by name. The list holds
// about 4600 entries, most of them without departures at any moment.
func (c Client) Stops(ctx context.Context) ([]Stop, error) {
	body, err := c.get(ctx, "/stopareacode")
	if err != nil {
		return nil, err
	}
	var all map[string]apiStopArea
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, fmt.Errorf("decode ovapi stop areas: %w", err)
	}
	stops := make([]Stop, 0, len(all))
	for code, area := range all {
		name := label(area.Town, area.Name)
		if ValidCode(code) && name != "" {
			stops = append(stops, Stop{Code: code, Name: name})
		}
	}
	if len(stops) == 0 {
		return nil, errors.New("ovapi listed no usable stop areas")
	}
	slices.SortFunc(stops, func(a, b Stop) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Code, b.Code))
	})
	return stops, nil
}

// get fetches path and returns the body, at most maxAnswerBytes of it.
func (c Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build ovapi request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request ovapi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ovapi answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read ovapi answer: %w", err)
	}
	if len(body) > maxAnswerBytes {
		return nil, fmt.Errorf("ovapi answer is larger than %d bytes", maxAnswerBytes)
	}
	return body, nil
}
