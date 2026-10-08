// Package displays defines a display: a physical screen that shows a
// dashboard, in a browser or through a Cast device.
package displays

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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
	CastDeviceID string    `json:"castDeviceId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FieldError describes one invalid field, for the API's error details.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError lists every invalid field at once, so a form can show all
// problems together.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "invalid display: " + strings.Join(parts, "; ")
}

// Validate checks a display before it is stored.
func Validate(d Display) error {
	var fields []FieldError
	if !validID.MatchString(d.ID) {
		fields = append(fields, FieldError{"id", "use 1 to 32 lowercase letters, digits or dashes, starting with a letter or digit"})
	}
	name := strings.TrimSpace(d.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || name != d.Name {
		fields = append(fields, FieldError{"name", "use 1 to 60 characters without leading or trailing spaces"})
	}
	if d.CastDeviceID != "" && !validCastDeviceID.MatchString(d.CastDeviceID) {
		fields = append(fields, FieldError{"castDeviceId", "must be the 32-character device ID from Cast discovery"})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// IsValidationError reports whether err is a ValidationError.
func IsValidationError(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}
