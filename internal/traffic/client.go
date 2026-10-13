package traffic

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// maxAnswerBytes bounds what is read. The national list is about 210 KB on
// a normal weekday; this stops a broken server without cutting a busy day.
const maxAnswerBytes = 8 << 20

// callerID is what the ANWB's own website sends to name itself; the service
// answers without it, but it marks the request as a traffic list reader.
const callerID = "routing/traffic-info-web"

// StatusError is an answer other than 200 OK.
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("anwb traffic answered %d %s", e.Code, http.StatusText(e.Code))
}

// Refused reports whether the ANWB refused the request itself (a 4xx other
// than a timeout or "too many requests"): asking again will not help.
func (e *StatusError) Refused() bool {
	return e.Code >= 400 && e.Code < 500 && e.Code != http.StatusRequestTimeout && e.Code != http.StatusTooManyRequests
}

// Client reads the national traffic list from the ANWB.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash and its version to the ANWB.
	UserAgent string
}

// Fetch returns the jams, roadworks and speed checks of the whole country.
func (c Client) Fetch(ctx context.Context) (Report, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL, nil)
	if err != nil {
		return Report{}, fmt.Errorf("build anwb traffic request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-anwb-caller-id", callerID)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("request anwb traffic: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Report{}, &StatusError{Code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return Report{}, fmt.Errorf("read anwb traffic answer: %w", err)
	}
	if len(body) > maxAnswerBytes {
		return Report{}, fmt.Errorf("anwb traffic answer is larger than %d bytes", maxAnswerBytes)
	}
	return decode(body)
}
