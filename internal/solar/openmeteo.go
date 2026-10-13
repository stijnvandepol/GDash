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

// DefaultOpenMeteoURL is the public Open-Meteo forecast endpoint, the same
// one the weather comes from.
const DefaultOpenMeteoURL = "https://api.open-meteo.com/v1/forecast"

const (
	// forecastDays is today and tomorrow.
	forecastDays = 2
	// openMeteoTime is how Open-Meteo writes a local hour.
	openMeteoTime = "2006-01-02T15:04"
)

// OpenMeteo estimates production from Open-Meteo's global tilted
// irradiance (GTI): the sunlight forecast on the panels' own plane, from
// the cloud forecast, as a mean over the preceding hour.
type OpenMeteo struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

type gtiResponse struct {
	UTCOffsetSeconds *int `json:"utc_offset_seconds"`
	Hourly           struct {
		Time []string   `json:"time"`
		GTI  []*float64 `json:"global_tilted_irradiance"`
	} `json:"hourly"`
}

var errNoIrradiance = errors.New("open-meteo response has no irradiance forecast")

// Forecast returns the estimate for setup, with one request per array:
// Open-Meteo takes one tilt and azimuth per request.
func (c OpenMeteo) Forecast(ctx context.Context, setup Setup) (Report, error) {
	energy := hourly{}
	var offset *int
	var from string
	for _, array := range setup.Arrays {
		body, err := c.irradiance(ctx, setup, array)
		if err != nil {
			return Report{}, err
		}
		if body.UTCOffsetSeconds == nil || len(body.Hourly.Time) == 0 || len(body.Hourly.GTI) == 0 {
			return Report{}, errNoIrradiance
		}
		zone := time.FixedZone("", *body.UTCOffsetSeconds)
		if err := addIrradiance(energy, body, zone, array, setup.PerformanceRatio); err != nil {
			return Report{}, err
		}
		if offset == nil {
			// addIrradiance has parsed the first time, so it is long enough.
			offset, from = body.UTCOffsetSeconds, body.Hourly.Time[0][:len(dateLayout)]
		}
	}
	return Report{
		Source:           SourceOpenMeteo,
		UTCOffsetSeconds: *offset,
		Days:             energy.days(time.FixedZone("", *offset), from),
	}, nil
}

// irradiance asks Open-Meteo for the hourly GTI on one array's plane.
func (c OpenMeteo) irradiance(ctx context.Context, setup Setup, array Array) (gtiResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+openMeteoQuery(setup, array).Encode(), nil)
	if err != nil {
		return gtiResponse{}, fmt.Errorf("build open-meteo solar request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return gtiResponse{}, fmt.Errorf("request open-meteo solar forecast: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return gtiResponse{}, fmt.Errorf("open-meteo answered %s", resp.Status)
	}
	var body gtiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return gtiResponse{}, fmt.Errorf("decode open-meteo solar forecast: %w", err)
	}
	return body, nil
}

// openMeteoQuery asks for today and tomorrow in the location's own time
// zone. The coordinates are rounded to about 1 km, as for the weather.
func openMeteoQuery(setup Setup, array Array) url.Values {
	return url.Values{
		"latitude":      {strconv.FormatFloat(setup.Location.Latitude, 'f', 2, 64)},
		"longitude":     {strconv.FormatFloat(setup.Location.Longitude, 'f', 2, 64)},
		"hourly":        {"global_tilted_irradiance"},
		"tilt":          {strconv.Itoa(array.Tilt)},
		"azimuth":       {strconv.Itoa(array.Azimuth)},
		"timezone":      {"auto"},
		"forecast_days": {strconv.Itoa(forecastDays)},
	}
}

// addIrradiance adds one array's energy per hour. A value at 13:00 is the
// mean of 12:00 to 13:00, so it counts for the hour from 12:00. A missing
// value counts as nothing; a broken time fails the whole answer, so the
// widget keeps its last good estimate.
func addIrradiance(energy hourly, body gtiResponse, zone *time.Location, array Array, ratio float64) error {
	times, values := body.Hourly.Time, body.Hourly.GTI
	for i := range min(len(times), len(values)) {
		end, err := time.ParseInLocation(openMeteoTime, times[i], zone)
		if err != nil {
			return fmt.Errorf("open-meteo time %q: %w", times[i], err)
		}
		kwh := 0.0
		if values[i] != nil {
			kwh = EnergyKWh(*values[i], array.PeakKW, ratio)
		}
		energy.add(end.Add(-time.Hour), kwh)
	}
	return nil
}
