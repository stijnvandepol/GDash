// Package displays defines a display: a physical screen that shows a
// dashboard, in a browser or through a Cast device.
package displays

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/schedule"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// MaxDisplays bounds how many displays one server manages. A home has a
// handful; the limit keeps every list query bounded.
const MaxDisplays = 50

// MaxRotation bounds how many dashboards a display shows in turn with its
// own; more pages than that are rarely read on a kitchen screen.
const MaxRotation = 5

// Rotation intervals, in seconds: long enough to read a page, short enough
// that every page comes by within a few minutes.
const (
	MinRotateSeconds = 10
	MaxRotateSeconds = 600
)

// TextScales are the text sizes a display can have, in percent. A layout
// is made for a TV across a room; a smart display at arm's length or a
// small screen in a kitchen shows it larger.
var TextScales = []int{100, 125, 150, 200}

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
	Schedule []Slot `json:"schedule"`
	// Rotation lists dashboards shown in turn with DashboardID, each for
	// RotateSeconds, while no time slot applies. Empty means no rotation.
	Rotation      []string `json:"rotation"`
	RotateSeconds int      `json:"rotateSeconds"`
	// TextScale is the display's text size in percent, one of TextScales.
	TextScale int       `json:"textScale"`
	CreatedAt time.Time `json:"createdAt"`
}

// Slot shows a dashboard during part of the day, such as a morning
// dashboard from 06:30 to 09:00. Times are HH:MM in the server's time zone
// and may wrap past midnight. Days limits the slot to some days of the
// week, such as a weekend dashboard; none means every day.
type Slot struct {
	From        string   `json:"from"`
	Until       string   `json:"until"`
	Days        []string `json:"days"`
	DashboardID string   `json:"dashboardId"`
}

// NormalizeRotation returns an empty list instead of nil, so the API always
// sends a list.
func NormalizeRotation(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// NormalizeSchedule returns slots with empty lists instead of nil, so the
// API always sends a list for the schedule and for each slot's days.
func NormalizeSchedule(slots []Slot) []Slot {
	out := make([]Slot, len(slots))
	for i, s := range slots {
		if s.Days == nil {
			s.Days = []string{}
		}
		out[i] = s
	}
	return out
}

// Window returns when the slot applies.
func (s Slot) Window() schedule.Window {
	return schedule.Window{Hours: schedule.Hours{From: s.From, Until: s.Until}, Days: s.Days}
}

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
	if msg := rotationProblem(d.Rotation, d.RotateSeconds); msg != "" {
		errs.Add("rotation", msg)
	}
	if !slices.Contains(TextScales, d.TextScale) {
		errs.Add("textScale", "choose 100, 125, 150 or 200 percent")
	}
	return errs.Err()
}

func rotationProblem(ids []string, seconds int) string {
	if len(ids) > MaxRotation {
		return fmt.Sprintf("use at most %d dashboards", MaxRotation)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !display.ValidDashboardID(id) || seen[id] {
			return "choose each dashboard at most once"
		}
		seen[id] = true
	}
	switch {
	case len(ids) == 0 && seconds != 0:
		return "choose the dashboards to show in turn, or no interval"
	case len(ids) > 0 && (seconds < MinRotateSeconds || seconds > MaxRotateSeconds):
		return fmt.Sprintf("show each page for %d to %d seconds", MinRotateSeconds, MaxRotateSeconds)
	}
	return ""
}

func slotProblem(s Slot) string {
	// A slot always has hours: "all day" is what DashboardID is for.
	window := s.Window()
	if window.Always() || window.Hours.Validate() != nil {
		return "use two different times as HH:MM"
	}
	if err := window.Validate(); err != nil {
		return err.Error()
	}
	if !display.ValidDashboardID(s.DashboardID) {
		return "choose a dashboard"
	}
	return ""
}
