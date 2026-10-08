package api

import (
	"errors"
	"net/http"

	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

type dashboardBody struct {
	Name    string           `json:"name"`
	Theme   string           `json:"theme"`
	Widgets []display.Widget `json:"widgets"`
}

// getWidgetTypes handles GET /api/widget-types: the widget catalogue with
// minimum sizes and the settings each type takes, the grid size and the
// themes.
func (s *Server) getWidgetTypes(w http.ResponseWriter, _ *http.Request) {
	type widgetType struct {
		MinW     int                        `json:"minW"`
		MinH     int                        `json:"minH"`
		Settings map[string]display.Setting `json:"settings"`
	}
	types := map[string]widgetType{}
	for name, t := range display.WidgetTypes {
		settings := t.Settings
		if settings == nil {
			settings = map[string]display.Setting{}
		}
		types[name] = widgetType{MinW: t.MinW, MinH: t.MinH, Settings: settings}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"columns": display.GridColumns, "rows": display.GridRows, "types": types, "themes": display.Themes,
	})
}

// listDashboards handles GET /api/dashboards: {"dashboards": [...]}.
func (s *Server) listDashboards(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Dashboards(r.Context())
	if err != nil {
		s.internalError(w, "list dashboards", err)
		return
	}
	if list == nil {
		list = []display.Dashboard{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboards": list})
}

// getDashboard handles GET /api/dashboards/{dashboard}.
func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Dashboard(r.Context(), r.PathValue("dashboard"))
	if err != nil {
		s.writeDashboardError(w, "get dashboard", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// createDashboard handles POST /api/dashboards {id, name, widgets}: 201 with
// the stored dashboard; 409 when the id is taken; 422 for an invalid layout.
func (s *Server) createDashboard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
		dashboardBody
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	d, ok := s.validDashboard(w, body.ID, body.dashboardBody)
	if !ok {
		return
	}
	if err := s.Store.CreateDashboard(r.Context(), d, s.Now()); err != nil {
		s.writeDashboardError(w, "create dashboard", err)
		return
	}
	s.DashboardsChanged()
	writeJSON(w, http.StatusCreated, d)
}

// updateDashboard handles PUT /api/dashboards/{dashboard} {name, widgets}: the
// whole layout is replaced. Displays showing it update at once.
func (s *Server) updateDashboard(w http.ResponseWriter, r *http.Request) {
	var body dashboardBody
	if !decodeJSON(w, r, &body) {
		return
	}
	d, ok := s.validDashboard(w, r.PathValue("dashboard"), body)
	if !ok {
		return
	}
	if err := s.Store.UpdateDashboard(r.Context(), d, s.Now()); err != nil {
		s.writeDashboardError(w, "update dashboard", err)
		return
	}
	s.DashboardsChanged()
	writeJSON(w, http.StatusOK, d)
}

// deleteDashboard handles DELETE /api/dashboards/{dashboard}: 204; its
// displays fall back to the default dashboard. 409 for the default itself.
func (s *Server) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDashboard(r.Context(), r.PathValue("dashboard")); err != nil {
		s.writeDashboardError(w, "delete dashboard", err)
		return
	}
	s.DashboardsChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) validDashboard(w http.ResponseWriter, id string, body dashboardBody) (display.Dashboard, bool) {
	d := display.Normalize(display.Dashboard{ID: id, Name: body.Name, Theme: body.Theme, Widgets: body.Widgets})
	if d.Widgets == nil {
		d.Widgets = []display.Widget{}
	}
	if err := display.ValidateDashboard(d); err != nil {
		writeValidationError(w, validation.Fields(err))
		return display.Dashboard{}, false
	}
	return d, true
}

func (s *Server) writeDashboardError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This dashboard does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "A dashboard with this id exists, the limit is reached, or it is the default dashboard.")
	default:
		s.internalError(w, op, err)
	}
}
