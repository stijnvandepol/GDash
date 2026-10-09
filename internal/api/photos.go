package api

import (
	"errors"
	"net/http"

	"github.com/stijnvandepol/nookboard/internal/pairing"
	"github.com/stijnvandepol/nookboard/internal/photos"
	"github.com/stijnvandepol/nookboard/internal/validation"
)

// uploadOverhead is room for the multipart framing around one photo.
const uploadOverhead = 64 << 10

// listPhotos handles GET /api/photos: {"photos": [...]}, oldest first.
func (s *Server) listPhotos(w http.ResponseWriter, _ *http.Request) {
	list, err := s.Photos.List()
	if err != nil {
		s.internalError(w, "list photos", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": list})
}

// postPhoto handles POST /api/photos with a multipart form whose "photo"
// field holds one JPEG, PNG or WebP file: 201 with the photo. 409 when the
// folder is full, 422 for another kind of file or one over the size limit.
func (s *Server) postPhoto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxBytes+uploadOverhead)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "Send the photo as a multipart form with a photo field.")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "photo" {
		writeError(w, http.StatusBadRequest, "invalid_upload", "Send the photo as a multipart form with a photo field.")
		return
	}
	defer part.Close()
	photo, err := s.Photos.Add(part)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, photos.ErrNotAPhoto), errors.Is(err, photos.ErrTooLarge), errors.As(err, &tooLarge):
		message := "use a JPEG, PNG or WebP photo of at most 8 MB"
		writeValidationError(w, []validation.FieldError{{Field: "photo", Message: message}})
		return
	case errors.Is(err, photos.ErrFull):
		writeError(w, http.StatusConflict, "conflict", err.Error()+".")
		return
	case err != nil:
		s.internalError(w, "add photo", err)
		return
	}
	s.auditUser(r, auditPhotoAdded, photo.Name)
	s.PhotosChanged()
	writeJSON(w, http.StatusCreated, photo)
}

// deletePhoto handles DELETE /api/photos/{name}: 204, or 404.
func (s *Server) deletePhoto(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := s.Photos.Delete(name)
	if errors.Is(err, photos.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	if err != nil {
		s.internalError(w, "delete photo", err)
		return
	}
	s.auditUser(r, auditPhotoRemoved, name)
	s.PhotosChanged()
	w.WriteHeader(http.StatusNoContent)
}

// getPhoto handles GET /api/photos/{name}: the photo, for an admin or any
// paired screen. Family photos never go to the rest of the network; an
// unknown photo and a refused request both answer 404.
func (s *Server) getPhoto(w http.ResponseWriter, r *http.Request) {
	allowed, err := s.mayViewPhotos(r)
	if err != nil {
		s.internalError(w, "check photo access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	f, contentType, err := s.Photos.Open(r.PathValue("name"))
	if errors.Is(err, photos.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This photo does not exist.")
		return
	}
	if err != nil {
		s.internalError(w, "open photo", err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		s.internalError(w, "open photo", err)
		return
	}
	// Names are random and never reused, so a screen may keep a photo.
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func (s *Server) mayViewPhotos(r *http.Request) (bool, error) {
	if _, err := s.displayFromCookie(r); err == nil {
		return true, nil
	} else if !errors.Is(err, pairing.ErrInvalidToken) {
		return false, err
	}
	return s.isAdmin(r)
}
