package display

import (
	"time"

	"github.com/stijnvandepol/gdash/internal/schedule"
)

// What a screen does to look after its own panel. With OLED care the screen
// drifts the dashboard further and slowly, dims what never changes and goes
// black for a second every hour; web/src/screenCare.ts does the drawing.
const (
	ScreenCareOff  = "off"
	ScreenCareOLED = "oled"
)

// ScreenCares are the screen care settings a display can have.
var ScreenCares = []string{ScreenCareOff, ScreenCareOLED}

// When a display draws its dashboard dimmed, at 60 % brightness.
const (
	DimOff = "off"
	// DimAfterSunset follows sunset and sunrise at the weather location,
	// the same moments as a dashboard's night theme.
	DimAfterSunset = "sunset"
	// DimDuringHours dims between two times of the day.
	DimDuringHours = "hours"
)

// DimModes are the ways a display can dim at night.
var DimModes = []string{DimOff, DimAfterSunset, DimDuringHours}

// Dimming is when a display draws its dashboard dimmed.
type Dimming struct {
	Mode string
	// Hours apply to DimDuringHours, in the server's time zone.
	Hours schedule.Hours
}

// At reports whether the display is dimmed at now. Night is whether the sun
// is down at the weather location; without a location it is never night.
func (d Dimming) At(now time.Time, night bool) bool {
	switch d.Mode {
	case DimAfterSunset:
		return night
	case DimDuringHours:
		// Contains counts invalid hours as always; dimming never does.
		return d.Hours.Validate() == nil && !d.Hours.Always() && d.Hours.Contains(now)
	}
	return false
}
