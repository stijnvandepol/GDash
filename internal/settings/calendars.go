package settings

import (
	"context"
	"slices"
	"strings"

	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// CalendarColors are the colours a calendar can have on the TV; the screen
// and the web interface map the names to the same shades.
var CalendarColors = []string{"blue", "green", "orange", "purple", "red", "teal", "pink", "yellow"}

// calendarColor returns the chosen colour, or one from the calendar's place
// in the list, so calendars differ without anyone choosing.
func calendarColor(chosen string, index int) string {
	if slices.Contains(CalendarColors, chosen) {
		return chosen
	}
	return CalendarColors[index%len(CalendarColors)]
}

// Calendar is a calendar as the web interface sees it: never its secret
// address, only the host it comes from.
type Calendar struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	// Color marks the calendar's events on the TV.
	Color string `json:"color"`
	// Unreadable is true when the address cannot be decrypted, for example
	// after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

// AddCalendar adds an ICS calendar; its address is stored encrypted. It
// returns a *validation.Error for invalid input and store.ErrConflict when
// MaxCalendars is reached.
func (s *Service) AddCalendar(ctx context.Context, name, address string) (Calendar, error) {
	u, err := checkSource(name, address, true)
	if err != nil {
		return Calendar{}, err
	}
	sealed, err := s.Box.Seal(u.String())
	if err != nil {
		return Calendar{}, err
	}
	id, err := s.Store.AddSource(ctx, store.Source{
		Kind: store.SourceCalendar, Name: name, Address: sealed, Host: u.Hostname(), CreatedAt: s.Now().UTC(),
	}, MaxCalendars)
	if err != nil {
		return Calendar{}, err
	}
	return Calendar{ID: id, Name: name, Host: u.Hostname()}, nil
}

// SetCalendarColor chooses the colour of a calendar. It returns a
// *validation.Error for a colour that is not in CalendarColors and
// store.ErrNotFound for an unknown calendar.
func (s *Service) SetCalendarColor(ctx context.Context, id int64, color string) error {
	if !slices.Contains(CalendarColors, color) {
		var errs validation.Collector
		errs.Add("color", "choose one of: "+strings.Join(CalendarColors, ", "))
		return errs.Err()
	}
	return s.Store.SetSourceColor(ctx, store.SourceCalendar, id, color)
}

// DeleteCalendar removes a calendar, or returns store.ErrNotFound.
func (s *Service) DeleteCalendar(ctx context.Context, id int64) error {
	return s.Store.DeleteSource(ctx, store.SourceCalendar, id)
}
