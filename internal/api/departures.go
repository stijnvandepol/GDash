package api

import (
	"context"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/departures"
	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// StopSearcher finds public transport stops by name for the Departures
// widget. The list it searches is loaded from OVapi and kept in memory.
type StopSearcher interface {
	Search(ctx context.Context, query string) ([]departures.Stop, error)
}

// getDepartureStops handles GET /api/departures/stops?q=utrecht+cs:
// {"stops": [{"code", "name"}]}, at most 20, best matches first. 422 for a
// query that is too short or long, 502 when the list of stops cannot be
// loaded from OVapi.
func (s *Server) getDepartureStops(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if !weather.ValidQuery(query) {
		writeValidationError(w, []validation.FieldError{{Field: "q", Message: "type 2 to 100 characters"}})
		return
	}
	stops, err := s.Stops.Search(r.Context(), query)
	if err != nil {
		s.Logger.Warn("stop search failed", "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "The list of stops is not available right now. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stops": stops})
}
