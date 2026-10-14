package api

import (
	"context"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/waste"
)

// wasteTestDays is how far ahead a test looks: long enough to see every
// kind of a calendar that collects some kinds once a month.
const wasteTestDays = 56

// CalendarReader reads one calendar once, for the Test buttons of the
// calendar and waste collection setups.
type CalendarReader interface {
	Read(ctx context.Context, src calendar.Source, days int) ([]calendar.Event, error)
}

// postWasteCalendarTest handles POST /api/waste-calendar-tests {url}: 200
// {"pickups": [{kind, title, date}]} with the pickups of the coming
// wasteTestDays, read now and not stored, so the setup can show what the
// widget will show before the calendar is added. 422 for an address that
// is not one, 502 fetch_failed with what went wrong.
func (s *Server) postWasteCalendarTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	address, err := settings.CalendarAddress(body.URL)
	if err != nil {
		s.writeSettingsError(w, "test waste calendar", err)
		return
	}
	events, err := s.Calendars.Read(r.Context(), calendar.Source{Name: "Waste", URL: address}, wasteTestDays)
	if err != nil {
		// The error never holds the address; see calendar.withoutURL.
		s.Logger.Warn("waste calendar test failed", "error", err)
		// fetch_failed for every failure, as this endpoint always answered.
		_, message := calendarFailure(err)
		writeError(w, http.StatusBadGateway, codeFetchFailed, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pickups": waste.Pickups(events)})
}
