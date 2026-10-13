package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

// WebPageChecker fetches a web page once, for the Check button.
type WebPageChecker interface {
	Check(ctx context.Context, address, ownOrigin string) (webpage.Framing, error)
}

// handleWebPages adds the routes of the web pages for the Web page widget,
// all for a logged-in admin only.
func (s *Server) handleWebPages(mux *http.ServeMux) {
	mux.Handle("GET /api/web-pages", s.requireUser(s.listWebPages))
	mux.Handle("POST /api/web-pages", s.requireUser(s.postWebPage))
	mux.Handle("PUT /api/web-pages/{id}", s.requireUser(s.putWebPage))
	mux.Handle("DELETE /api/web-pages/{id}", s.requireUser(s.deleteWebPage))
	mux.Handle("POST /api/web-pages/{id}/checks", s.requireUser(s.postWebPageCheck))
}

// listWebPages handles GET /api/web-pages: {"pages": [...]}.
func (s *Server) listWebPages(w http.ResponseWriter, r *http.Request) {
	pages, err := s.Settings.WebPages(r.Context())
	if err != nil {
		s.internalError(w, "list web pages", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

// postWebPage handles POST /api/web-pages with {name, url, refreshMinutes,
// zoom, allowScripts}: 201 with the page, 422 naming every invalid field
// (also for an address on GDash's own origin), 409 at the limit.
func (s *Server) postWebPage(w http.ResponseWriter, r *http.Request) {
	var in webpage.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	page, err := s.Settings.AddWebPage(r.Context(), in, ownOrigin(r))
	if err != nil {
		s.writeWebPageError(w, "add web page", err)
		return
	}
	s.auditUser(r, auditWebPageAdded, describeWebPage(page))
	s.SettingsChanged()
	writeJSON(w, http.StatusCreated, page)
}

// putWebPage handles PUT /api/web-pages/{id}: 200 with the page, 404 or 422.
func (s *Server) putWebPage(w http.ResponseWriter, r *http.Request) {
	id, ok := webPageID(w, r)
	if !ok {
		return
	}
	var in webpage.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	page, err := s.Settings.UpdateWebPage(r.Context(), id, in, ownOrigin(r))
	if err != nil {
		s.writeWebPageError(w, "update web page", err)
		return
	}
	s.auditUser(r, auditWebPageChanged, describeWebPage(page))
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, page)
}

// deleteWebPage handles DELETE /api/web-pages/{id}: 204 or 404. Widgets
// that show it then say that the page is not available.
func (s *Server) deleteWebPage(w http.ResponseWriter, r *http.Request) {
	id, ok := webPageID(w, r)
	if !ok {
		return
	}
	if err := s.Settings.DeleteWebPage(r.Context(), id); err != nil {
		s.writeWebPageError(w, "delete web page", err)
		return
	}
	s.auditUser(r, auditWebPageRemoved, "id "+strconv.FormatInt(id, 10))
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// postWebPageCheck handles POST /api/web-pages/{id}/checks: fetches the
// page once through the outbound client and returns {status, blocked,
// header}, where blocked names the header that forbids GDash to frame it:
// "x-frame-options" or "frame-ancestors". 404, or 502 when the page could
// not be reached; the message names the host, never the path or query.
func (s *Server) postWebPageCheck(w http.ResponseWriter, r *http.Request) {
	id, ok := webPageID(w, r)
	if !ok {
		return
	}
	page, err := s.Store.WebPage(r.Context(), id)
	if err != nil {
		s.writeWebPageError(w, "read web page", err)
		return
	}
	framing, err := s.WebPageChecker.Check(r.Context(), page.URL, ownOrigin(r))
	if err != nil {
		s.Logger.Warn("web page check failed", "page", page.Name, "error", err)
		message := "The page could not be fetched. Check the GDash log."
		var checkErr *webpage.CheckError
		if errors.As(err, &checkErr) {
			message = "The page could not be fetched: " + checkErr.Message + "."
		}
		writeError(w, http.StatusBadGateway, "fetch_failed", message)
		return
	}
	writeJSON(w, http.StatusOK, framing)
}

// writeWebPageError words the store's errors for a web page; anything else
// is answered as for the other settings.
func (s *Server) writeWebPageError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "This web page does not exist.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", fmt.Sprintf("The limit of %d web pages is reached. Remove one first.", webpage.MaxPages))
	default:
		s.writeSettingsError(w, op, err)
	}
}

func webPageID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "This web page does not exist.")
		return 0, false
	}
	return id, true
}

// describeWebPage is the audit detail: the name and host, never the path
// or query, which could hold a token.
func describeWebPage(page webpage.Page) string {
	host := ""
	if u, err := url.Parse(page.URL); err == nil {
		host = u.Host
	}
	return page.Name + " (" + host + ")"
}

// ownOrigin is GDash's origin as this request reached it, such as
// http://192.168.1.2:8420.
func ownOrigin(r *http.Request) string {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// withFrameSrc adds a frame-src directive to a page's CSP: exactly the
// origins given, or 'none'. Origins come from webpage.Origin, which only
// lets letters, digits, dots, dashes and a port through, so none can add a
// directive of its own.
func withFrameSrc(csp string, origins []string) string {
	if len(origins) == 0 {
		return csp + "; frame-src 'none'"
	}
	return csp + "; frame-src " + strings.Join(origins, " ")
}

// displayFrameOrigins returns the origins that the display page of
// displayID may frame: those of the web pages on every dashboard the
// display shows, its schedule and rotation included. A request that may
// not show the display gets none, so the page does not tell an unpaired
// device which pages the display has.
func (s *Server) displayFrameOrigins(r *http.Request, displayID string) ([]string, error) {
	allowed, err := s.mayShowDisplay(r, displayID)
	if err != nil || !allowed {
		return nil, err
	}
	views, err := s.Store.DisplayViews(r.Context())
	if err != nil {
		return nil, err
	}
	view, ok := views[displayID]
	if !ok {
		return nil, nil
	}
	pages, err := s.Store.WebPages(r.Context())
	if err != nil {
		return nil, err
	}
	return display.FrameOrigins(view.Dashboards(), pages, ownOrigin(r)), nil
}

// previewFrameOrigins returns the origins the preview page may frame: an
// unsaved layout in the editor can show any web page under Settings, so
// all of them, for an admin only.
func (s *Server) previewFrameOrigins(r *http.Request) ([]string, error) {
	admin, err := s.isAdmin(r)
	if err != nil || !admin {
		return nil, err
	}
	pages, err := s.Store.WebPages(r.Context())
	if err != nil {
		return nil, err
	}
	return display.PageOrigins(pages, ownOrigin(r)), nil
}
