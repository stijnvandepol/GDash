package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/synology"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// SynologyReader is the part of the Synology Photos client the server
// uses: trying a shared album and fetching a photo for a screen.
type SynologyReader interface {
	Check(ctx context.Context, album synology.Album) (int, error)
	Thumbnail(ctx context.Context, album synology.Album, token string) ([]byte, string, error)
}

// certificateTimeout bounds reading a NAS's certificate to accept it.
const certificateTimeout = 10 * time.Second

// handleSynology adds the routes of the Synology Photos album, for a
// logged-in admin, and of its photos, also for the screens that show them.
func (s *Server) handleSynology(mux *http.ServeMux) {
	mux.Handle("PUT /api/synology", s.requireUser(s.putSynology))
	mux.Handle("DELETE /api/synology", s.requireUser(s.deleteSynology))
	mux.HandleFunc("GET /api/synology/photos/{id}", s.getSynologyPhoto)
}

// putSynology handles PUT /api/synology {link, password, acceptCertificate}.
// The album is opened first, so a wrong link or password is reported now:
// 200 with settings.synology and {photos}, the number of photos GDash can
// show, or 422 naming the field to fix. An empty password keeps the stored
// one for the same link. With acceptCertificate, GDash reads the
// certificate the NAS presents now and from then on accepts only that one.
func (s *Server) putSynology(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Link              string `json:"link"`
		Password          string `json:"password"`
		AcceptCertificate bool   `json:"acceptCertificate"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	album, err := s.Settings.CheckSynology(r.Context(), body.Link, body.Password)
	if err != nil {
		s.writeSettingsError(w, "check synology", err)
		return
	}
	if body.AcceptCertificate {
		if !strings.HasPrefix(album.Link, "https://") {
			writeValidationError(w, []validation.FieldError{{Field: "acceptCertificate", Message: "only an https link has a certificate to accept"}})
			return
		}
		if album.Pin, err = outbound.CertificateFingerprint(r.Context(), withPort(synology.Host(album.Link)), certificateTimeout); err != nil {
			s.Logger.Warn("reading the certificate of the NAS failed", "host", synology.Host(album.Link), "error", err)
			writeValidationError(w, []validation.FieldError{{Field: "link", Message: "the NAS cannot be reached at this address from the GDash server"}})
			return
		}
	}
	count, err := s.Synology.Check(r.Context(), album)
	if err != nil {
		s.Logger.Warn("synology album test failed", "host", synology.Host(album.Link), "error", err)
		writeValidationError(w, []validation.FieldError{synologyProblem(err)})
		return
	}
	if err := s.Settings.SetSynology(r.Context(), album); err != nil {
		s.internalError(w, "store synology", err)
		return
	}
	s.auditUser(r, auditSynologyConnected, synology.Host(album.Link))
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, struct {
		*settings.Synology
		Photos int `json:"photos"`
	}{settings.SynologyView(album), count})
}

// withPort adds https's port to a host that has none.
func withPort(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), "443")
}

// synologyProblem tells the admin which field to fix.
func synologyProblem(err error) validation.FieldError {
	var verification *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	switch {
	case errors.Is(err, synology.ErrLogin):
		return validation.FieldError{Field: "password", Message: "Synology did not open the album; check the password, and that the album is still shared by link"}
	case errors.As(err, &verification), errors.As(err, &unknown), errors.As(err, &hostname), errors.Is(err, outbound.ErrCertificateChanged):
		return validation.FieldError{Field: "acceptCertificate", Message: "the NAS's certificate is not trusted; accept this NAS's certificate, or use its http address on port 5000 at home"}
	case errors.Is(err, synology.ErrNotSynology):
		return validation.FieldError{Field: "link", Message: "this address answers, but not as Synology Photos; copy the share link from Synology Photos again"}
	default:
		return validation.FieldError{Field: "link", Message: "Synology Photos cannot be reached at this address from the GDash server"}
	}
}

// deleteSynology handles DELETE /api/synology: 204.
func (s *Server) deleteSynology(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.ClearSynology(r.Context()); err != nil {
		s.internalError(w, "clear synology", err)
		return
	}
	s.auditUser(r, auditSynologyRemoved, "")
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// getSynologyPhoto handles GET /api/synology/photos/{id}: a photo of the
// shared album, fetched from the NAS; see serveAlbumPhoto for who gets it.
func (s *Server) getSynologyPhoto(w http.ResponseWriter, r *http.Request) {
	s.serveAlbumPhoto(w, r, display.SynologySource, "Synology Photos", func(r *http.Request, id string) ([]byte, string, error) {
		album, ok, err := s.Settings.SynologyAlbum(r.Context())
		if err != nil || !ok {
			return nil, "", errAlbumPhotoNotFound
		}
		data, contentType, err := s.Synology.Thumbnail(r.Context(), album, id)
		if errors.Is(err, synology.ErrNotFound) {
			return nil, "", errAlbumPhotoNotFound
		}
		return data, contentType, err
	})
}
