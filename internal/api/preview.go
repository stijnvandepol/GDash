package api

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// previewPage is the dashboard shown as a TV shows it, for the editor to
// frame. Like the web interface it loads without a session; the preview
// endpoints it polls require one.
func (s *Server) previewPage(w http.ResponseWriter, r *http.Request) {
	if !display.ValidDashboardID(r.PathValue("dashboard")) {
		http.NotFound(w, r)
		return
	}
	origins, err := s.previewFrameOrigins(r)
	if err != nil {
		s.Logger.Error("could not read the web pages for the preview; they are not shown", "error", err)
	}
	w.Header().Set("Content-Security-Policy", withFrameSrc(previewCSP, origins))
	s.servePage(w, r, "preview/index.html")
}

// getDashboardPreview handles GET /api/dashboards/{dashboard}/preview
// ?textScale=: the snapshot a screen showing the stored dashboard gets now.
func (s *Server) getDashboardPreview(w http.ResponseWriter, r *http.Request) {
	textScale, ok := previewTextScale(w, r)
	if !ok {
		return
	}
	d, err := s.Store.Dashboard(r.Context(), r.PathValue("dashboard"))
	if err != nil {
		s.writeDashboardError(w, "preview dashboard", err)
		return
	}
	s.writePreview(w, r, d, textScale)
}

// postDashboardPreview handles POST /api/dashboards/{dashboard}/preview
// ?textScale= {name, theme, font, nightTheme, widgets}: the snapshot for a layout
// that is not saved yet. It is validated like a save and never stored; the
// dashboard it belongs to must exist.
func (s *Server) postDashboardPreview(w http.ResponseWriter, r *http.Request) {
	textScale, ok := previewTextScale(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.Dashboard(r.Context(), r.PathValue("dashboard")); err != nil {
		s.writeDashboardError(w, "preview dashboard", err)
		return
	}
	var body dashboardBody
	if !decodeJSON(w, r, &body) {
		return
	}
	d, ok := s.validDashboard(w, r.PathValue("dashboard"), body)
	if !ok {
		return
	}
	s.writePreview(w, r, d, textScale)
}

func (s *Server) writePreview(w http.ResponseWriter, r *http.Request, d display.Dashboard, textScale int) {
	language, err := s.Settings.Language(r.Context())
	if err != nil {
		s.internalError(w, "read the display language", err)
		return
	}
	snapshot, err := display.PreviewSnapshot(s.Now(), d, s.Data(), language, textScale)
	if err != nil {
		s.internalError(w, "build preview", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, snapshot)
}

// previewTextScale reads the textScale query parameter: one of the sizes a
// display can have, 100 when absent. It answers 422 itself for another value.
func previewTextScale(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("textScale")
	if raw == "" {
		return defaultTextScale, true
	}
	scale, err := strconv.Atoi(raw)
	if err != nil || !slices.Contains(displays.TextScales, scale) {
		writeValidationError(w, []validation.FieldError{{Field: "textScale", Message: "choose 100, 125, 150 or 200 percent"}})
		return 0, false
	}
	return scale, true
}
