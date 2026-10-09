package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DefaultGeocodingURL is the public Open-Meteo place search, which uses
// GeoNames data.
const DefaultGeocodingURL = "https://geocoding-api.open-meteo.com/v1/search"

const (
	maxPlaces = 8
	// Open-Meteo returns nothing for one character; long input is not a
	// place name.
	minQueryRunes = 2
	maxQueryRunes = 100
)

// Place is a search result: a name to show and its location.
type Place struct {
	Name string `json:"name"`
	// Region is the province or state and the country, for telling places
	// with the same name apart.
	Region    string  `json:"region"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Geocoder searches places by name.
type Geocoder struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

// ValidQuery reports whether query can be searched.
func ValidQuery(query string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(query))
	return n >= minQueryRunes && n <= maxQueryRunes
}

// Search returns up to eight places matching query, in English.
func (g Geocoder) Search(ctx context.Context, query string) ([]Place, error) {
	params := url.Values{
		"name":     {strings.TrimSpace(query)},
		"count":    {strconv.Itoa(maxPlaces)},
		"language": {"en"},
		"format":   {"json"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.BaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build place search: %w", err)
	}
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search places: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("place search answered %s", resp.Status)
	}
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode place search: %w", err)
	}
	places := []Place{}
	for _, r := range body.Results {
		if len(places) == maxPlaces {
			break
		}
		if _, err := NewLocation(r.Latitude, r.Longitude); err != nil || r.Name == "" {
			continue
		}
		region := strings.Trim(r.Admin1+", "+r.Country, ", ")
		places = append(places, Place{Name: r.Name, Region: region, Latitude: r.Latitude, Longitude: r.Longitude})
	}
	return places, nil
}
