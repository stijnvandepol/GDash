package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/p2000"
	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// P2000PlaceFinder looks a place up on alarmeringen.nl for the P2000
// widget's Test button.
type P2000PlaceFinder interface {
	FindPlace(ctx context.Context, name string) (p2000.Place, error)
}

// Error codes of a P2000 place test that failed, so the editor can say
// what to do.
const (
	codePlaceUnknown       = "place_unknown"
	codePlaceWithoutAlerts = "place_without_alerts"
)

// postP2000PlaceTest handles POST /api/p2000-place-tests {name}: 200
// {"name", "slug", "region", "regionName"} for a place alarmeringen.nl
// knows, read now and not stored. 422 for a name of fewer than 2 or more
// than 100 characters; 502 with place_unknown, place_without_alerts or
// fetch_failed.
func (s *Server) postP2000PlaceTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !weather.ValidQuery(body.Name) {
		writeValidationError(w, []validation.FieldError{{Field: "name", Message: "type 2 to 100 characters"}})
		return
	}
	place, err := s.P2000Places.FindPlace(r.Context(), body.Name)
	switch {
	case errors.Is(err, p2000.ErrUnknownPlace):
		writeError(w, http.StatusBadGateway, codePlaceUnknown, "alarmeringen.nl knows no place by this name. Check the spelling; a name that more places share may need its province or municipality, such as Hengelo (Ov). Or choose the safety region instead.")
		return
	case errors.Is(err, p2000.ErrPlaceWithoutAlerts):
		writeError(w, http.StatusBadGateway, codePlaceWithoutAlerts, "This place had no alerts recently, so GDash cannot tell its safety region. Choose the safety region instead.")
		return
	case err != nil:
		s.Logger.Warn("P2000 place test failed", "error", err)
		writeError(w, http.StatusBadGateway, codeFetchFailed, "alarmeringen.nl could not be reached. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": place.Name, "slug": place.Slug, "region": place.Region, "regionName": p2000.RegionName(place.Region),
	})
}
