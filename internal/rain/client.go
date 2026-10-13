package rain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	// DefaultBuienradarURL is Buienradar's short-term rain forecast for a
	// point, which its own apps and widgets read.
	DefaultBuienradarURL = "https://gpsgadget.buienradar.nl/data/raintext"
	// DefaultOpenMeteoURL is Open-Meteo's forecast API.
	DefaultOpenMeteoURL = "https://api.open-meteo.com/v1/forecast"

	// Buienradar's text is about 300 bytes and Open-Meteo's answer under
	// 1 KB; this bounds a broken server without cutting a real answer.
	maxAnswerBytes = 64 << 10
	// openMeteoQuarters covers two hours from now with room to spare, as
	// the first quarter Open-Meteo sends may already have begun.
	openMeteoQuarters = 10
)

// Buienradar covers the Netherlands and Belgium. Outside this box it
// answers 404, so it is not asked at all.
const (
	coverageMinLatitude  = 49.4
	coverageMaxLatitude  = 53.8
	coverageMinLongitude = 2.4
	coverageMaxLongitude = 7.3
)

// Client reads rain forecasts.
type Client struct {
	BuienradarURL string
	OpenMeteoURL  string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash to the services.
	UserAgent string
	// Location is the Dutch time zone, in which Buienradar writes times.
	Location *time.Location
	Now      func() time.Time
}

// Forecasts returns the forecast of each query, by the query's Key. Queries
// at the same rounded place share one request. One place that neither
// service can answer fails the call, so the dashboard keeps the last
// complete answer instead of showing rain for some places and none for
// others.
func (c Client) Forecasts(ctx context.Context, queries []Query) (map[string]Forecast, error) {
	type place struct{ latitude, longitude float64 }
	answers := map[place]Forecast{}
	out := make(map[string]Forecast, len(queries))
	for _, q := range queries {
		if !q.valid() {
			return nil, fmt.Errorf("rain query %q is not a place on earth", q.Key())
		}
		p := place{Round(q.Latitude), Round(q.Longitude)}
		if _, done := answers[p]; !done {
			forecast, err := c.forecast(ctx, p.latitude, p.longitude)
			if err != nil {
				return nil, err
			}
			answers[p] = forecast
		}
		out[q.Key()] = answers[p]
	}
	return out, nil
}

// forecast asks Buienradar, and Open-Meteo when Buienradar cannot answer.
// When both fail, both reasons are in the error.
func (c Client) forecast(ctx context.Context, latitude, longitude float64) (Forecast, error) {
	var primary error
	if covered(latitude, longitude) {
		points, err := c.buienradar(ctx, latitude, longitude)
		if err == nil {
			return Forecast{Source: SourceBuienradar, StepMinutes: buienradarStepMinutes, Points: points}, nil
		}
		primary = err
	}
	points, err := c.openMeteo(ctx, latitude, longitude)
	if err != nil {
		return Forecast{}, errors.Join(primary, err)
	}
	return Forecast{Source: SourceOpenMeteo, StepMinutes: openMeteoStepMinutes, Points: points}, nil
}

func covered(latitude, longitude float64) bool {
	return latitude >= coverageMinLatitude && latitude <= coverageMaxLatitude &&
		longitude >= coverageMinLongitude && longitude <= coverageMaxLongitude
}

func (c Client) buienradar(ctx context.Context, latitude, longitude float64) ([]Point, error) {
	query := url.Values{
		"lat": {strconv.FormatFloat(latitude, 'f', 2, 64)},
		"lon": {strconv.FormatFloat(longitude, 'f', 2, 64)},
	}
	body, err := c.get(ctx, "buienradar", c.BuienradarURL+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	return parseBuienradar(string(body), c.Now(), c.Location)
}

func (c Client) openMeteo(ctx context.Context, latitude, longitude float64) ([]Point, error) {
	query := url.Values{
		"latitude":             {strconv.FormatFloat(latitude, 'f', 2, 64)},
		"longitude":            {strconv.FormatFloat(longitude, 'f', 2, 64)},
		"minutely_15":          {"precipitation"},
		"forecast_minutely_15": {strconv.Itoa(openMeteoQuarters)},
		"timezone":             {"auto"},
	}
	body, err := c.get(ctx, "open-meteo", c.OpenMeteoURL+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	return parseOpenMeteo(body, c.Now())
}

// get fetches address and returns its body, at most maxAnswerBytes.
func (c Client) get(ctx context.Context, service, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", service, err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", service, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", service, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s answer: %w", service, err)
	}
	if len(body) > maxAnswerBytes {
		return nil, fmt.Errorf("%s answer is larger than %d bytes", service, maxAnswerBytes)
	}
	return body, nil
}
