package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/settings"
)

const (
	// calendarTestDays is how far ahead the test of an appointments
	// calendar looks, so a quiet calendar still shows something.
	calendarTestDays = 30
	// calendarTestEvents is how many of the events found a test returns.
	calendarTestEvents = 5
)

// Error codes of a calendar test that failed, so the web interface can
// explain the common mistakes for the service the address is from.
const (
	codeCalendarNotFound = "calendar_not_found"
	codeCalendarRefused  = "calendar_refused"
	codeNotACalendar     = "not_a_calendar"
	codeCalendarTooLarge = "calendar_too_large"
	codeFetchFailed      = "fetch_failed"
)

// postCalendarTest handles POST /api/calendar-tests {url}: 200 {"name",
// "count", "events": [{title, start, end, allDay, calendar}]} with the
// calendar's own name, the number of events in the next calendarTestDays
// and the first calendarTestEvents of them, read now and not stored, so the
// setup shows that an address works before it is added. 422 for an address
// that is not one; 502 with calendar_not_found, calendar_refused,
// not_a_calendar, calendar_too_large or fetch_failed.
func (s *Server) postCalendarTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	address, err := settings.CalendarAddress(body.URL)
	if err != nil {
		s.writeSettingsError(w, "test calendar", err)
		return
	}
	events, err := s.Calendars.Read(r.Context(), calendar.Source{URL: address}, calendarTestDays)
	if err != nil {
		// The error never holds the address; see calendar.withoutURL.
		s.Logger.Warn("calendar test failed", "error", err)
		code, message := calendarFailure(err)
		writeError(w, http.StatusBadGateway, code, message)
		return
	}
	name := ""
	if len(events) > 0 {
		name = events[0].Calendar
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":   name,
		"count":  len(events),
		"events": events[:min(len(events), calendarTestEvents)],
	})
}

// calendarFailure says what went wrong reading a calendar, for an admin
// who just pasted its address: the error code and a message.
func calendarFailure(err error) (code, message string) {
	const prefix = "The calendar could not be read: "
	var status *calendar.StatusError
	switch {
	case errors.As(err, &status) && (status.Code == http.StatusNotFound || status.Code == http.StatusGone):
		return codeCalendarNotFound, prefix + status.Error() + ". There is no calendar at this address, or it is not shared."
	case errors.As(err, &status) && (status.Code == http.StatusUnauthorized || status.Code == http.StatusForbidden):
		return codeCalendarRefused, prefix + status.Error() + ". The address needs a login; use the secret or published iCal address instead."
	case errors.Is(err, calendar.ErrNotCalendar):
		return codeNotACalendar, "This address shows a web page, not an iCal calendar. Copy the iCal (ICS) address instead."
	case errors.Is(err, calendar.ErrTooLarge):
		return codeCalendarTooLarge, fmt.Sprintf("The calendar is larger than GDash reads (%d MB).", calendar.MaxBytes>>20)
	}
	return codeFetchFailed, prefix + err.Error() + ". Check that you copied the whole link."
}
