package display

import (
	"encoding/json"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// WeatherSource is the data key of the weather at the configured location.
const WeatherSource = "weather.current"

// sunTimeLayout is how Open-Meteo writes sunrise and sunset: the local time
// at the weather location, without a time zone.
const sunTimeLayout = "2006-01-02T15:04"

// IsNight reports whether the sun is down at the weather location at now,
// from the weather entry in data. It is false whenever that cannot be told:
// no location, a weather report that is still loading or cached by an older
// version, or no sun times for today. A dashboard then keeps its own theme.
func IsNight(now time.Time, data map[string]datahub.Entry) bool {
	entry, ok := data[WeatherSource]
	if !ok || len(entry.Value) == 0 {
		return false
	}
	var report weather.Report
	if err := json.Unmarshal(entry.Value, &report); err != nil || report.UTCOffsetSeconds == nil {
		return false
	}
	// The sun times are wall-clock times of the location. Reading both as
	// UTC makes them comparable without a time zone database.
	local := now.UTC().Add(time.Duration(*report.UTCOffsetSeconds) * time.Second)
	date := local.Format(time.DateOnly)
	for _, day := range report.Sun {
		if day.Date == date {
			return sunIsDown(local, day, report.IsDay)
		}
	}
	return false
}

// sunIsDown decides from one day's sunrise and sunset. A time is empty when
// the sun does not rise or set that day, as near the poles: with only a
// sunset the sun was up from midnight until then, with only a sunrise it
// stays up after it, and with neither the report's own day or night is all
// there is. A time that does not parse counts as unknown, which is day.
func sunIsDown(local time.Time, day weather.SunTimes, isDay bool) bool {
	rise, riseOK := parseSunTime(day.Sunrise)
	set, setOK := parseSunTime(day.Sunset)
	switch {
	case (day.Sunrise != "" && !riseOK) || (day.Sunset != "" && !setOK):
		return false
	case !riseOK && !setOK:
		return !isDay
	}
	return (riseOK && local.Before(rise)) || (setOK && !local.Before(set))
}

func parseSunTime(text string) (time.Time, bool) {
	t, err := time.Parse(sunTimeLayout, text)
	return t, err == nil
}

// withNightTheme returns d in its night theme when it has one and it is
// night. The font stays the dashboard's own.
func withNightTheme(d Dashboard, night bool) Dashboard {
	if night && d.NightTheme != "" {
		d.Theme = d.NightTheme
	}
	return d
}

// withNightThemes does the same for every dashboard in list, leaving list
// itself alone.
func withNightThemes(list []Dashboard, night bool) []Dashboard {
	if !night || list == nil {
		return list
	}
	themed := make([]Dashboard, len(list))
	for i, d := range list {
		themed[i] = withNightTheme(d, night)
	}
	return themed
}
