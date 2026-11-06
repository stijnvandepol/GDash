// Package schedule decides whether a display is in its active hours. Outside
// them a TV is left alone: no cast, no refresh, a dark page. That saves
// power, lets the TV sleep and avoids burn-in at night.
package schedule

import (
	"errors"
	"fmt"
	"slices"
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

// Weekdays are the day names a Window uses, Monday first.
var Weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// Window is daily hours on some days of the week. No days means every day.
type Window struct {
	Hours
	Days []string
}

// Validate checks the days; the hours are checked by their own rules.
func (w Window) Validate() error {
	seen := map[string]bool{}
	for _, d := range w.Days {
		if !slices.Contains(Weekdays, d) || seen[d] {
			return errors.New("use each of mon, tue, wed, thu, fri, sat and sun at most once")
		}
		seen[d] = true
	}
	return nil
}

// Contains reports whether t falls in the window. A window that wraps past
// midnight belongs to the day it starts: Friday 22:00 to 01:00 includes
// Saturday 00:30.
func (w Window) Contains(t time.Time) bool {
	if !w.Hours.Contains(t) {
		return false
	}
	if len(w.Days) == 0 {
		return true
	}
	started := t
	if w.startedYesterday(t) {
		started = t.AddDate(0, 0, -1)
	}
	return slices.Contains(w.Days, weekdayName(started.Weekday()))
}

// startedYesterday reports whether t lies in the part of a wrapping window
// after midnight. It assumes Hours.Contains(t).
func (w Window) startedYesterday(t time.Time) bool {
	from, errFrom := minutes(w.From)
	until, errUntil := minutes(w.Until)
	if errFrom != nil || errUntil != nil || from <= until {
		return false
	}
	return t.Hour()*60+t.Minute() < until
}

func weekdayName(d time.Weekday) string {
	// time.Weekday counts from Sunday; Weekdays starts on Monday.
	return Weekdays[(int(d)+6)%7]
}

// MaxBlocks bounds the blocks of a Plan. A week with a few blocks a day fits
// easily; the limit keeps every list and every check bounded.
const MaxBlocks = 21

// Block is one stretch of time a screen is on: from and until as HH:MM on
// some days of the week. No days means every day. Until before From wraps
// past midnight, and then belongs to the day it starts.
type Block struct {
	Days  []string `json:"days"`
	From  string   `json:"from"`
	Until string   `json:"until"`
}

// Window returns the block as a window.
func (b Block) Window() Window {
	return Window{Hours: Hours{From: b.From, Until: b.Until}, Days: b.Days}
}

// Validate checks the block. A block always has two different times: "all
// day" is what a day without blocks is for, in a plan without blocks.
func (b Block) Validate() error {
	w := b.Window()
	if w.Always() || w.Hours.Validate() != nil {
		return errors.New("use two different times as HH:MM")
	}
	return w.Validate()
}

// Plan is when a screen is on: the blocks together. Blocks may overlap and
// several may fall on one day. No blocks means always on.
type Plan []Block

// Contains reports whether the screen is on at t. A block with invalid times
// counts as always on, as Hours does, so a bad value never switches a screen
// off; Validate keeps them out of the database.
func (p Plan) Contains(t time.Time) bool {
	if len(p) == 0 {
		return true
	}
	for _, b := range p {
		if b.Window().Contains(t) {
			return true
		}
	}
	return false
}

// Validate checks the plan and returns the index of the first invalid block
// with its message, or -1 for a valid plan. Too many blocks are index -2.
func (p Plan) Validate() (int, error) {
	if len(p) > MaxBlocks {
		return -2, fmt.Errorf("use at most %d blocks", MaxBlocks)
	}
	for i, b := range p {
		if err := b.Validate(); err != nil {
			return i, err
		}
	}
	return -1, nil
}

// Normalize returns a plan with empty lists instead of nil, so the API
// always sends a list for the plan and for each block's days.
func (p Plan) Normalize() Plan {
	out := make(Plan, len(p))
	for i, b := range p {
		if b.Days == nil {
			b.Days = []string{}
		}
		out[i] = b
	}
	return out
}

// Plan returns hours as a block on every day, or no blocks for hours that
// are always on: how a single daily window is a plan.
func (h Hours) Plan() Plan {
	if h.Always() {
		return Plan{}
	}
	return Plan{{Days: []string{}, From: h.From, Until: h.Until}}
}
