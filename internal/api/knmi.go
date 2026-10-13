package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/knmi"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// KNMIChecker tries a KNMI key.
type KNMIChecker interface {
	Check(ctx context.Context, key string) error
}

// knmiView is GET /api/knmi: whether a key is set, never the key, and the
// status of the measured weather.
type knmiView struct {
	settings.KNMI
	Data *sourceStatus `json:"data,omitempty"`
}

// getKNMI handles GET /api/knmi.
func (s *Server) getKNMI(w http.ResponseWriter, r *http.Request) {
	view, err := s.Settings.KNMIView(r.Context())
	if err != nil {
		s.internalError(w, "read knmi key", err)
		return
	}
	writeJSON(w, http.StatusOK, knmiView{KNMI: view, Data: statusOf(s.SourceStatus(), display.KNMISource)})
}

// putKNMI handles PUT /api/knmi {key}. The key is tried first: 200 with
// {connected}, or 422 on the key field when the KNMI refuses it or cannot
// be reached.
func (s *Server) putKNMI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	key, err := settings.CheckKNMIKey(body.Key)
	if err != nil {
		s.writeSettingsError(w, "check knmi key", err)
		return
	}
	if err := s.KNMI.Check(r.Context(), key); err != nil {
		// The error never holds the key (see internal/knmi).
		s.Logger.Warn("knmi key test failed", "error", err)
		writeValidationError(w, []validation.FieldError{knmiProblem(err)})
		return
	}
	if err := s.Settings.SetKNMIKey(r.Context(), key); err != nil {
		s.internalError(w, "store knmi key", err)
		return
	}
	s.auditUser(r, auditKNMIConnected, "")
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, settings.KNMI{Connected: true})
}

func knmiProblem(err error) validation.FieldError {
	if errors.Is(err, knmi.ErrUnauthorized) {
		return validation.FieldError{Field: "key", Message: "the KNMI did not accept this key; check that it is an EDR API key, or request a new one"}
	}
	return validation.FieldError{Field: "key", Message: "the KNMI Data Platform cannot be reached from the GDash server; try again later"}
}

// deleteKNMI handles DELETE /api/knmi: 204.
func (s *Server) deleteKNMI(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearKNMIKey(r.Context()); err != nil {
		s.internalError(w, "clear knmi key", err)
		return
	}
	s.auditUser(r, auditKNMIRemoved, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}
