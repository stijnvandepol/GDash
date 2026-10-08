package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/stijnvandepol/daycast/internal/settings"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
	"github.com/stijnvandepol/daycast/internal/weather"
)

// PlaceSearcher finds places by name for the location setting.
type PlaceSearcher interface {
	Search(ctx context.Context, query string) ([]weather.Place, error)
}

// sourceStatus tells the Settings page whether a source works.
type sourceStatus struct {
	// UpdatedAt is when the last value arrived; absent before the first.
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	Failing   bool       `json:"failing"`
}

// getSettings handles GET /api/settings: the settings and, per configured
// data source, whether it works.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	current, err := s.Settings.Get(r.Context())
	if err != nil {
		s.internalError(w, "read settings", err)
		return
	}
	status := map[string]sourceStatus{}
	for key, entry := range s.SourceStatus() {
		st := sourceStatus{Failing: entry.Failing}
		if !entry.FetchedAt.IsZero() {
			st.UpdatedAt = &entry.FetchedAt
		}
		status[key] = st
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": current, "status": status})
}

// putLocation handles PUT /api/settings/location {name, latitude, longitude}.
// A missing or null coordinate is a 422, not 0: 0, 0 is a real place.
func (s *Server) putLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string   `json:"name"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var missing []validation.FieldError
	if body.Latitude == nil {
		missing = append(missing, validation.FieldError{Field: "latitude", Message: "use a number from -90 to 90"})
	}
	if body.Longitude == nil {
		missing = append(missing, validation.FieldError{Field: "longitude", Message: "use a number from -180 to 180"})
	}
	if missing != nil {
		writeValidationError(w, missing)
		return
	}
	loc := settings.Location{Name: body.Name, Latitude: *body.Latitude, Longitude: *body.Longitude}
	if err := s.Settings.SetLocation(r.Context(), loc); err != nil {
		s.writeSettingsError(w, "set location", err)
		return
	}
	s.auditUser(r, auditLocationChanged, loc.Name)
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, loc)
}

// putLanguage handles PUT /api/settings/language {language}: the language
// of the texts and dates on the TVs, "en" or "nl". 422 for another one.
func (s *Server) putLanguage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Language string `json:"language"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.Settings.SetLanguage(r.Context(), body.Language); err != nil {
		s.writeSettingsError(w, "set language", err)
		return
	}
	s.auditUser(r, auditLanguageChanged, body.Language)
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, body)
}

// deleteLocation handles DELETE /api/settings/location: 204.
func (s *Server) deleteLocation(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearLocation(r.Context()); err != nil {
		s.internalError(w, "clear location", err)
		return
	}
	s.auditUser(r, auditLocationCleared, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

type sourceBody struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// postNewsFeed handles POST /api/news-feeds {name, url}: 201 with the feed.
func (s *Server) postNewsFeed(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !decodeJSON(w, r, &body) {
		return
	}
	feed, err := s.Settings.AddNewsFeed(r.Context(), body.Name, body.URL)
	if err != nil {
		s.writeSettingsError(w, "add news feed", err)
		return
	}
	s.auditUser(r, auditNewsFeedAdded, feed.Name)
	s.SettingsChanged()
	writeJSON(w, http.StatusCreated, feed)
}

// postCalendar handles POST /api/calendars {name, url}: 201 with the
// calendar, which never includes the secret address.
func (s *Server) postCalendar(w http.ResponseWriter, r *http.Request) {
	var body sourceBody
	if !decodeJSON(w, r, &body) {
		return
	}
	cal, err := s.Settings.AddCalendar(r.Context(), body.Name, body.URL)
	if err != nil {
		s.writeSettingsError(w, "add calendar", err)
		return
	}
	// The name only: the address is a secret.
	s.auditUser(r, auditCalendarAdded, cal.Name)
	s.SettingsChanged()
	writeJSON(w, http.StatusCreated, cal)
}

// deleteNewsFeed handles DELETE /api/news-feeds/{id}: 204, or 404.
func (s *Server) deleteNewsFeed(w http.ResponseWriter, r *http.Request) {
	s.deleteSource(w, r, s.Settings.DeleteNewsFeed, auditNewsFeedRemoved)
}

// deleteCalendar handles DELETE /api/calendars/{id}: 204, or 404.
func (s *Server) deleteCalendar(w http.ResponseWriter, r *http.Request) {
	s.deleteSource(w, r, s.Settings.DeleteCalendar, auditCalendarRemoved)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request, remove func(context.Context, int64) error, action string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "This source does not exist.")
		return
	}
	if err := remove(r.Context(), id); err != nil {
		s.writeSettingsError(w, "delete source", err)
		return
	}
	s.auditUser(r, action, "id "+strconv.FormatInt(id, 10))
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// getPlaces handles GET /api/places?name=Oss: {"places": [...]}. The name is
// sent to Open-Meteo's place search; 502 when that search is unavailable.
func (s *Server) getPlaces(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("name")
	if !weather.ValidQuery(query) {
		writeValidationError(w, []validation.FieldError{{Field: "name", Message: "type 2 to 100 characters"}})
		return
	}
	places, err := s.Places.Search(r.Context(), query)
	if err != nil {
		s.Logger.Warn("place search failed", "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "The place search is not available right now. Try again, or enter the coordinates.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": places})
}

func (s *Server) writeSettingsError(w http.ResponseWriter, op string, err error) {
	switch {
	case validation.Fields(err) != nil:
		writeValidationError(w, validation.Fields(err))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This source does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "The limit is reached. Remove a source first.")
	default:
		s.internalError(w, op, err)
	}
}
