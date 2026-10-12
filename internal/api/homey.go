package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// HomeyReader is the part of the Homey client the web interface uses:
// trying a connection and listing capabilities and users to choose from.
type HomeyReader interface {
	Check(ctx context.Context, conn homey.Connection) error
	Capabilities(ctx context.Context, conn homey.Connection) ([]homey.Capability, error)
	Users(ctx context.Context, conn homey.Connection) ([]homey.User, error)
}

// putHomey handles PUT /api/homey {url, key}. The connection is tried
// first, so a typo is reported now and not later as a failing widget: 200
// with {url}, or 422 naming the field that is wrong.
func (s *Server) putHomey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	conn, err := settings.CheckHomey(body.URL, body.Key)
	if err != nil {
		s.writeSettingsError(w, "check homey", err)
		return
	}
	if err := s.Homey.Check(r.Context(), conn); err != nil {
		s.Logger.Warn("homey connection test failed", "url", conn.URL, "error", err)
		writeValidationError(w, []validation.FieldError{homeyProblem(err)})
		return
	}
	if err := s.Settings.SetHomey(r.Context(), conn); err != nil {
		s.internalError(w, "store homey", err)
		return
	}
	// The address only: the key is a secret.
	s.auditUser(r, auditHomeyConnected, conn.URL)
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, settings.Homey{URL: conn.URL})
}

// homeyProblem tells the admin which field to fix.
func homeyProblem(err error) validation.FieldError {
	switch {
	case errors.Is(err, homey.ErrUnauthorized):
		return validation.FieldError{Field: "key", Message: "Homey did not accept this API key; make a new one with View devices and View zones"}
	case errors.Is(err, homey.ErrNotHomey):
		return validation.FieldError{Field: "url", Message: "this address answers, but not as a Homey Pro; check the address, or add the port 4859 for Homey Self-Hosted Server"}
	default:
		return validation.FieldError{Field: "url", Message: "Homey cannot be reached at this address from the GDash server"}
	}
}

// deleteHomey handles DELETE /api/homey: 204.
func (s *Server) deleteHomey(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearHomey(r.Context()); err != nil {
		s.internalError(w, "clear homey", err)
		return
	}
	s.auditUser(r, auditHomeyRemoved, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// homeyConnection returns the stored connection, or writes the answer for
// when there is none and reports false.
func (s *Server) homeyConnection(w http.ResponseWriter, r *http.Request) (homey.Connection, bool) {
	conn, ok, err := s.Settings.HomeyConnection(r.Context())
	if err != nil {
		s.internalError(w, "read homey connection", err)
		return homey.Connection{}, false
	}
	if !ok {
		writeError(w, http.StatusConflict, "not_connected", "Connect Homey under Settings first.")
	}
	return conn, ok
}

// getHomeyCapabilities handles GET /api/homey/capabilities:
// {"capabilities": [...]}, every readable capability of every device with
// its current value, for the widget editor. 409 when no connection is set
// up, 502 when Homey fails.
func (s *Server) getHomeyCapabilities(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.homeyConnection(w, r)
	if !ok {
		return
	}
	list, err := s.Homey.Capabilities(r.Context(), conn)
	if err != nil {
		s.Logger.Warn("listing homey capabilities failed", "url", conn.URL, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Homey did not answer. Check the connection under Settings.")
		return
	}
	if list == nil {
		list = []homey.Capability{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"capabilities": list})
}

// homeyUser is a user as the editor lists it: whether someone is home is
// for the screens that show it, not for the editor.
type homeyUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// getHomeyUsers handles GET /api/homey/users: {"users": [{id, name}]},
// sorted by name. 409 when no connection is set up, 502 when Homey fails or
// the key may not read users.
func (s *Server) getHomeyUsers(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.homeyConnection(w, r)
	if !ok {
		return
	}
	users, err := s.Homey.Users(r.Context(), conn)
	if err != nil {
		s.Logger.Warn("listing homey users failed", "url", conn.URL, "error", err)
		message := "Homey did not answer. Check the connection under Settings."
		if errors.Is(err, homey.ErrUnauthorized) {
			message = "Homey did not list its users. Connect again with an API key that has View users."
		}
		writeError(w, http.StatusBadGateway, "upstream_unavailable", message)
		return
	}
	list := make([]homeyUser, len(users))
	for i, u := range users {
		list[i] = homeyUser{ID: u.ID, Name: u.Name}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": list})
}
