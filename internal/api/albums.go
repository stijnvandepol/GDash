package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/stijnvandepol/gdash/internal/display"
)

// errAlbumPhotoNotFound is what a fetch of an album photo returns for a
// photo the album server does not have.
var errAlbumPhotoNotFound = errors.New("album photo not found")

// albumPhoto fetches one photo of an album from the server that holds it,
// and returns it with its content type.
type albumPhoto func(r *http.Request, id string) ([]byte, string, error)

// serveAlbumPhoto answers with photo id of the album with data key, from
// fetch, for an admin or a screen paired with a display that shows the
// album now, and only for one of the photos the album gave in its last
// refresh. Anyone else, and any other ID, gets 404, so the server never
// hands out the rest of a library. name is the album server, for the log
// and the message of a 502.
func (s *Server) serveAlbumPhoto(w http.ResponseWriter, r *http.Request, key, name string, fetch albumPhoto) {
	id := r.PathValue("id")
	allowed, err := s.mayViewAlbumPhoto(r, key, id)
	if err != nil {
		s.internalError(w, "check album access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	data, contentType, err := fetch(r, id)
	if errors.Is(err, errAlbumPhotoNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	if err != nil {
		s.Logger.Warn("album photo could not be fetched", "from", name, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", name+" did not send this photo.")
		return
	}
	// A photo does not change under its ID, so a screen may keep it.
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(data); err != nil {
		s.Logger.Debug("screen left before the photo was sent", "error", err)
	}
}

// mayViewAlbumPhoto reports whether r may have photo id of the album with
// data key: it must be one of the album's current photos, and r must come
// from an admin or from a screen whose display shows the album now.
func (s *Server) mayViewAlbumPhoto(r *http.Request, key, id string) (bool, error) {
	var ids []string
	if entry, ok := s.Data()[key]; !ok || json.Unmarshal(entry.Value, &ids) != nil || !slices.Contains(ids, id) {
		return false, nil
	}
	widgets, err := s.screenWidgets(r)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(widgets, func(w display.Widget) bool { return display.ShowsAlbum(w, key) }) {
		return true, nil
	}
	return s.isAdmin(r)
}
