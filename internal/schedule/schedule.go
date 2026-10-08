// Package schedule decides whether a display is in its active hours. Outside
// them a TV is left alone: no cast, no refresh, a dark page. That saves
// power, lets the TV sleep and avoids burn-in at night.
package schedule

import (
	"errors"
	"fmt"
	"time"
)

// Hours is a daily window in the server's time zone, as "HH:MM". Both empty
// means always active. From after Until wraps past midnight, so 22:00 to
// 06:00 is a night window.
type Hours struct {
	From  string `json:"activeFrom"`
	Until string `json:"activeUntil"`
}

// Always reports whether the window covers the whole day.
func (h Hours) Always() bool { return h.From == "" && h.Until == "" }

// Validate checks the window. It returns a plain message for the user.
func (h Hours) Validate() error {
	if h.Always() {
		return nil
	}
	from, errFrom := minutes(h.From)
	until, errUntil := minutes(h.Until)
	if errFrom != nil || errUntil != nil {
		return errors.New("use two times as HH:MM, or leave both empty to stay on all day")
	}
	if from == until {
		return errors.New("the start and end time must differ")
	}
	return nil
}

// Contains reports whether t falls in the window. Invalid hours count as
// always active, so a bad value never switches a screen off; Validate keeps
// them out of the database.
func (h Hours) Contains(t time.Time) bool {
	from, errFrom := minutes(h.From)
	until, errUntil := minutes(h.Until)
	if errFrom != nil || errUntil != nil || from == until {
		return true
	}
	now := t.Hour()*60 + t.Minute()
	if from < until {
		return now >= from && now < until
	}
	return now >= from || now < until
}

func minutes(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil || len(hhmm) != 5 {
		return 0, fmt.Errorf("invalid time %q", hhmm)
	}
	return t.Hour()*60 + t.Minute(), nil
}
