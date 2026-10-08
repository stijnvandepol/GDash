// Package api serves the display pages, the Cast receiver page and the HTTP
// API. Every API route that reads configuration or changes anything needs a
// logged-in user. A display's event stream needs a screen paired with that
// display, or a logged-in user previewing it.
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

	"github.com/stijnvandepol/daycast/internal/auth"
	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/pairing"
	"github.com/stijnvandepol/daycast/internal/store"
	"github.com/stijnvandepol/daycast/internal/validation"
)

const (
	// Proxies close idle connections after 30 to 60 seconds; a comment line
	// every 20 seconds keeps the event stream open through them.
	defaultPingInterval = 20 * time.Second
	sseRetryMillis      = 5000
	discoveryTimeout    = 3 * time.Second
)

var validDisplayID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Subscriber hands out display messages, newest first.
type Subscriber interface {
	Subscribe(ctx context.Context) <-chan []byte
}

// Server wires the HTTP routes.
type Server struct {
	Snapshots Subscriber
	Auth      *auth.Service
	Pairing   *pairing.Service
	Store     *store.Store
	// Web holds the built frontend: display/index.html, receiver/index.html
	// and assets/.
	Web      fs.FS
	Discover func(ctx context.Context) ([]cast.Device, error)
	// DisplaysChanged is called after a display is created, changed or
	// deleted, so the cast manager can follow at once.
	DisplaysChanged func()
	// PingInterval is how often an open display stream sends a keep-alive
	// and re-checks that it may still show the display. Zero means 20
	// seconds.
	PingInterval time.Duration
	Now          func() time.Time
	Logger       *slog.Logger
}

// Handler returns the routes of the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /{$}", s.adminPage)
	mux.HandleFunc("GET /d/{display}", s.displayPage)
	mux.HandleFunc("GET /receiver/", s.receiverPage)
	mux.Handle("GET /assets/", http.FileServerFS(s.Web))
	mux.HandleFunc("GET /api/displays/{display}/events", s.displayEvents)

	mux.HandleFunc("GET /api/setup", s.getSetup)
	mux.HandleFunc("POST /api/setup", s.postSetup)
	mux.HandleFunc("POST /api/sessions", s.postSession)
	mux.Handle("DELETE /api/sessions/current", s.requireUser(s.deleteSession))
	mux.Handle("GET /api/me", s.requireUser(s.getMe))

	mux.HandleFunc("POST /api/pairing-requests", s.postPairingRequest)
	mux.HandleFunc("POST /api/display-sessions", s.postDisplaySession)
	mux.HandleFunc("GET /api/display-sessions/current", s.getDisplaySession)
	mux.Handle("POST /api/displays/{display}/pairings", s.requireUser(s.postPairing))
	mux.Handle("DELETE /api/displays/{display}/pairings", s.requireUser(s.deletePairings))

	mux.Handle("GET /api/cast-devices", s.requireUser(s.castDevices))
	mux.Handle("GET /api/displays", s.requireUser(s.listDisplays))
	mux.Handle("POST /api/displays", s.requireUser(s.createDisplay))
	mux.Handle("PATCH /api/displays/{display}", s.requireUser(s.updateDisplay))
	mux.Handle("DELETE /api/displays/{display}", s.requireUser(s.deleteDisplay))

	// Rejects state-changing requests that a browser sends from another
	// site (CSRF), based on Sec-Fetch-Site and Origin.
	var csrf http.CrossOriginProtection
	return withSecurityHeaders(csrf.Handler(mux))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// displayCSP (also used for the web interface) allows nothing from outside the server: no CDN, no fonts from
// Google, no inline scripts.
const displayCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// adminPage is the web interface. It loads without a session; the API it
// calls requires one.
func (s *Server) adminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", displayCSP)
	s.servePage(w, r, "admin/index.html")
}

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
	if _, err := w.Write(page); err != nil {
		s.Logger.Debug("client left before the page was sent", "page", name, "error", err)
	}
}

// displayEvents handles GET /api/displays/{display}/events: a Server-Sent
// Events stream with a "snapshot" event for every change of the display.
func (s *Server) displayEvents(w http.ResponseWriter, r *http.Request) {
	displayID := r.PathValue("display")
	if !validDisplayID.MatchString(displayID) {
		http.NotFound(w, r)
		return
	}
	allowed, err := s.mayShowDisplay(r, displayID)
	if err != nil {
		s.internalError(w, "authorize display stream", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusUnauthorized, "not_paired", "Pair this screen with the display first.")
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

	messages := s.Snapshots.Subscribe(r.Context())
	interval := s.PingInterval
	if interval == 0 {
		interval = defaultPingInterval
	}
	ping := time.NewTicker(interval)
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
			// A screen that was unpaired, or an admin whose session ended,
			// must stop receiving data now, not at its next reconnect.
			allowed, authErr := s.mayShowDisplay(r, displayID)
			if authErr != nil {
				s.Logger.Error("could not re-check display stream", "display", displayID, "error", authErr)
				return
			}
			if !allowed {
				return
			}
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

// castDevices handles GET /api/cast-devices: {"devices": [...]}, the Cast
// devices that answered a 3-second mDNS query.
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
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Fields  []validation.FieldError `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message}})
}

// writeValidationError reports every invalid field of a request at once.
func writeValidationError(w http.ResponseWriter, fields []validation.FieldError) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]apiError{"error": {
		Code: "invalid_input", Message: "Some fields are not valid.", Fields: fields,
	}})
}

// maxRequestBody is far above any valid request; it stops a client from
// making the server read an endless body.
const maxRequestBody = 64 << 10

// decodeJSON reads one JSON object into dst and rejects unknown fields, so a
// typo in a field name is an error instead of being silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is not valid JSON for this endpoint.")
		return false
	}
	if dec.More() {
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body must contain one JSON object.")
		return false
	}
	return true
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	s.Logger.Error("request failed", "operation", op, "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong on the server. Check the Daycast log.")
}

// writeJSON sends body as JSON. The status line is already sent when the
// body is written, so an encode or write error can only mean the client
// went away; there is no response left to correct and nothing to log.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
