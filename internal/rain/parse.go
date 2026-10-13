package rain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	buienradarStepMinutes = 5
	openMeteoStepMinutes  = 15
	// Buienradar's value runs from 0 to 255 and is a logarithmic scale:
	// mm/h = 10^((value - 109) / 32), with 0 meaning dry.
	maxBuienradarValue = 255
	buienradarOffset   = 109
	buienradarDivisor  = 32
	// An answer that covers less than half an hour (Buienradar) or an hour
	// (Open-Meteo) is not a forecast and keeps the last good one on screen.
	minBuienradarPoints = 6
	minOpenMeteoPoints  = 4
	// halfDay decides on which day a Buienradar clock time falls: the one
	// closest to the point before it.
	halfDay = 12 * time.Hour
	// openMeteoTime is how Open-Meteo writes a local time.
	openMeteoTime = "2006-01-02T15:04"
	// maxMMPerHour is far above any real rain (the Dutch record is about
	// 100 mm in an hour); a larger value is a broken answer.
	maxMMPerHour = 500
)

var errTooFewPoints = errors.New("the forecast has too few points")

// parseBuienradar reads Buienradar's raintext: one "VVV|HH:MM" line per five
// minutes, in Dutch time. The lines have no date, so each time is placed on
// the day that puts it closest after the one before, starting from now; a
// forecast that runs past midnight continues on the next day. A line that
// cannot be read is left out, so one broken line does not hide the rest.
func parseBuienradar(body string, now time.Time, loc *time.Location) ([]Point, error) {
	var points []Point
	previous := now.In(loc)
	for line := range strings.Lines(body) {
		value, clock, ok := strings.Cut(strings.TrimSpace(line), "|")
		if !ok {
			continue
		}
		level, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || level < 0 || level > maxBuienradarValue {
			continue
		}
		at, err := time.ParseInLocation("15:04", strings.TrimSpace(clock), loc)
		if err != nil {
			continue
		}
		start := closestDay(at, previous)
		points = append(points, Point{Time: start, MMPerHour: buienradarIntensity(level)})
		previous = start
	}
	if len(points) < minBuienradarPoints {
		return nil, fmt.Errorf("buienradar: %w (%d)", errTooFewPoints, len(points))
	}
	return points, nil
}

// closestDay puts the clock time of at on the day closest to reference.
func closestDay(at, reference time.Time) time.Time {
	t := time.Date(reference.Year(), reference.Month(), reference.Day(), at.Hour(), at.Minute(), 0, 0, reference.Location())
	switch {
	case t.Sub(reference) > halfDay:
		return t.AddDate(0, 0, -1)
	case reference.Sub(t) > halfDay:
		return t.AddDate(0, 0, 1)
	}
	return t
}

func buienradarIntensity(level int) float64 {
	if level == 0 {
		return 0
	}
	return roundIntensity(math.Pow(10, float64(level-buienradarOffset)/buienradarDivisor))
}

// openMeteoAnswer is the part of Open-Meteo's forecast that GDash reads.
// precipitation is the total of the quarter hour that ends at time.
type openMeteoAnswer struct {
	UTCOffsetSeconds *int `json:"utc_offset_seconds"`
	Minutely15       struct {
		Time          []string   `json:"time"`
		Precipitation []*float64 `json:"precipitation"`
	} `json:"minutely_15"`
}

// parseOpenMeteo reads Open-Meteo's quarter-hourly precipitation. Each total
// is turned into the intensity of the quarter hour it covers, which starts
// 15 minutes before its time. Quarters that ended before now are left out,
// as is a quarter without a value.
func parseOpenMeteo(body []byte, now time.Time) ([]Point, error) {
	var answer openMeteoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode open-meteo answer: %w", err)
	}
	if answer.UTCOffsetSeconds == nil {
		return nil, errors.New("open-meteo answer has no utc_offset_seconds")
	}
	zone := time.FixedZone("", *answer.UTCOffsetSeconds)
	m := answer.Minutely15
	step := openMeteoStepMinutes * time.Minute
	var points []Point
	for i, text := range m.Time {
		if i >= len(m.Precipitation) || m.Precipitation[i] == nil {
			continue
		}
		end, err := time.ParseInLocation(openMeteoTime, text, zone)
		mm := *m.Precipitation[i]
		perHour := mm * float64(time.Hour/step)
		if err != nil || mm < 0 || perHour > maxMMPerHour || !end.After(now) {
			continue
		}
		points = append(points, Point{Time: end.Add(-step), MMPerHour: roundIntensity(perHour)})
	}
	if len(points) < minOpenMeteoPoints {
		return nil, fmt.Errorf("open-meteo: %w (%d)", errTooFewPoints, len(points))
	}
	return points, nil
}
