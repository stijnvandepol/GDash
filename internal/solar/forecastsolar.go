package solar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultForecastSolarURL is Forecast.Solar's public estimate endpoint. It
// needs no key and allows 12 requests per hour from one address.
const DefaultForecastSolarURL = "https://api.forecast.solar/estimate"

// ForecastSolar asks Forecast.Solar, which models the panels itself, for
// its estimate. Its free use covers today and tomorrow and one array per
// request.
type ForecastSolar struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

type estimateResponse struct {
	Result *struct {
		// WattHoursPeriod is the energy in the period that ends at each
		// time: the hour before, or less at sunrise and sunset.
		WattHoursPeriod map[string]float64 `json:"watt_hours_period"`
	} `json:"result"`
}

var errNoEstimate = errors.New("forecast.solar response has no estimate")

// Forecast returns the estimate for setup, with one request per array.
// The performance ratio does not apply: Forecast.Solar has its own.
func (c ForecastSolar) Forecast(ctx context.Context, setup Setup) (Report, error) {
	energy := hourly{}
	var first *time.Time
	for _, array := range setup.Arrays {
		periods, err := c.estimate(ctx, setup, array)
		if err != nil {
			return Report{}, err
		}
		for text, wh := range periods {
			end, err := time.Parse(time.RFC3339, text)
			if err != nil {
				return Report{}, fmt.Errorf("forecast.solar time %q: %w", text, err)
			}
			if first == nil || end.Before(*first) {
				first = &end
			}
			// The period that ends at 09:00 is the hour from 08:00; the
			// one that ends at sunset (18:49) is part of the hour from 18:00.
			energy.add(hourStart(end.Add(-time.Second)), max(wh, 0)/1000)
		}
	}
	if first == nil {
		return Report{}, errNoEstimate
	}
	_, offset := first.Zone()
	return Report{
		Source:           SourceForecastSolar,
		UTCOffsetSeconds: offset,
		Days:             energy.days(first.Location(), first.Format(dateLayout)),
	}, nil
}

// estimate asks for one array, with times in ISO 8601 with the location's
// offset.
func (c ForecastSolar) estimate(ctx context.Context, setup Setup, array Array) (map[string]float64, error) {
	address := c.BaseURL + "/" + url.PathEscape(strconv.FormatFloat(setup.Location.Latitude, 'f', 2, 64)) +
		"/" + url.PathEscape(strconv.FormatFloat(setup.Location.Longitude, 'f', 2, 64)) +
		"/" + strconv.Itoa(array.Tilt) + "/" + strconv.Itoa(array.Azimuth) +
		"/" + strconv.FormatFloat(array.PeakKW, 'f', -1, 64) + "?time=iso8601"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build forecast.solar request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request forecast.solar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errors.New("forecast.solar answered 429: more than 12 requests in the last hour from this address")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forecast.solar answered %s", resp.Status)
	}
	var body estimateResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode forecast.solar response: %w", err)
	}
	if body.Result == nil || len(body.Result.WattHoursPeriod) == 0 {
		return nil, errNoEstimate
	}
	return body.Result.WattHoursPeriod, nil
}
