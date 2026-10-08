// Package weather fetches current weather from Open-Meteo, which needs no
// API key and uses the KNMI HARMONIE-AROME model for the Netherlands.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
)

// DefaultBaseURL is the public Open-Meteo forecast endpoint.
const DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"

// Open-Meteo answers are a few hundred bytes; anything far larger is not a
// forecast and is not read into memory.
const maxResponseBytes = 1 << 20

// Location is a point on earth, rounded to two decimals (about 1 km). The
// weather service never needs the exact address, so it never gets it.
type Location struct {
	Latitude  float64
	Longitude float64
}

// NewLocation validates and rounds a coordinate pair.
func NewLocation(latitude, longitude float64) (Location, error) {
	if math.IsNaN(latitude) || latitude < -90 || latitude > 90 {
		return Location{}, fmt.Errorf("latitude %v is outside -90..90", latitude)
	}
	if math.IsNaN(longitude) || longitude < -180 || longitude > 180 {
		return Location{}, fmt.Errorf("longitude %v is outside -180..180", longitude)
	}
	return Location{Latitude: round2(latitude), Longitude: round2(longitude)}, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Report is the weather the dashboard shows.
type Report struct {
	TemperatureC float64 `json:"temperatureC"`
	WeatherCode  int     `json:"weatherCode"`
	IsDay        bool    `json:"isDay"`
	WindKmh      float64 `json:"windKmh"`
	TodayMaxC    float64 `json:"todayMaxC"`
	TodayMinC    float64 `json:"todayMinC"`
	// ObservedAt is the local time of the observation as Open-Meteo reports
	// it, for example "2026-10-08T13:30".
	ObservedAt string `json:"observedAt"`
	// Days is the forecast from today on, at most forecastDays long.
	Days []Day `json:"days"`
}

// Day is the forecast for one day.
type Day struct {
	// Date is the local date, for example "2026-10-09".
	Date        string  `json:"date"`
	WeatherCode int     `json:"weatherCode"`
	MaxC        float64 `json:"maxC"`
	MinC        float64 `json:"minC"`
	// RainChance is the highest chance of precipitation that day, 0 to 100.
	RainChance int `json:"rainChance"`
}

// forecastDays covers a week; the forecast widget shows a part of it.
const forecastDays = 7

// Client fetches reports from Open-Meteo.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

type forecastResponse struct {
	Current struct {
		Time          string   `json:"time"`
		Temperature2m *float64 `json:"temperature_2m"`
		WeatherCode   *int     `json:"weather_code"`
		IsDay         int      `json:"is_day"`
		WindSpeed10m  float64  `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Time                        []string  `json:"time"`
		WeatherCode                 []int     `json:"weather_code"`
		Temperature2mMax            []float64 `json:"temperature_2m_max"`
		Temperature2mMin            []float64 `json:"temperature_2m_min"`
		PrecipitationProbabilityMax []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

var errIncomplete = errors.New("open-meteo response is missing fields")

// Current returns the current weather, today's range and the forecast for
// the coming week for loc, in one request.
func (c Client) Current(ctx context.Context, loc Location) (Report, error) {
	query := url.Values{
		"latitude":      {strconv.FormatFloat(loc.Latitude, 'f', 2, 64)},
		"longitude":     {strconv.FormatFloat(loc.Longitude, 'f', 2, 64)},
		"current":       {"temperature_2m,weather_code,is_day,wind_speed_10m"},
		"daily":         {"weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"},
		"timezone":      {"auto"},
		"forecast_days": {strconv.Itoa(forecastDays)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+query.Encode(), nil)
	if err != nil {
		return Report{}, fmt.Errorf("build open-meteo request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("request open-meteo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Report{}, fmt.Errorf("open-meteo answered %s", resp.Status)
	}

	var body forecastResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return Report{}, fmt.Errorf("decode open-meteo response: %w", err)
	}
	cur := body.Current
	days := body.days()
	if cur.Temperature2m == nil || cur.WeatherCode == nil || len(days) == 0 {
		return Report{}, errIncomplete
	}
	return Report{
		TemperatureC: *cur.Temperature2m,
		WeatherCode:  *cur.WeatherCode,
		IsDay:        cur.IsDay == 1,
		WindKmh:      cur.WindSpeed10m,
		TodayMaxC:    days[0].MaxC,
		TodayMinC:    days[0].MinC,
		ObservedAt:   cur.Time,
		Days:         days,
	}, nil
}

// days pairs the daily arrays up to the shortest of them, so a response
// with a missing value never shifts one day's numbers onto another.
func (r forecastResponse) days() []Day {
	d := r.Daily
	n := min(len(d.Time), len(d.WeatherCode), len(d.Temperature2mMax), len(d.Temperature2mMin), len(d.PrecipitationProbabilityMax), forecastDays)
	days := make([]Day, n)
	for i := range n {
		days[i] = Day{Date: d.Time[i], WeatherCode: d.WeatherCode[i], MaxC: d.Temperature2mMax[i], MinC: d.Temperature2mMin[i], RainChance: d.PrecipitationProbabilityMax[i]}
	}
	return days
}
