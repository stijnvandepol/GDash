package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/immich"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// ImmichReader is the part of the Immich client the server uses: trying a
// connection by listing its albums, and fetching a photo for a screen.
type ImmichReader interface {
	Albums(ctx context.Context, conn immich.Connection) ([]immich.Album, error)
	Thumbnail(ctx context.Context, conn immich.Connection, assetID string) ([]byte, string, error)
}

// handleImmich adds the routes of the Immich connection, for a logged-in
// admin, and of its photos, also for the screens that show them.
func (s *Server) handleImmich(mux *http.ServeMux) {
	mux.Handle("PUT /api/immich", s.requireUser(s.putImmich))
	mux.Handle("DELETE /api/immich", s.requireUser(s.deleteImmich))
	mux.Handle("GET /api/immich/albums", s.requireUser(s.getImmichAlbums))
	mux.Handle("PUT /api/immich/album", s.requireUser(s.putImmichAlbum))
	mux.HandleFunc("GET /api/immich/assets/{id}", s.getImmichAsset)
}

// putImmich handles PUT /api/immich {url, key}. The connection is tried
// first, so a typo is reported now: 200 with {url, albumId, albumName}, or
// 422 naming the field that is wrong. The album stays chosen when the
// address stays the same.
func (s *Server) putImmich(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	conn, err := settings.CheckImmich(body.URL, body.Key)
	if err != nil {
		s.writeSettingsError(w, "check immich", err)
		return
	}
	if _, err := s.Immich.Albums(r.Context(), conn); err != nil {
		s.Logger.Warn("immich connection test failed", "url", conn.URL, "error", err)
		writeValidationError(w, []validation.FieldError{immichProblem(err)})
		return
	}
	if err := s.Settings.SetImmich(r.Context(), conn); err != nil {
		s.internalError(w, "store immich", err)
		return
	}
	s.auditUser(r, auditImmichConnected, conn.URL)
	s.SettingsChanged()
	s.writeImmich(w, r)
}

func (s *Server) writeImmich(w http.ResponseWriter, r *http.Request) {
	current, err := s.Settings.Get(r.Context())
	if err != nil {
		s.internalError(w, "read immich", err)
		return
	}
	writeJSON(w, http.StatusOK, current.Immich)
}

// immichProblem tells the admin which field to fix.
func immichProblem(err error) validation.FieldError {
	switch {
	case errors.Is(err, immich.ErrUnauthorized):
		return validation.FieldError{Field: "key", Message: "Immich did not accept this API key; make one with at least album.read, asset.read and asset.view"}
	case errors.Is(err, immich.ErrNotImmich):
		return validation.FieldError{Field: "url", Message: "this address answers, but not as Immich; use the address you open Immich at, with its port"}
	default:
		return validation.FieldError{Field: "url", Message: "Immich cannot be reached at this address from the GDash server"}
	}
}

// deleteImmich handles DELETE /api/immich: 204.
func (s *Server) deleteImmich(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearImmich(r.Context()); err != nil {
		s.internalError(w, "clear immich", err)
		return
	}
	s.auditUser(r, auditImmichRemoved, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// immichConnection returns the stored connection, or writes the answer for
// when there is none and reports false.
func (s *Server) immichConnection(w http.ResponseWriter, r *http.Request) (immich.Connection, bool) {
	conn, ok, err := s.Settings.ImmichConnection(r.Context())
	if err != nil {
		s.internalError(w, "read immich connection", err)
		return immich.Connection{}, false
	}
	if !ok {
		writeError(w, http.StatusConflict, "not_connected", "Connect Immich under Settings first.")
	}
	return conn, ok
}

// getImmichAlbums handles GET /api/immich/albums: {"albums": [{id, name,
// assetCount}]}. 409 when no connection is set up, 502 when Immich fails.
func (s *Server) getImmichAlbums(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.immichConnection(w, r)
	if !ok {
		return
	}
	albums, err := s.Immich.Albums(r.Context(), conn)
	if err != nil {
		s.Logger.Warn("listing immich albums failed", "url", conn.URL, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Immich did not list its albums. Check the connection under Settings.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
}

// putImmichAlbum handles PUT /api/immich/album {albumId}: chooses the album
// that Photos widgets show. 200 with the connection, 422 for an album the
// key cannot read, 409 without a connection, 502 when Immich fails.
func (s *Server) putImmichAlbum(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AlbumID string `json:"albumId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	conn, ok := s.immichConnection(w, r)
	if !ok {
		return
	}
	albums, err := s.Immich.Albums(r.Context(), conn)
	if err != nil {
		s.Logger.Warn("listing immich albums failed", "url", conn.URL, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Immich did not list its albums. Check the connection under Settings.")
		return
	}
	i := slices.IndexFunc(albums, func(a immich.Album) bool { return a.ID == body.AlbumID })
	if i < 0 {
		writeValidationError(w, []validation.FieldError{{Field: "albumId", Message: "choose one of the albums Immich lists"}})
		return
	}
	if err := s.Settings.SetImmichAlbum(r.Context(), albums[i]); err != nil {
		s.writeSettingsError(w, "choose immich album", err)
		return
	}
	s.auditUser(r, auditImmichAlbumChosen, albums[i].Name)
	s.SettingsChanged()
	s.writeImmich(w, r)
}

// getImmichAsset handles GET /api/immich/assets/{id}: a photo of the
// Immich album, fetched from Immich for an admin or a screen paired with a
// display that shows the album now, and only one of the photos the album
// gave. Anyone else, and any other ID, gets 404, so the server never hands
// out the rest of the library.
func (s *Server) getImmichAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	allowed, err := s.mayViewImmich(r, id)
	if err != nil {
		s.internalError(w, "check immich access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	conn, ok, err := s.Settings.ImmichConnection(r.Context())
	if err != nil || !ok {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	data, contentType, err := s.Immich.Thumbnail(r.Context(), conn, id)
	if errors.Is(err, immich.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	if err != nil {
		s.Logger.Warn("immich photo could not be fetched", "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Immich did not send this photo.")
		return
	}
	// A photo does not change under its ID, so a screen may keep it.
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(data); err != nil {
		s.Logger.Debug("screen left before the photo was sent", "error", err)
	}
}

// mayViewImmich reports whether r may have asset id: it must be one of the
// album's current photos, and r must come from an admin or from a screen
// whose display shows the album now.
func (s *Server) mayViewImmich(r *http.Request, id string) (bool, error) {
	var ids []string
	if entry, ok := s.Data()[display.ImmichSource]; !ok || json.Unmarshal(entry.Value, &ids) != nil || !slices.Contains(ids, id) {
		return false, nil
	}
	widgets, err := s.screenWidgets(r)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(widgets, display.ShowsImmich) {
		return true, nil
	}
	return s.isAdmin(r)
}
