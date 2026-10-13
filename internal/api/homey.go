package api

import (
	"context"
	"errors"
	"mime"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
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

// homeyPushKey is the rate limit key of Homey snapshots: there is one Homey.
const homeyPushKey = "homey"

// putHomey handles PUT /api/homey {url, key} or {"mode": "push"}. A
// connection is tried first, so a typo is reported now and not later as a
// failing widget: 200 with {mode, url}, or 422 naming the field that is
// wrong. Push mode drops a connection and waits for snapshots at
// PUT /api/homey/snapshot: 200 with {"mode": "push", "url": ""}, or 422
// when it comes with an address or key.
func (s *Server) putHomey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
		URL  string `json:"url"`
		Key  string `json:"key"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	switch body.Mode {
	case settings.HomeyPush:
		s.setHomeyPush(w, r, body.URL, body.Key)
		return
	case "", settings.HomeyLocal:
	default:
		writeValidationError(w, []validation.FieldError{{Field: "mode", Message: "choose local or push"}})
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
	writeJSON(w, http.StatusOK, settings.Homey{Mode: settings.HomeyLocal, URL: conn.URL})
}

func (s *Server) setHomeyPush(w http.ResponseWriter, r *http.Request, url, key string) {
	var errs []validation.FieldError
	const unused = "a Homey that sends its data has none; leave it empty"
	if url != "" {
		errs = append(errs, validation.FieldError{Field: "url", Message: unused})
	}
	if key != "" {
		errs = append(errs, validation.FieldError{Field: "key", Message: unused})
	}
	if errs != nil {
		writeValidationError(w, errs)
		return
	}
	if err := s.Settings.SetHomeyPush(r.Context()); err != nil {
		s.internalError(w, "store homey push", err)
		return
	}
	s.auditUser(r, auditHomeyConnected, "sends its data (HomeyScript)")
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, settings.Homey{Mode: settings.HomeyPush})
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

// homeySetup returns how Homey is set up, or writes the answer for when it
// is not and reports false.
func (s *Server) homeySetup(w http.ResponseWriter, r *http.Request) (settings.HomeySetup, bool) {
	setup, ok, err := s.Settings.HomeySetup(r.Context())
	if err != nil {
		s.internalError(w, "read homey connection", err)
		return settings.HomeySetup{}, false
	}
	if !ok {
		writeError(w, http.StatusConflict, "not_connected", "Connect Homey under Integrations first.")
	}
	return setup, ok
}

// lastHomeySnapshot returns the snapshot a Homey sent last, or writes the
// answer for when none arrived yet and reports false.
func (s *Server) lastHomeySnapshot(w http.ResponseWriter) (homey.Snapshot, bool) {
	entry := s.SourceStatus()[display.HomeySnapshotSource]
	if len(entry.Value) == 0 {
		writeError(w, http.StatusConflict, "nothing_received", "Your Homey has not sent its data yet. Check the HomeyScript and its Flow under Integrations, Homey.")
		return homey.Snapshot{}, false
	}
	snapshot, err := homey.ParseSnapshot(entry.Value)
	if err != nil {
		s.internalError(w, "read homey snapshot", err)
		return homey.Snapshot{}, false
	}
	return snapshot, true
}

// getHomeyCapabilities handles GET /api/homey/capabilities:
// {"capabilities": [...]}, every readable capability of every device with
// its current value, for the widget editor; for a Homey that sends its
// data, from its last snapshot. 409 when no connection is set up or no
// snapshot arrived yet, 502 when Homey fails.
func (s *Server) getHomeyCapabilities(w http.ResponseWriter, r *http.Request) {
	setup, ok := s.homeySetup(w, r)
	if !ok {
		return
	}
	var list []homey.Capability
	if setup.Pushed {
		snapshot, ok := s.lastHomeySnapshot(w)
		if !ok {
			return
		}
		list = snapshot.Capabilities()
	} else {
		var err error
		if list, err = s.Homey.Capabilities(r.Context(), setup.Connection); err != nil {
			s.Logger.Warn("listing homey capabilities failed", "url", setup.Connection.URL, "error", err)
			writeError(w, http.StatusBadGateway, "upstream_unavailable", "Homey did not answer. Check the connection under Integrations.")
			return
		}
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
// sorted by name; for a Homey that sends its data, from its last snapshot.
// 409 when no connection is set up or no snapshot arrived yet, 502 when
// Homey fails or the key may not read users.
func (s *Server) getHomeyUsers(w http.ResponseWriter, r *http.Request) {
	setup, ok := s.homeySetup(w, r)
	if !ok {
		return
	}
	var users []homey.User
	if setup.Pushed {
		snapshot, ok := s.lastHomeySnapshot(w)
		if !ok {
			return
		}
		users = snapshot.UserList()
	} else {
		var err error
		if users, err = s.Homey.Users(r.Context(), setup.Connection); err != nil {
			s.Logger.Warn("listing homey users failed", "url", setup.Connection.URL, "error", err)
			message := "Homey did not answer. Check the connection under Integrations."
			if errors.Is(err, homey.ErrUnauthorized) {
				message = "Homey did not list its users. Connect again with an API key that has View users."
			}
			writeError(w, http.StatusBadGateway, "upstream_unavailable", message)
			return
		}
	}
	list := make([]homeyUser, len(users))
	for i, u := range users {
		list[i] = homeyUser{ID: u.ID, Name: u.Name}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": list})
}

// putHomeySnapshot handles PUT /api/homey/snapshot with an API token or an
// admin session: the snapshot that a HomeyScript sends, as application/json
// of at most 1 MB, {"devices": [{id, name, zone, available, caps: {<id>:
// {title, value, units}}}], "users": [{id, name, present, asleep}]}. Odd
// entries are left out (homey.ParseSnapshot). 204; 409 when Homey is not
// set up to send its data, 413 over 1 MB, 415 for another content type, 422
// for a body that is no snapshot, 429 within 5 seconds of the last
// accepted one. Not audited: the Flow sends one every minute.
func (s *Server) putHomeySnapshot(w http.ResponseWriter, r *http.Request) {
	setup, ok, err := s.Settings.HomeySetup(r.Context())
	if err != nil {
		s.internalError(w, "read homey connection", err)
		return
	}
	if !ok || !setup.Pushed {
		writeError(w, http.StatusConflict, "not_pushed", "GDash does not wait for data from Homey. Under Settings, Homey, choose that your Homey sends its data.")
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Send the data as application/json.")
		return
	}
	if !s.pushes.Attempt(homeyPushKey) {
		writeRateLimited(w)
		return
	}
	raw, err := custom.ReadAnswer(http.MaxBytesReader(w, r.Body, custom.MaxResponseBytes+1))
	if err != nil {
		s.pushes.Release(homeyPushKey)
		writeAnswerError(w, err)
		return
	}
	snapshot, err := homey.ParseSnapshot(raw)
	if err != nil {
		s.pushes.Release(homeyPushKey)
		writeError(w, http.StatusUnprocessableEntity, "not_a_snapshot", "This is JSON, but not what the GDash HomeyScript sends: it has no list of devices.")
		return
	}
	if err := s.ReceiveHomey(snapshot); err != nil {
		s.pushes.Release(homeyPushKey)
		if errors.Is(err, datahub.ErrNotPushed) {
			writeError(w, http.StatusConflict, "not_pushed", "GDash is still switching Homey over. Try again in a moment.")
			return
		}
		s.internalError(w, "receive homey snapshot", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
