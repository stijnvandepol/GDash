// Package api serves the display pages, the Cast receiver page and the HTTP
// API. Every API route that reads configuration or changes anything needs a
// logged-in user. A display's event stream needs a screen paired with that
// display, or a logged-in user previewing it.
package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"regexp"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/apitoken"
	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/casting"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/photos"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// Proxies close idle connections after 30 to 60 seconds; a comment line
	// every 20 seconds keeps the event stream open through them.
	defaultPingInterval = 20 * time.Second
	// streamWriteTimeout ends an event stream whose screen stopped reading,
	// so a stalled client does not hold a goroutine and a connection.
	streamWriteTimeout = 30 * time.Second
	sseRetryMillis     = 5000
	discoveryTimeout   = 3 * time.Second
)

var validDisplayID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Subscriber hands out a display's messages, newest first.
type Subscriber interface {
	Subscribe(ctx context.Context, displayID string) <-chan []byte
}

// Server wires the HTTP routes.
type Server struct {
	Snapshots Subscriber
	Auth      *auth.Service
	Pairing   *pairing.Service
	APITokens *apitoken.Service
	Store     *store.Store
	// Web holds the built frontend: display/index.html, receiver/index.html
	// and assets/.
	Web      fs.FS
	Discover func(ctx context.Context) ([]cast.Device, error)
	// DisplaysChanged is called after a display is created, changed or
	// deleted, so screens and the cast manager can follow at once.
	DisplaysChanged func()
	// DashboardsChanged is called after a dashboard is created, changed or
	// deleted, so screens showing it update at once.
	DashboardsChanged func()
	Settings          *settings.Service
	// Announcements holds the messages that automations put on the
	// displays. AnnouncementsChanged is called after one is posted, with
	// the time it expires, so the screens receive it and lose it on time.
	Announcements        *announcement.Board
	AnnouncementsChanged func(expiresAt time.Time)
	Places               PlaceSearcher
	HomeAssistant        HomeAssistantReader
	Stops                StopSearcher
	// SettingsChanged is called after a data source setting changes, so
	// the data hub fetches what is configured now.
	SettingsChanged func()
	// SourceStatus returns the data hub's entries, for the Settings page.
	SourceStatus func() map[string]datahub.Entry
	// Data returns the data displays are sent, for the dashboard preview:
	// the hub's entries and the photo list.
	Data func() map[string]datahub.Entry
	// Version is the running release, shown in the web interface.
	Version string
	// CastStatus returns what each display's cast keeper last found.
	CastStatus func() map[string]casting.Status
	Photos     *photos.Store
	// Backup writes the zip that POST /api/backup downloads.
	Backup BackupWriter
	// PhotosChanged is called after a photo is added or removed, so photo
	// widgets update.
	PhotosChanged func()
	// PingInterval is how often an open display stream sends a keep-alive
	// and re-checks that it may still show the display. Zero means 20
	// seconds.
	PingInterval time.Duration
	// TrustedProxies are the reverse proxies whose X-Forwarded-For and
	// X-Forwarded-Proto headers count; see behindProxies.
	TrustedProxies []netip.Prefix
	Now            func() time.Time
	Logger         *slog.Logger
}

// Handler returns the routes of the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /{$}", s.adminPage)
	mux.HandleFunc("GET /d/{display}", s.displayPage)
	mux.HandleFunc("GET /preview/{dashboard}", s.previewPage)
	mux.HandleFunc("GET /receiver/", s.receiverPage)
	mux.Handle("GET /assets/", withoutDirectoryListing(http.FileServerFS(s.Web)))
	mux.HandleFunc("GET /api/displays/{display}/events", s.displayEvents)

	mux.HandleFunc("GET /api/setup", s.getSetup)
	mux.HandleFunc("POST /api/setup", s.postSetup)
	mux.HandleFunc("POST /api/sessions", s.postSession)
	mux.Handle("DELETE /api/sessions/current", s.requireUser(s.deleteSession))
	mux.Handle("GET /api/me", s.requireUser(s.getMe))
	mux.Handle("PUT /api/me/password", s.requireUser(s.putPassword))
	mux.Handle("POST /api/backup", s.requireUser(s.postBackup))
	mux.Handle("GET /api/system", s.requireUser(s.getSystem))
	mux.Handle("GET /api/audit-events", s.requireUser(s.getAuditEvents))
	mux.Handle("POST /api/announcements", s.requireUserOrToken(s.postAnnouncement))
	mux.Handle("GET /api/api-tokens", s.requireUser(s.listAPITokens))
	mux.Handle("POST /api/api-tokens", s.requireUser(s.postAPIToken))
	mux.Handle("DELETE /api/api-tokens/{id}", s.requireUser(s.deleteAPIToken))

	mux.HandleFunc("POST /api/pairing-requests", s.postPairingRequest)
	mux.HandleFunc("POST /api/display-sessions", s.postDisplaySession)
	mux.HandleFunc("GET /api/display-sessions/current", s.getDisplaySession)
	mux.Handle("POST /api/displays/{display}/pairings", s.requireUser(s.postPairing))
	mux.Handle("DELETE /api/displays/{display}/pairings", s.requireUser(s.deletePairings))

	mux.Handle("GET /api/widget-types", s.requireUser(s.getWidgetTypes))
	mux.Handle("GET /api/dashboard-templates", s.requireUser(s.getDashboardTemplates))
	mux.Handle("GET /api/dashboards", s.requireUser(s.listDashboards))
	mux.Handle("POST /api/dashboards", s.requireUser(s.createDashboard))
	mux.Handle("GET /api/dashboards/{dashboard}", s.requireUser(s.getDashboard))
	mux.Handle("PUT /api/dashboards/{dashboard}", s.requireUser(s.updateDashboard))
	mux.Handle("DELETE /api/dashboards/{dashboard}", s.requireUser(s.deleteDashboard))
	mux.Handle("GET /api/dashboards/{dashboard}/preview", s.requireUser(s.getDashboardPreview))
	mux.Handle("POST /api/dashboards/{dashboard}/preview", s.requireUser(s.postDashboardPreview))

	mux.Handle("GET /api/settings", s.requireUser(s.getSettings))
	mux.Handle("PUT /api/settings/location", s.requireUser(s.putLocation))
	mux.Handle("DELETE /api/settings/location", s.requireUser(s.deleteLocation))
	mux.Handle("PUT /api/settings/language", s.requireUser(s.putLanguage))
	mux.Handle("POST /api/news-feeds", s.requireUser(s.postNewsFeed))
	mux.Handle("DELETE /api/news-feeds/{id}", s.requireUser(s.deleteNewsFeed))
	mux.Handle("POST /api/calendars", s.requireUser(s.postCalendar))
	mux.Handle("PATCH /api/calendars/{id}", s.requireUser(s.patchCalendar))
	mux.Handle("DELETE /api/calendars/{id}", s.requireUser(s.deleteCalendar))
	mux.Handle("GET /api/places", s.requireUser(s.getPlaces))
	mux.Handle("PUT /api/homeassistant", s.requireUser(s.putHomeAssistant))
	mux.Handle("DELETE /api/homeassistant", s.requireUser(s.deleteHomeAssistant))
	mux.Handle("GET /api/homeassistant/entities", s.requireUser(s.getHomeAssistantEntities))
	mux.Handle("GET /api/departures/stops", s.requireUser(s.getDepartureStops))

	mux.Handle("GET /api/photos", s.requireUser(s.listPhotos))
	mux.Handle("POST /api/photos", s.requireUser(s.postPhoto))
	mux.Handle("DELETE /api/photos/{name}", s.requireUser(s.deletePhoto))
	mux.HandleFunc("GET /api/photos/{name}", s.getPhoto)

	mux.Handle("GET /api/cast-devices", s.requireUser(s.castDevices))
	mux.Handle("GET /api/displays", s.requireUser(s.listDisplays))
	mux.Handle("POST /api/displays", s.requireUser(s.createDisplay))
	mux.Handle("PATCH /api/displays/{display}", s.requireUser(s.updateDisplay))
	mux.Handle("DELETE /api/displays/{display}", s.requireUser(s.deleteDisplay))

	// Rejects state-changing requests that a browser sends from another
	// site (CSRF), based on Sec-Fetch-Site and Origin.
	var csrf http.CrossOriginProtection
	return behindProxies(s.TrustedProxies, withSecurityHeaders(csrf.Handler(mux)))
}

// displayCSP (also used for the web interface) allows nothing from outside the server: no CDN, no fonts from
// Google, no inline scripts.
const (
	pageCSP    = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'"
	displayCSP = pageCSP + "; frame-ancestors 'none'"
	// previewCSP is the same, except that the web interface may show the
	// preview in a frame.
	previewCSP = pageCSP + "; frame-ancestors 'self'"
)

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
// docs/receiver.md. Its CSP cannot limit scripts, because the Cast
// framework loads code from gstatic.com at runtime; see receiverCSP.
func (s *Server) receiverPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", receiverCSP)
	s.servePage(w, r, "receiver/index.html")
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, name string) {
	page, err := fs.ReadFile(s.Web, name)
	if err != nil {
		s.Logger.Error("frontend page missing; was the frontend built?", "page", name, "error", err)
		writeError(w, http.StatusInternalServerError, "frontend_missing", "The display page is not available. Build the frontend and restart GDash.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(page); err != nil {
		s.Logger.Debug("client left before the page was sent", "page", name, "error", err)
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
		writeError(w, http.StatusInternalServerError, "discovery_failed", "Searching the network for Cast devices failed. Check that GDash runs with host networking.")
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
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Send the request body as application/json.")
		return false
	}
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
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong on the server. Check the GDash log.")
}

// writeJSON sends body as JSON. The status line is already sent when the
// body is written, so an encode or write error can only mean the client
// went away; there is no response left to correct and nothing to log.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
