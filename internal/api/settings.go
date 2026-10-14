package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/news"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
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

// entryStatus is whether a data hub entry works, for the Settings page.
func entryStatus(entry datahub.Entry) sourceStatus {
	st := sourceStatus{Failing: entry.Failing}
	if !entry.FetchedAt.IsZero() {
		st.UpdatedAt = &entry.FetchedAt
	}
	return st
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
		status[key] = entryStatus(entry)
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
// of the texts and dates on the TVs, "en", "nl", "de" or "fr".
// 422 for another one.
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

// putUpdateCheck handles PUT /api/settings/update-check {enabled}: turns the
// daily check for a new release on or off. 200 with {enabled}.
func (s *Server) putUpdateCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Enabled == nil {
		writeValidationError(w, []validation.FieldError{{Field: "enabled", Message: "use true or false"}})
		return
	}
	if err := s.Settings.SetUpdateCheck(r.Context(), *body.Enabled); err != nil {
		s.internalError(w, "set update check", err)
		return
	}
	detail := "off"
	if *body.Enabled {
		detail = "on"
	}
	s.auditUser(r, auditUpdateCheckChanged, detail)
	s.Updates.Changed()
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": *body.Enabled})
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

// putSolar handles PUT /api/settings/solar {location, arrays,
// performanceRatio, source}: the household's solar panels, at their own
// location or, with a null location, the weather location. 200 with the
// panels; 422 naming every invalid field.
func (s *Server) putSolar(w http.ResponseWriter, r *http.Request) {
	var body settings.Solar
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.Settings.SetSolar(r.Context(), body); err != nil {
		s.writeSettingsError(w, "set solar panels", err)
		return
	}
	s.auditUser(r, auditSolarChanged, body.Source)
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, body)
}

// deleteSolar handles DELETE /api/settings/solar: 204.
func (s *Server) deleteSolar(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearSolar(r.Context()); err != nil {
		s.internalError(w, "clear solar panels", err)
		return
	}
	s.auditUser(r, auditSolarRemoved, "")
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

// patchCalendar handles PATCH /api/calendars/{id} {color}: 204; 404 for an
// unknown calendar, 422 for a colour that is not offered.
func (s *Server) patchCalendar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Color string `json:"color"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "This source does not exist.")
		return
	}
	if err := s.Settings.SetCalendarColor(r.Context(), id, body.Color); err != nil {
		s.writeSettingsError(w, "set calendar color", err)
		return
	}
	s.auditUser(r, auditCalendarChanged, "id "+strconv.FormatInt(id, 10)+": "+body.Color)
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// getNewsCatalogue handles GET /api/news-catalogue: the news feeds GDash
// offers to choose from, Dutch ones first, and their categories.
func (s *Server) getNewsCatalogue(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"categories": news.Categories, "sources": news.Catalogue()})
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
