// Package displays defines a display: a physical screen that shows a
// dashboard, in a browser or through a Cast device.
package displays

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/validation"
)

// MaxDisplays bounds how many displays one server manages. A home has a
// handful; the limit keeps every list query bounded.
const MaxDisplays = 50

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
	DashboardID string    `json:"dashboardId"`
	CreatedAt   time.Time `json:"createdAt"`
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
	return errs.Err()
}
