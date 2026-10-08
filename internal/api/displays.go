package api

import (
	"errors"
	"net/http"

	"github.com/stijnvandepol/daycast/internal/displays"
	"github.com/stijnvandepol/daycast/internal/schedule"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

// listDisplays handles GET /api/displays: {"displays": [...]}, at most
// displays.MaxDisplays, each with the number of screens paired by code.
func (s *Server) listDisplays(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Displays(r.Context())
	if err != nil {
		s.internalError(w, "list displays", err)
		return
	}
	counts, err := s.Store.PairedScreenCounts(r.Context())
	if err != nil {
		s.internalError(w, "count paired screens", err)
		return
	}
	type listed struct {
		displays.Display
		PairedScreens int `json:"pairedScreens"`
	}
	out := make([]listed, len(list))
	for i, d := range list {
		out[i] = listed{Display: d, PairedScreens: counts[d.ID]}
	}
	writeJSON(w, http.StatusOK, map[string]any{"displays": out})
}

// createDisplay handles POST /api/displays {id, name, castDeviceId}: 201 with
// the display, 409 when the id is taken or the limit reached, 422 for
// invalid fields.
func (s *Server) createDisplay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		CastDeviceID string `json:"castDeviceId"`
		ActiveFrom   string `json:"activeFrom"`
		ActiveUntil  string `json:"activeUntil"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	d := displays.Display{
		ID: body.ID, Name: body.Name, CastDeviceID: body.CastDeviceID,
		Hours: schedule.Hours{From: body.ActiveFrom, Until: body.ActiveUntil}, CreatedAt: s.Now().UTC(),
	}
	if err := displays.Validate(d); err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	if err := s.Store.CreateDisplay(r.Context(), d); err != nil {
		s.writeStoreError(w, "create display", err)
		return
	}
	s.DisplaysChanged()
	writeJSON(w, http.StatusCreated, d)
}

// updateDisplay handles PATCH /api/displays/{display} {name?, castDeviceId?,
// dashboardId?, activeFrom?, activeUntil?}; an empty dashboardId means the
// default dashboard, empty active hours mean always on:
// changes only the fields present and answers 200 with the display. The
// id is fixed, because it is part of the display's URL.
func (s *Server) updateDisplay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         *string `json:"name"`
		CastDeviceID *string `json:"castDeviceId"`
		DashboardID  *string `json:"dashboardId"`
		ActiveFrom   *string `json:"activeFrom"`
		ActiveUntil  *string `json:"activeUntil"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	d, err := s.Store.Display(r.Context(), r.PathValue("display"))
	if err != nil {
		s.writeStoreError(w, "find display", err)
		return
	}
	if body.Name != nil {
		d.Name = *body.Name
	}
	if body.CastDeviceID != nil {
		d.CastDeviceID = *body.CastDeviceID
	}
	if body.DashboardID != nil {
		d.DashboardID = *body.DashboardID
	}
	if body.ActiveFrom != nil {
		d.From = *body.ActiveFrom
	}
	if body.ActiveUntil != nil {
		d.Until = *body.ActiveUntil
	}
	if err := displays.Validate(d); err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	if err := s.Store.UpdateDisplay(r.Context(), d); err != nil {
		s.writeStoreError(w, "update display", err)
		return
	}
	s.DisplaysChanged()
	writeJSON(w, http.StatusOK, d)
}

// deleteDisplay handles DELETE /api/displays/{display}: 204, or 404.
func (s *Server) deleteDisplay(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDisplay(r.Context(), r.PathValue("display")); err != nil {
		s.writeStoreError(w, "delete display", err)
		return
	}
	s.DisplaysChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeStoreError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, store.ErrUnknownDashboard):
		writeValidationError(w, []validation.FieldError{{Field: "dashboardId", Message: "this dashboard does not exist"}})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This display does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "A display with this id already exists, or the display limit is reached.")
	default:
		s.internalError(w, op, err)
	}
}
