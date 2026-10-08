// Package api serves the display pages, the Cast receiver page and the HTTP
// API. The prototype has no authentication yet; it must only run on a
// trusted network until v0.5 adds accounts and display pairing.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/stijnvandepol/daycast/internal/cast"
)

const (
	// Proxies close idle connections after 30 to 60 seconds; a comment line
	// every 20 seconds keeps the event stream open through them.
	ssePingInterval  = 20 * time.Second
	sseRetryMillis   = 5000
	discoveryTimeout = 3 * time.Second
)

var validDisplayID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Subscriber hands out display messages, newest first.
type Subscriber interface {
	Subscribe(ctx context.Context) <-chan []byte
}

// Server wires the HTTP routes.
type Server struct {
	Displays Subscriber
	// Web holds the built frontend: display/index.html, receiver/index.html
	// and assets/.
	Web      fs.FS
	Discover func(ctx context.Context) ([]cast.Device, error)
	Logger   *slog.Logger
}

// Handler returns the routes of the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /d/{display}", s.displayPage)
	mux.HandleFunc("GET /receiver/", s.receiverPage)
	mux.HandleFunc("GET /api/displays/{display}/events", s.displayEvents)
	mux.HandleFunc("GET /api/cast-devices", s.castDevices)
	mux.Handle("GET /assets/", http.FileServerFS(s.Web))
	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// displayCSP allows nothing from outside the server: no CDN, no fonts from
// Google, no inline scripts.
const displayCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func (s *Server) displayPage(w http.ResponseWriter, r *http.Request) {
	if !validDisplayID.MatchString(r.PathValue("display")) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Security-Policy", displayCSP)
	s.servePage(w, r, "display/index.html")
}

// receiverPage is served for development: an unpublished Cast app may point
// at a LAN address. A published receiver must come from HTTPS, see
// docs/receiver.md. No CSP is set because the Cast framework loads code from
// gstatic.com at runtime.
func (s *Server) receiverPage(w http.ResponseWriter, r *http.Request) {
	s.servePage(w, r, "receiver/index.html")
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, name string) {
	page, err := fs.ReadFile(s.Web, name)
	if err != nil {
		s.Logger.Error("frontend page missing; was the frontend built?", "page", name, "error", err)
		writeError(w, http.StatusInternalServerError, "frontend_missing", "The display page is not available. Build the frontend and restart Daycast.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(page)
}

func (s *Server) displayEvents(w http.ResponseWriter, r *http.Request) {
	if !validDisplayID.MatchString(r.PathValue("display")) {
		http.NotFound(w, r)
		return
	}
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// nginx buffers responses by default, which holds events back.
	h.Set("X-Accel-Buffering", "no")

	if _, err := fmt.Fprintf(w, "retry: %d\n\n", sseRetryMillis); err != nil {
		return
	}
	if err := rc.Flush(); err != nil {
		s.Logger.Error("event stream cannot flush", "error", err)
		return
	}

	messages := s.Displays.Subscribe(r.Context())
	ping := time.NewTicker(ssePingInterval)
	defer ping.Stop()
	for {
		var err error
		select {
		case msg, ok := <-messages:
			if !ok {
				return
			}
			// Snapshots are compact JSON without newlines, so one data
			// line is a complete event.
			_, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", msg)
		case <-ping.C:
			_, err = fmt.Fprint(w, ": ping\n\n")
		}
		if err == nil {
			err = rc.Flush()
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) castDevices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	devices, err := s.Discover(ctx)
	if err != nil {
		s.Logger.Error("cast discovery failed", "error", err)
		writeError(w, http.StatusInternalServerError, "discovery_failed", "Searching the network for Cast devices failed. Check that Daycast runs with host networking.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
