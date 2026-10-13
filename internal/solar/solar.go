// Package solar estimates what a household's solar panels produce today
// and tomorrow, per hour. By default it computes the estimate from the
// irradiance forecast on the panels' plane that Open-Meteo gives (no key,
// the service GDash already uses for the weather); Forecast.Solar, which
// models the panels itself, is the alternative. An estimate follows the
// cloud forecast and is no measurement.
package solar

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// Where the estimate comes from.
const (
	SourceOpenMeteo     = "open-meteo"
	SourceForecastSolar = "forecast.solar"
)

// Sources are the services an admin can choose, the default first.
var Sources = []string{SourceOpenMeteo, SourceForecastSolar}

// Bounds of a setup. Two arrays cover a roof with two sides; Forecast.Solar
// answers one array per request, and its free use allows 12 an hour.
const (
	MaxArrays  = 2
	MinTilt    = 0
	MaxTilt    = 90
	MinAzimuth = -180
	MaxAzimuth = 180
	// MinPeakKW and MaxPeakKW bound one array: a single panel to a large
	// farm roof.
	MinPeakKW = 0.1
	MaxPeakKW = 100.0
	// DefaultPerformanceRatio is the share of the irradiance model that a
	// typical system turns into AC power, after inverter, cable, heat and
	// dirt losses.
	DefaultPerformanceRatio = 0.85
	MinPerformanceRatio     = 0.5
	MaxPerformanceRatio     = 1.0
)

// Array is a group of panels that face the same way.
type Array struct {
	// Tilt is the angle with the ground in degrees: 0 flat, 90 upright.
	Tilt int `json:"tilt"`
	// Azimuth is where the panels face in degrees: 0 south, -90 east,
	// 90 west, ±180 north (the convention of Open-Meteo and
	// Forecast.Solar both).
	Azimuth int `json:"azimuth"`
	// PeakKW is the panels' total peak power in kWp.
	PeakKW float64 `json:"kwp"`
}

// Setup is everything an estimate needs.
type Setup struct {
	Location weather.Location
	Arrays   []Array
	// PerformanceRatio scales the Open-Meteo estimate; Forecast.Solar
	// applies its own.
	PerformanceRatio float64
	Source           string
}

// Hour is the energy expected in the hour from Start.
type Hour struct {
	// Start is written in the location's UTC offset, so a screen can read
	// the local hour from the text.
	Start time.Time `json:"start"`
	KWh   float64   `json:"kwh"`
}

// Day is the expected production of one local day.
type Day struct {
	Date     string  `json:"date"`
	TotalKWh float64 `json:"totalKWh"`
	// Hours run from the first to the last hour with production.
	Hours []Hour `json:"hours"`
}

// Report is the estimate for today and tomorrow.
type Report struct {
	Source string `json:"source"`
	// UTCOffsetSeconds is how far the location's local time is ahead of
	// UTC, so a screen elsewhere can tell which day is today there.
	UTCOffsetSeconds int   `json:"utcOffsetSeconds"`
	Days             []Day `json:"days"`
}

// maxResponseBytes bounds an answer: two days of hourly numbers are a few
// kilobytes.
const maxResponseBytes = 1 << 20

const dateLayout = "2006-01-02"

// CheckPanels validates the panels of a setup. It returns a
// *validation.Error naming every invalid field.
func CheckPanels(arrays []Array, ratio float64, source string) error {
	var errs validation.Collector
	if len(arrays) == 0 || len(arrays) > MaxArrays {
		errs.Add("arrays", fmt.Sprintf("add 1 or %d groups of panels", MaxArrays))
	}
	for i, a := range arrays {
		field := fmt.Sprintf("arrays[%d]", i)
		if a.Tilt < MinTilt || a.Tilt > MaxTilt {
			errs.Add(field+".tilt", "use a whole number of degrees from 0 (flat) to 90 (upright)")
		}
		if a.Azimuth < MinAzimuth || a.Azimuth > MaxAzimuth {
			errs.Add(field+".azimuth", "use a whole number of degrees from -180 to 180: 0 is south, -90 east, 90 west")
		}
		if math.IsNaN(a.PeakKW) || a.PeakKW < MinPeakKW || a.PeakKW > MaxPeakKW {
			errs.Add(field+".kwp", "use the peak power in kWp, from 0.1 to 100")
		}
	}
	if math.IsNaN(ratio) || ratio < MinPerformanceRatio || ratio > MaxPerformanceRatio {
		errs.Add("performanceRatio", "use a number from 0.5 to 1.0, such as 0.85")
	}
	if !slices.Contains(Sources, source) {
		errs.Add("source", "choose open-meteo or forecast.solar")
	}
	return errs.Err()
}

// Client fetches an estimate from the source that a setup names.
type Client struct {
	OpenMeteo     OpenMeteo
	ForecastSolar ForecastSolar
}

// Forecast returns the estimate for setup.
func (c Client) Forecast(ctx context.Context, setup Setup) (Report, error) {
	switch setup.Source {
	case SourceOpenMeteo:
		return c.OpenMeteo.Forecast(ctx, setup)
	case SourceForecastSolar:
		return c.ForecastSolar.Forecast(ctx, setup)
	}
	return Report{}, fmt.Errorf("unknown solar forecast source %q", setup.Source)
}

// EnergyKWh is the energy that panels of peakKW produce in an hour with a
// mean irradiance of gti W/m² on their plane: peak power is rated at
// 1000 W/m², and ratio covers the losses on the way to the meter. Night
// and missing values give nothing.
func EnergyKWh(gti, peakKW, ratio float64) float64 {
	if math.IsNaN(gti) || gti <= 0 {
		return 0
	}
	return gti / 1000 * peakKW * ratio
}

// hourly is the energy per hour start of one or more arrays, summed.
type hourly map[time.Time]float64

// add sums energy into the hour that starts at start, keyed in UTC so the
// same instant written in two offsets is one hour.
func (h hourly) add(start time.Time, kwh float64) {
	h[start.UTC()] += kwh
}

// days splits the hours into local days, in the fixed offset of the
// location, from the day of from on. Each day's hours are trimmed to the
// first and last with production.
func (h hourly) days(offset *time.Location, from string) []Day {
	starts := make([]time.Time, 0, len(h))
	for start := range h {
		starts = append(starts, start)
	}
	slices.SortFunc(starts, time.Time.Compare)
	days := []Day{}
	for _, start := range starts {
		local := start.In(offset)
		date := local.Format(dateLayout)
		if date < from {
			continue
		}
		if len(days) == 0 || days[len(days)-1].Date != date {
			days = append(days, Day{Date: date, Hours: []Hour{}})
		}
		day := &days[len(days)-1]
		day.Hours = append(day.Hours, Hour{Start: local, KWh: round(h[start], 1000)})
		day.TotalKWh += h[start]
	}
	for i := range days {
		days[i].TotalKWh = round(days[i].TotalKWh, 100)
		days[i].Hours = trim(days[i].Hours)
	}
	return days
}

// trim drops the hours without production at both ends of a day.
func trim(hours []Hour) []Hour {
	first := slices.IndexFunc(hours, func(h Hour) bool { return h.KWh > 0 })
	if first < 0 {
		return []Hour{}
	}
	last := len(hours) - 1
	for hours[last].KWh <= 0 {
		last--
	}
	return hours[first : last+1]
}

func round(v, factor float64) float64 { return math.Round(v*factor) / factor }

// hourStart returns the start of the local hour that holds t, in t's own
// offset (Truncate would round in UTC, which is wrong for an offset such
// as +05:30).
func hourStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
