package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/nowplaying"
	"github.com/stijnvandepol/gdash/internal/pairing"
)

// ArtworkSource hands out the cover art of what plays on Cast devices.
type ArtworkSource interface {
	Get(ctx context.Context, id string) (nowplaying.Image, error)
}

// artworkMaxAge is how long a screen may keep a cover: the ID is that of
// one cover address, and the server keeps it for as long.
const artworkMaxAge = "private, max-age=600"

// getNowPlayingArtwork handles GET /api/now-playing/artwork/{id}: the
// cover of something that plays now, for an admin or for a screen of a
// display that was sent it. Anyone else, an unknown cover and one that
// could not be fetched all get 404.
func (s *Server) getNowPlayingArtwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	allowed, err := s.mayViewArtwork(r, id)
	if err != nil {
		s.internalError(w, "check cover access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "This cover does not exist.")
		return
	}
	img, err := s.Artwork.Get(r.Context(), id)
	if errors.Is(err, nowplaying.ErrNoArtwork) {
		writeError(w, http.StatusNotFound, "not_found", "This cover does not exist.")
		return
	}
	if err != nil {
		s.internalError(w, "get cover", err)
		return
	}
	w.Header().Set("Cache-Control", artworkMaxAge)
	w.Header().Set("Content-Type", img.ContentType)
	_, _ = w.Write(img.Data)
}

// mayViewArtwork reports whether the request may load the cover: an admin
// may, for the preview; a paired screen only when its display shows it.
func (s *Server) mayViewArtwork(r *http.Request, id string) (bool, error) {
	displayID, err := s.displayFromCookie(r)
	if err == nil {
		entry, ok := s.SentData(displayID, display.NowPlayingSource)
		return ok && nowplaying.Shows(entry.Value, id), nil
	}
	if !errors.Is(err, pairing.ErrInvalidToken) {
		return false, err
	}
	return s.isAdmin(r)
}
