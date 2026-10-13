// Package knmi reads the latest 10-minute observations of the KNMI weather
// station nearest to a place, through the KNMI Data Platform's EDR API. The
// API needs a personal key; the data is open (CC BY 4.0, KNMI).
//
// The key is a secret: it only ever goes into the Authorization header, and
// no error or log line from this package contains it.
package knmi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/geo"
)

// DefaultBaseURL is the collection of 10-minute in-situ observations.
const DefaultBaseURL = "https://api.dataplatform.knmi.nl/edr/v1/collections/10-minute-in-situ-meteorological-observations"

const (
	// maxAnswerBytes bounds what is read: the station list is about 20 KB
	// and an hour of three values a few KB.
	maxAnswerBytes = 4 << 20
	// stationsValidity is how long the station list is used before it is
	// asked again; stations open and close a few times a year.
	stationsValidity = 24 * time.Hour
	// window is how far back an observation is looked for. A station
	// reports every 10 minutes, with some delay.
	window = time.Hour
	// MaxStationKm is the furthest a station may be to count as "the
	// weather here".
	MaxStationKm = 50
	// tries is how many of the nearest stations are asked, for a station
	// that does not measure temperature or is down.
	tries = 3
	// metresPerSecondToKmh turns the wind speed into what the weather
	// widget shows.
	metresPerSecondToKmh = 3.6
	oneDecimal           = 10
)

// The parameters asked for: air temperature (°C), relative humidity (%)
// and mean wind speed (m/s), as the collection names them.
const (
	paramTemperature = "ta"
	paramHumidity    = "rh"
	paramWind        = "ff"
)

var (
	// ErrUnauthorized means the KNMI did not accept the key.
	ErrUnauthorized = errors.New("the KNMI did not accept the API key")
	// ErrNoStation means no station near the place reported in the last
	// hour.
	ErrNoStation = errors.New("no KNMI station nearby has reported in the last hour")
)

// Station is a weather station.
type Station struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
}

// Observation is what a station measured, as the weather widget shows it.
// A value is nil when the station did not report it.
type Observation struct {
	// Station is the station's name, such as "Volkel".
	Station    string    `json:"station"`
	DistanceKm float64   `json:"distanceKm"`
	ObservedAt time.Time `json:"observedAt"`
	// TemperatureC is the air temperature at 1.5 m.
	TemperatureC *float64 `json:"temperatureC"`
	// Humidity is the relative humidity in percent.
	Humidity *float64 `json:"humidity"`
	// WindKmh is the 10-minute mean wind speed.
	WindKmh *float64 `json:"windKmh"`
}

// Client asks the EDR API. It keeps the station list for a day.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP      *http.Client
	UserAgent string
	Now       func() time.Time

	mu         sync.Mutex
	stations   []Station
	stationsAt time.Time
	stationKey string
}

// Check tries a key by asking for the station list: nil, ErrUnauthorized,
// or another error when the KNMI cannot be reached.
func (c *Client) Check(ctx context.Context, key string) error {
	_, err := c.fetchStations(ctx, key)
	return err
}

// Latest returns the newest observation of the station nearest to the
// place that reported a temperature in the last hour.
func (c *Client) Latest(ctx context.Context, key string, latitude, longitude float64) (Observation, error) {
	stations, err := c.stationList(ctx, key)
	if err != nil {
		return Observation{}, err
	}
	var errs []error
	for _, s := range Nearest(stations, latitude, longitude, tries) {
		obs, err := c.observe(ctx, key, s)
		if errors.Is(err, ErrUnauthorized) {
			return Observation{}, err
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("station %s: %w", s.ID, err))
			continue
		}
		obs.DistanceKm = math.Round(geo.DistanceKm(latitude, longitude, s.Latitude, s.Longitude)*oneDecimal) / oneDecimal
		return obs, nil
	}
	return Observation{}, errors.Join(append([]error{ErrNoStation}, errs...)...)
}

// Nearest returns at most n stations within MaxStationKm of the place,
// nearest first.
func Nearest(stations []Station, latitude, longitude float64, n int) []Station {
	type near struct {
		station Station
		km      float64
	}
	var found []near
	for _, s := range stations {
		if km := geo.DistanceKm(latitude, longitude, s.Latitude, s.Longitude); km <= MaxStationKm {
			found = append(found, near{s, km})
		}
	}
	slices.SortFunc(found, func(a, b near) int {
		return cmp.Or(cmp.Compare(a.km, b.km), strings.Compare(a.station.ID, b.station.ID))
	})
	out := make([]Station, 0, min(n, len(found)))
	for _, f := range found[:min(n, len(found))] {
		out = append(out, f.station)
	}
	return out
}

// stationList returns the kept station list, or fetches it when it is
// older than a day or was fetched with another key.
func (c *Client) stationList(ctx context.Context, key string) ([]Station, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stations != nil && c.stationKey == key && c.Now().Sub(c.stationsAt) < stationsValidity {
		return c.stations, nil
	}
	stations, err := c.fetchStations(ctx, key)
	if err != nil {
		return nil, err
	}
	c.stations, c.stationsAt, c.stationKey = stations, c.Now(), key
	return stations, nil
}

func (c *Client) fetchStations(ctx context.Context, key string) ([]Station, error) {
	body, err := c.get(ctx, key, c.BaseURL+"/locations")
	if err != nil {
		return nil, err
	}
	return decodeStations(body)
}

// observe asks one station for the last hour and returns its newest values.
func (c *Client) observe(ctx context.Context, key string, s Station) (Observation, error) {
	now := c.Now().UTC().Truncate(time.Second)
	query := url.Values{
		"datetime":       {now.Add(-window).Format(time.RFC3339) + "/" + now.Format(time.RFC3339)},
		"parameter-name": {strings.Join([]string{paramTemperature, paramHumidity, paramWind}, ",")},
	}
	body, err := c.get(ctx, key, c.BaseURL+"/locations/"+url.PathEscape(s.ID)+"?"+query.Encode())
	if err != nil {
		return Observation{}, err
	}
	obs, err := decodeObservation(body)
	if err != nil {
		return Observation{}, err
	}
	obs.Station = s.Name
	return obs, nil
}

// get asks the API with the key in the Authorization header, as the KNMI
// documents: the bare key, without a scheme.
func (c *Client) get(ctx context.Context, key, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build knmi request: %w", err)
	}
	req.Header.Set("Authorization", key)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request knmi: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("knmi answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read knmi answer: %w", err)
	}
	if len(body) > maxAnswerBytes {
		return nil, fmt.Errorf("knmi answer is larger than %d bytes", maxAnswerBytes)
	}
	return body, nil
}
