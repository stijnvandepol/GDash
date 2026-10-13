package knmi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// decodeStations reads the GeoJSON station list. A station without an id
// or a point is left out.
func decodeStations(body []byte) ([]Station, error) {
	var answer struct {
		Features *[]struct {
			ID       string `json:"id"`
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Name string `json:"name"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode knmi station list: %w", err)
	}
	if answer.Features == nil {
		return nil, errors.New("knmi station list has no features")
	}
	var stations []Station
	for _, f := range *answer.Features {
		coords := f.Geometry.Coordinates
		if f.ID == "" || len(coords) < 2 || math.Abs(coords[1]) > 90 || math.Abs(coords[0]) > 180 {
			continue
		}
		stations = append(stations, Station{ID: f.ID, Name: stationName(f.Properties.Name, f.ID), Latitude: coords[1], Longitude: coords[0]})
	}
	return stations, nil
}

// stationName writes a name the KNMI sends in capitals ("VALKENBURG VK")
// with capitals at the start of words only; others stay as sent.
func stationName(name, id string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return id
	}
	if strings.ToUpper(name) != name {
		return name
	}
	words := strings.Fields(strings.ToLower(name))
	for i, w := range words {
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// coverage is the part of a CoverageJSON answer that GDash reads.
type coverage struct {
	Domain struct {
		Axes struct {
			T struct {
				Values []string `json:"values"`
			} `json:"t"`
		} `json:"axes"`
	} `json:"domain"`
	Ranges map[string]struct {
		Values []*float64 `json:"values"`
	} `json:"ranges"`
}

// decodeObservation reads a CoverageJSON answer for one station: either a
// CoverageCollection or a single Coverage. Each value is the newest that
// is not null; the observation time is that of the temperature.
func decodeObservation(body []byte) (Observation, error) {
	var answer struct {
		Type      string     `json:"type"`
		Coverages []coverage `json:"coverages"`
		coverage
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return Observation{}, fmt.Errorf("decode knmi observation: %w", err)
	}
	cov := answer.coverage
	if answer.Type == "CoverageCollection" {
		if len(answer.Coverages) == 0 {
			return Observation{}, errors.New("knmi answer holds no coverage")
		}
		cov = answer.Coverages[0]
	}
	times := cov.Domain.Axes.T.Values
	temperature, at, ok := newest(cov.Ranges[paramTemperature].Values, times)
	if !ok {
		return Observation{}, errors.New("knmi station reported no temperature in the last hour")
	}
	obs := Observation{ObservedAt: at, TemperatureC: &temperature}
	if humidity, _, ok := newest(cov.Ranges[paramHumidity].Values, times); ok && humidity >= 0 && humidity <= 100 {
		obs.Humidity = &humidity
	}
	if wind, _, ok := newest(cov.Ranges[paramWind].Values, times); ok && wind >= 0 {
		kmh := math.Round(wind*metresPerSecondToKmh*oneDecimal) / oneDecimal
		obs.WindKmh = &kmh
	}
	return obs, nil
}

// newest returns the last value that is a number, with its time. Values
// and times that do not line up are not trusted.
func newest(values []*float64, times []string) (float64, time.Time, bool) {
	if len(values) != len(times) {
		return 0, time.Time{}, false
	}
	for i := len(values) - 1; i >= 0; i-- {
		v := values[i]
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			continue
		}
		at, err := time.Parse(time.RFC3339, times[i])
		if err != nil {
			continue
		}
		return *v, at, true
	}
	return 0, time.Time{}, false
}
