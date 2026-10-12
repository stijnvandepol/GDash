package fuel

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// A price below this (euro per litre) is a station that does not sell
	// the fuel or has not filled in a price; the service sends 0 for those.
	minPrice = 0.10
	// A price above this is a broken answer, not a market.
	maxPrice = 10.0
	// maxNameRunes keeps one odd title from filling a tile.
	maxNameRunes = 60
	// priceScale and distanceScale round to three and one decimals.
	priceScale    = 1000
	distanceScale = 10
)

// poi is the part of an ANWB point of interest that GDash uses.
type poi struct {
	Title       string `json:"title"`
	Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"coordinates"`
	Address struct {
		City string `json:"city"`
	} `json:"address"`
	Prices []struct {
		FuelType string  `json:"fuelType"`
		Value    float64 `json:"value"`
	} `json:"prices"`
}

// decode reads an ANWB answer. An answer without a station list is an
// error, so a changed or broken service keeps the last good prices on
// screen instead of showing "no stations". A list that is empty is real.
func decode(body []byte) ([]poi, error) {
	var answer struct {
		Value []poi `json:"value"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode anwb response: %w", err)
	}
	if answer.Value == nil {
		return nil, errors.New("anwb response has no station list")
	}
	return answer.Value, nil
}

// pick turns stations into the cheapest ones for a fuel within the circle:
// cheapest first, nearest first among equal prices, at most MaxStations.
func pick(stations []poi, area Area, fuel string) []Station {
	found := []Station{}
	for _, s := range stations {
		price, hasPrice := s.price(fuel)
		name := s.label()
		dist := distanceKm(area.Latitude, area.Longitude, s.Coordinates.Latitude, s.Coordinates.Longitude)
		if !hasPrice || name == "" || dist > float64(area.RadiusKm) {
			continue
		}
		found = append(found, Station{Name: name, Price: price, DistanceKm: math.Round(dist*distanceScale) / distanceScale})
	}
	slices.SortStableFunc(found, func(a, b Station) int {
		return cmp.Or(cmp.Compare(a.Price, b.Price), cmp.Compare(a.DistanceKm, b.DistanceKm), strings.Compare(a.Name, b.Name))
	})
	return found[:min(len(found), MaxStations)]
}

// price is the lowest sane price the station lists for the fuel.
func (s poi) price(fuel string) (float64, bool) {
	best, found := 0.0, false
	for _, p := range s.Prices {
		if p.FuelType != fuel || math.IsNaN(p.Value) || p.Value < minPrice || p.Value > maxPrice {
			continue
		}
		if !found || p.Value < best {
			best, found = p.Value, true
		}
	}
	return math.Round(best*priceScale) / priceScale, found
}

// label is the station's title with its town, unless the title has the
// town already ("TinQ Oss").
func (s poi) label() string {
	title, city := strings.TrimSpace(s.Title), strings.TrimSpace(s.Address.City)
	if title != "" && city != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(city)) {
		title += ", " + city
	}
	if utf8.RuneCountInString(title) > maxNameRunes {
		title = string([]rune(title)[:maxNameRunes])
	}
	return title
}
