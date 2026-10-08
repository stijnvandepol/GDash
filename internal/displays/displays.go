// Package displays defines a display: a physical screen that shows a
// dashboard, in a browser or through a Cast device.
package displays

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/schedule"
	"github.com/stijnvandepol/daycast/internal/validation"
)

// MaxDisplays bounds how many displays one server manages. A home has a
// handful; the limit keeps every list query bounded.
const MaxDisplays = 50

// MaxSlots bounds the day plan of one display. Morning, afternoon, evening
// and a weekend-style exception fit easily.
const MaxSlots = 6

const maxNameLength = 60

// validID is the slug used in /d/<id>.
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// validCastDeviceID matches the 32 hex characters Cast devices announce.
var validCastDeviceID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Display is one physical screen.
type Display struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// CastDeviceID links the display to a Cast device. Empty for browser
	// and kiosk displays.
	CastDeviceID string `json:"castDeviceId"`
	// DashboardID is the dashboard the display shows; empty means the
	// default dashboard.
	DashboardID string `json:"dashboardId"`
	// Hours are the display's active hours; outside them it is left dark.
	schedule.Hours
	// Schedule shows other dashboards during parts of the day. The first
	// slot that contains the current time wins; outside every slot the
	// display shows DashboardID.
	Schedule  []Slot    `json:"schedule"`
	CreatedAt time.Time `json:"createdAt"`
}

// Slot shows a dashboard during part of the day, such as a morning
// dashboard from 06:30 to 09:00. Times are HH:MM in the server's time zone
// and may wrap past midnight.
type Slot struct {
	From        string `json:"from"`
	Until       string `json:"until"`
	DashboardID string `json:"dashboardId"`
}

// Hours returns the slot's daily window.
func (s Slot) Hours() schedule.Hours { return schedule.Hours{From: s.From, Until: s.Until} }

// Validate checks a display before it is stored. It returns a
// *validation.Error listing every invalid field.
func Validate(d Display) error {
	var errs validation.Collector
	if !validID.MatchString(d.ID) {
		errs.Add("id", "use 1 to 32 lowercase letters, digits or dashes, starting with a letter or digit")
	}
	name := strings.TrimSpace(d.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || name != d.Name {
		errs.Add("name", "use 1 to 60 characters without leading or trailing spaces")
	}
	if d.CastDeviceID != "" && !validCastDeviceID.MatchString(d.CastDeviceID) {
		errs.Add("castDeviceId", "must be the 32-character device ID from Cast discovery")
	}
	if d.DashboardID != "" && !display.ValidDashboardID(d.DashboardID) {
		errs.Add("dashboardId", "must be the id of a dashboard")
	}
	if err := d.Hours.Validate(); err != nil {
		errs.Add("activeHours", err.Error())
	}
	if len(d.Schedule) > MaxSlots {
		errs.Add("schedule", fmt.Sprintf("use at most %d time slots", MaxSlots))
		return errs.Err()
	}
	for i, slot := range d.Schedule {
		if msg := slotProblem(slot); msg != "" {
			errs.Add(fmt.Sprintf("schedule[%d]", i), msg)
		}
	}
	return errs.Err()
}

func slotProblem(s Slot) string {
	// A slot always has a window: "all day" is what DashboardID is for.
	if hours := s.Hours(); hours.Always() || hours.Validate() != nil {
		return "use two different times as HH:MM"
	}
	if !display.ValidDashboardID(s.DashboardID) {
		return "choose a dashboard"
	}
	return ""
}
