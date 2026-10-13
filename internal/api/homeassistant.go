package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// HomeAssistantReader is the part of the Home Assistant client the web
// interface uses: trying a connection and listing entities to choose from.
type HomeAssistantReader interface {
	Check(ctx context.Context, conn homeassistant.Connection) error
	Entities(ctx context.Context, conn homeassistant.Connection) ([]homeassistant.Entity, error)
}

// putHomeAssistant handles PUT /api/homeassistant {url, token}. The
// connection is tried first, so a typo is reported now and not later as a
// failing widget: 200 with {url}, or 422 naming the field that is wrong.
func (s *Server) putHomeAssistant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	conn, err := settings.CheckHomeAssistant(body.URL, body.Token)
	if err != nil {
		s.writeSettingsError(w, "check home assistant", err)
		return
	}
	if err := s.HomeAssistant.Check(r.Context(), conn); err != nil {
		s.Logger.Warn("home assistant connection test failed", "url", conn.URL, "error", err)
		writeValidationError(w, []validation.FieldError{connectionProblem(err)})
		return
	}
	if err := s.Settings.SetHomeAssistant(r.Context(), conn); err != nil {
		s.internalError(w, "store home assistant", err)
		return
	}
	// The address only: the token is a secret.
	s.auditUser(r, auditHomeAssistantConnected, conn.URL)
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, settings.HomeAssistant{URL: conn.URL})
}

// connectionProblem tells the admin which field to fix.
func connectionProblem(err error) validation.FieldError {
	switch {
	case errors.Is(err, homeassistant.ErrUnauthorized):
		return validation.FieldError{Field: "token", Message: "Home Assistant did not accept this token; make a new one under your profile, Security"}
	case errors.Is(err, homeassistant.ErrNotHomeAssistant):
		return validation.FieldError{Field: "url", Message: "this address answers, but not as Home Assistant; check the port, usually 8123"}
	default:
		return validation.FieldError{Field: "url", Message: "Home Assistant cannot be reached at this address from the GDash server"}
	}
}

// deleteHomeAssistant handles DELETE /api/homeassistant: 204.
func (s *Server) deleteHomeAssistant(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearHomeAssistant(r.Context()); err != nil {
		s.internalError(w, "clear home assistant", err)
		return
	}
	s.auditUser(r, auditHomeAssistantRemoved, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// getHomeAssistantEntities handles GET /api/homeassistant/entities:
// {"entities": [...]}, every entity with its name and state, for the widget
// editor. 409 when no connection is set up, 502 when Home Assistant fails.
func (s *Server) getHomeAssistantEntities(w http.ResponseWriter, r *http.Request) {
	conn, ok, err := s.Settings.HomeAssistantConnection(r.Context())
	if err != nil {
		s.internalError(w, "read home assistant connection", err)
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "not_connected", "Connect Home Assistant under Integrations first.")
		return
	}
	list, err := s.HomeAssistant.Entities(r.Context(), conn)
	if err != nil {
		s.Logger.Warn("listing home assistant entities failed", "url", conn.URL, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Home Assistant did not answer. Check the connection under Integrations.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entities": list})
}
