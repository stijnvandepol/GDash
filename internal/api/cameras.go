package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/camera"
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
)

// handleCameras adds the routes of cameras: the list and its changes for a
// logged-in admin, the image also for the screens that show it.
func (s *Server) handleCameras(mux *http.ServeMux) {
	mux.Handle("GET /api/cameras", s.requireUser(s.listCameras))
	mux.Handle("POST /api/cameras", s.requireUser(s.postCamera))
	mux.Handle("PUT /api/cameras/{id}", s.requireUser(s.putCamera))
	mux.Handle("DELETE /api/cameras/{id}", s.requireUser(s.deleteCamera))
	mux.Handle("POST /api/cameras/{id}/tests", s.requireUser(s.postCameraTest))
	mux.HandleFunc("GET /api/cameras/{id}/image", s.getCameraImage)
}

// listCameras handles GET /api/cameras: {"cameras": [...], "unreadable"},
// without secrets; each says whether its secret is set.
func (s *Server) listCameras(w http.ResponseWriter, r *http.Request) {
	list, err := s.Settings.CameraList(r.Context())
	if err != nil {
		s.internalError(w, "list cameras", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// postCamera handles POST /api/cameras {name, url, auth}: 201 with the
// camera, 422 naming every invalid field, 409 at the limit.
func (s *Server) postCamera(w http.ResponseWriter, r *http.Request) {
	var in camera.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	cam, err := s.Settings.AddCamera(r.Context(), in)
	if err != nil {
		s.writeCameraError(w, "add camera", err)
		return
	}
	s.auditUser(r, auditCameraAdded, describeCamera(cam))
	writeJSON(w, http.StatusCreated, cam)
}

// putCamera handles PUT /api/cameras/{id}: a secret left empty keeps the
// stored one. 200 with the camera, 404, or 422.
func (s *Server) putCamera(w http.ResponseWriter, r *http.Request) {
	id, ok := cameraID(w, r)
	if !ok {
		return
	}
	var in camera.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	cam, err := s.Settings.UpdateCamera(r.Context(), id, in)
	if err != nil {
		s.writeCameraError(w, "update camera", err)
		return
	}
	s.Cameras.Forget(id)
	s.auditUser(r, auditCameraChanged, describeCamera(cam))
	writeJSON(w, http.StatusOK, cam)
}

// deleteCamera handles DELETE /api/cameras/{id}: 204 or 404. Widgets that
// show it then say that their camera is not available.
func (s *Server) deleteCamera(w http.ResponseWriter, r *http.Request) {
	id, ok := cameraID(w, r)
	if !ok {
		return
	}
	if err := s.Settings.DeleteCamera(r.Context(), id); err != nil {
		s.writeCameraError(w, "delete camera", err)
		return
	}
	s.Cameras.Forget(id)
	s.auditUser(r, auditCameraRemoved, "id "+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusNoContent)
}

// postCameraTest handles POST /api/cameras/{id}/tests: asks the camera for
// one image, past what is kept, and returns {contentType, bytes}. 502 with
// what went wrong; the message names the host but never a secret.
func (s *Server) postCameraTest(w http.ResponseWriter, r *http.Request) {
	id, ok := cameraID(w, r)
	if !ok {
		return
	}
	cam, err := s.Settings.Camera(r.Context(), id)
	if err != nil {
		s.writeCameraError(w, "read camera", err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), camera.Timeout)
	defer cancel()
	data, contentType, err := s.Cameras.Fetcher.Fetch(ctx, cam)
	if err != nil {
		s.Logger.Warn("camera test failed", "camera", cam.Name, "error", err)
		writeError(w, http.StatusBadGateway, "fetch_failed", cameraFailure(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contentType": contentType, "bytes": len(data)})
}

// getCameraImage handles GET /api/cameras/{id}/image: the camera's latest
// image, for an admin or a screen paired with a display that shows the
// camera now. Anyone else, and an unknown camera, gets 404. The image is
// never cached: Last-Modified says when GDash received it, and
// X-Image-Stale: true that the camera failed since. 502 when the camera
// has not sent an image yet.
func (s *Server) getCameraImage(w http.ResponseWriter, r *http.Request) {
	id, ok := cameraID(w, r)
	if !ok {
		return
	}
	every, allowed, err := s.cameraAccess(r, id)
	if err != nil {
		s.internalError(w, "check camera access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "This camera does not exist.")
		return
	}
	cam, err := s.Settings.Camera(r.Context(), id)
	if err != nil {
		s.writeCameraError(w, "read camera", err)
		return
	}
	image, err := s.Cameras.Latest(r.Context(), cam, every)
	if err != nil {
		writeError(w, http.StatusBadGateway, "camera_unavailable", "The camera has not sent an image. Test it under Settings.")
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Type", image.ContentType)
	h.Set("Last-Modified", image.TakenAt.UTC().Format(http.TimeFormat))
	if image.Stale {
		h.Set("X-Image-Stale", "true")
	}
	if _, err := w.Write(image.Data); err != nil {
		s.Logger.Debug("screen left before the camera image was sent", "error", err)
	}
}

// cameraAccess reports whether r may have the image of camera id, and how
// old the kept image may be: for a screen, the shortest refresh of the
// Camera widgets its display shows now; for an admin, who previews any
// dashboard, the shortest refresh there is.
func (s *Server) cameraAccess(r *http.Request, id int64) (time.Duration, bool, error) {
	widgets, err := s.screenWidgets(r)
	if err != nil {
		return 0, false, err
	}
	if every, ok := display.CameraRefresh(widgets, id); ok {
		return every, true, nil
	}
	admin, err := s.isAdmin(r)
	return display.MinCameraSeconds * time.Second, admin, err
}

// screenWidgets returns the widgets that the display of the screen making
// r shows now, or none when r comes from no paired screen.
func (s *Server) screenWidgets(r *http.Request) ([]display.Widget, error) {
	displayID, err := s.displayFromCookie(r)
	if errors.Is(err, pairing.ErrInvalidToken) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	views, err := s.Store.DisplayViews(r.Context())
	if err != nil {
		return nil, err
	}
	return views[displayID].WidgetsAt(s.Now()), nil
}

// cameraFailure is what an admin is told about a failed request: the
// camera's own words when they are safe to show.
func cameraFailure(err error) string {
	var fetchErr *custom.FetchError
	if errors.As(err, &fetchErr) {
		return "The camera could not be fetched: " + fetchErr.Message + "."
	}
	return "The camera could not be fetched. Check the GDash log."
}

func (s *Server) writeCameraError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This camera does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "The limit of cameras is reached. Remove a camera first.")
	default:
		s.writeSettingsError(w, op, err)
	}
}

func cameraID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "This camera does not exist.")
		return 0, false
	}
	return id, true
}

// describeCamera is the audit detail: the name and host, never the path or
// query, which could hold a key.
func describeCamera(cam settings.Camera) string {
	host := ""
	if u, err := url.Parse(cam.URL); err == nil {
		host = u.Host
	}
	return cam.Name + " (" + host + ")"
}
