// Package airquality fetches the air quality and the pollen count from
// Open-Meteo, which needs no API key. It answers for the same locations as
// the weather.
package airquality

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

	"github.com/stijnvandepol/gdash/internal/weather"
)

// DefaultBaseURL is the public Open-Meteo air quality endpoint.
const DefaultBaseURL = "https://air-quality-api.open-meteo.com/v1/air-quality"

// Open-Meteo answers are a few hundred bytes; anything far larger is not an
// air quality report and is not read into memory.
const maxResponseBytes = 1 << 20

// currentVariables is what the request asks for: the index and the six
// kinds of pollen Open-Meteo models.
const currentVariables = "european_aqi,alder_pollen,birch_pollen,grass_pollen,mugwort_pollen,olive_pollen,ragweed_pollen"

// Report is the air quality the dashboard shows.
type Report struct {
	// EuropeanAQI is the European Air Quality Index. From 0 to 100 it runs
	// from good to very poor; above 100 it is extremely poor.
	EuropeanAQI int `json:"europeanAqi"`
	// ObservedAt is the local time of the values as Open-Meteo reports it,
	// for example "2026-10-09T13:00".
	ObservedAt string `json:"observedAt"`
	// Pollen holds the kinds Open-Meteo has a value for: alder, birch,
	// grass, mugwort, olive and ragweed, in that order. Pollen is only
	// modelled in Europe and in the season, so the list is often empty. A
	// zero means the kind is modelled but not in the air.
	Pollen []Pollen `json:"pollen"`
}

// Pollen is the concentration of one kind of pollen.
type Pollen struct {
	Type        string  `json:"type"`
	GrainsPerM3 float64 `json:"grainsPerM3"`
}

// Client fetches reports from Open-Meteo.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

type airQualityResponse struct {
	Current struct {
		Time          string   `json:"time"`
		EuropeanAQI   *float64 `json:"european_aqi"`
		AlderPollen   *float64 `json:"alder_pollen"`
		BirchPollen   *float64 `json:"birch_pollen"`
		GrassPollen   *float64 `json:"grass_pollen"`
		MugwortPollen *float64 `json:"mugwort_pollen"`
		OlivePollen   *float64 `json:"olive_pollen"`
		RagweedPollen *float64 `json:"ragweed_pollen"`
	} `json:"current"`
}

var errIncomplete = errors.New("open-meteo response has no air quality index")

// Current returns the air quality and pollen now at loc, in one request.
func (c Client) Current(ctx context.Context, loc weather.Location) (Report, error) {
	query := url.Values{
		"latitude":  {strconv.FormatFloat(loc.Latitude, 'f', 2, 64)},
		"longitude": {strconv.FormatFloat(loc.Longitude, 'f', 2, 64)},
		"current":   {currentVariables},
		"timezone":  {"auto"},
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

	var body airQualityResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return Report{}, fmt.Errorf("decode open-meteo response: %w", err)
	}
	cur := body.Current
	if cur.EuropeanAQI == nil || !validAQI(*cur.EuropeanAQI) {
		return Report{}, errIncomplete
	}
	report := Report{EuropeanAQI: int(math.Round(*cur.EuropeanAQI)), ObservedAt: cur.Time}
	// A null value is the normal case outside Europe and outside the
	// season; that kind is left out rather than shown as a broken number.
	for _, kind := range []struct {
		name   string
		grains *float64
	}{
		{"alder", cur.AlderPollen},
		{"birch", cur.BirchPollen},
		{"grass", cur.GrassPollen},
		{"mugwort", cur.MugwortPollen},
		{"olive", cur.OlivePollen},
		{"ragweed", cur.RagweedPollen},
	} {
		if kind.grains != nil && valid(*kind.grains) {
			report.Pollen = append(report.Pollen, Pollen{Type: kind.name, GrainsPerM3: *kind.grains})
		}
	}
	return report, nil
}

// valid reports whether v can be an index or a concentration; JSON has no
// NaN or infinity, so only a negative number is out.
// The largest values that can be real: the European index stays far below
// a thousand even in a wildfire, and pollen below a million grains per m³.
// Anything larger is a broken answer, and converting a huge float to an
// int would give a negative index.
const (
	maxAQI    = 1000
	maxGrains = 1_000_000
)

func valid(v float64) bool { return v >= 0 && v <= maxGrains }

func validAQI(v float64) bool { return v >= 0 && v <= maxAQI }
