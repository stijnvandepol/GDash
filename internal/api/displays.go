package api

import (
	"errors"
	"net/http"

	"github.com/stijnvandepol/daycast/internal/casting"
	"github.com/stijnvandepol/daycast/internal/displays"
	"github.com/stijnvandepol/daycast/internal/schedule"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

// defaultTextScale is the size a display gets when a client does not choose.
const defaultTextScale = 100

// listDisplays handles GET /api/displays: {"displays": [...]}, at most
// displays.MaxDisplays, each with the number of screens paired by code and,
// for a display on a Cast device, what its keeper last found there.
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
		PairedScreens int             `json:"pairedScreens"`
		Cast          *casting.Status `json:"cast,omitempty"`
	}
	statuses := s.CastStatus()
	out := make([]listed, len(list))
	for i, d := range list {
		out[i] = listed{Display: d, PairedScreens: counts[d.ID]}
		if st, ok := statuses[d.ID]; ok {
			out[i].Cast = &st
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"displays": out})
}

// createDisplay handles POST /api/displays {id, name, castDeviceId,
// activeFrom, activeUntil, schedule}: 201 with the display, 409 when the id
// is taken or the limit reached, 422 for invalid fields.
func (s *Server) createDisplay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID            string          `json:"id"`
		Name          string          `json:"name"`
		CastDeviceID  string          `json:"castDeviceId"`
		ActiveFrom    string          `json:"activeFrom"`
		ActiveUntil   string          `json:"activeUntil"`
		Schedule      []displays.Slot `json:"schedule"`
		Rotation      []string        `json:"rotation"`
		RotateSeconds int             `json:"rotateSeconds"`
		TextScale     int             `json:"textScale"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	d := displays.Display{
		ID: body.ID, Name: body.Name, CastDeviceID: body.CastDeviceID,
		Hours:    schedule.Hours{From: body.ActiveFrom, Until: body.ActiveUntil},
		Schedule: displays.NormalizeSchedule(body.Schedule), CreatedAt: s.Now().UTC(),
		Rotation: displays.NormalizeRotation(body.Rotation), RotateSeconds: body.RotateSeconds,
		TextScale: body.TextScale,
	}
	if d.TextScale == 0 {
		d.TextScale = defaultTextScale
	}
	if err := displays.Validate(d); err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	if err := s.Store.CreateDisplay(r.Context(), d); err != nil {
		s.writeStoreError(w, "create display", err)
		return
	}
	s.auditUser(r, auditDisplayCreated, d.ID)
	s.DisplaysChanged()
	writeJSON(w, http.StatusCreated, d)
}

// updateDisplay handles PATCH /api/displays/{display} {name?, castDeviceId?,
// dashboardId?, activeFrom?, activeUntil?, schedule?}; an empty dashboardId
// means the default dashboard, empty active hours mean always on, and a
// schedule replaces the whole day plan. It changes only the fields present
// and answers 200 with the display. The id is fixed, because it is part of
// the display's URL.
func (s *Server) updateDisplay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          *string          `json:"name"`
		CastDeviceID  *string          `json:"castDeviceId"`
		DashboardID   *string          `json:"dashboardId"`
		ActiveFrom    *string          `json:"activeFrom"`
		ActiveUntil   *string          `json:"activeUntil"`
		Schedule      *[]displays.Slot `json:"schedule"`
		Rotation      *[]string        `json:"rotation"`
		RotateSeconds *int             `json:"rotateSeconds"`
		TextScale     *int             `json:"textScale"`
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
	if body.Schedule != nil {
		d.Schedule = displays.NormalizeSchedule(*body.Schedule)
	}
	if body.Rotation != nil {
		d.Rotation = displays.NormalizeRotation(*body.Rotation)
	}
	if body.RotateSeconds != nil {
		d.RotateSeconds = *body.RotateSeconds
	}
	if body.TextScale != nil {
		d.TextScale = *body.TextScale
	}
	if err := displays.Validate(d); err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	if err := s.Store.UpdateDisplay(r.Context(), d); err != nil {
		s.writeStoreError(w, "update display", err)
		return
	}
	s.auditUser(r, auditDisplayUpdated, d.ID)
	s.DisplaysChanged()
	writeJSON(w, http.StatusOK, d)
}

// deleteDisplay handles DELETE /api/displays/{display}: 204, or 404.
func (s *Server) deleteDisplay(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDisplay(r.Context(), r.PathValue("display")); err != nil {
		s.writeStoreError(w, "delete display", err)
		return
	}
	s.auditUser(r, auditDisplayDeleted, r.PathValue("display"))
	s.DisplaysChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeStoreError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, store.ErrUnknownDashboard):
		writeValidationError(w, []validation.FieldError{{Field: "dashboardId", Message: "this dashboard does not exist"}})
	case errors.Is(err, store.ErrUnknownScheduleDashboard):
		writeValidationError(w, []validation.FieldError{{Field: "schedule", Message: "a time slot uses a dashboard that does not exist"}})
	case errors.Is(err, store.ErrUnknownRotationDashboard):
		writeValidationError(w, []validation.FieldError{{Field: "rotation", Message: "a dashboard in the rotation does not exist"}})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This display does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "A display with this id already exists, or the display limit is reached.")
	default:
		s.internalError(w, op, err)
	}
}
