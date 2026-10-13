// Package webpage holds the web pages that an admin sets up under Integrations
// for the Web page widget: a name, an address and how a TV frames it. Only
// these addresses ever reach a TV's frame, and only their origins reach a
// page's Content-Security-Policy, so a dashboard can never embed an address
// of its own.
package webpage

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Limits of the pages.
const (
	// MaxPages is far more than a household frames, and keeps the list and
	// a Content-Security-Policy that names them short.
	MaxPages       = 20
	maxNameLength  = 60
	maxURLLength   = 2048
	MinZoom        = 50
	MaxZoom        = 200
	DefaultZoom    = 100
	MaxRefreshMins = 24 * 60
)

// Page is a web page as stored and as the web interface sees it.
type Page struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// RefreshMinutes reloads the page that often; 0 never does.
	RefreshMinutes int `json:"refreshMinutes"`
	// Zoom is the page's size in percent; a widget may choose its own.
	Zoom int `json:"zoom"`
	// AllowScripts lets the page run scripts in its frame, which most
	// embeds need. Without it the frame is sandboxed completely.
	AllowScripts bool      `json:"allowScripts"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Input is a page as the web interface sends it. Zoom 0 means 100 and an
// absent AllowScripts means true.
type Input struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	RefreshMinutes int    `json:"refreshMinutes"`
	Zoom           int    `json:"zoom"`
	AllowScripts   *bool  `json:"allowScripts"`
}

// hostName is what a Content-Security-Policy source can name: letters,
// digits, dashes and dots, which an IPv4 address is too. IPv6 literals and
// names with other characters cannot be listed in frame-src, so a page on
// one could never be shown.
var hostName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

const addressHint = "enter the page's full address, such as http://192.168.1.10:3000/d/abc?kiosk"

// Check validates in and returns the page it describes, without ID or
// times. ownOrigin is the origin the admin reaches GDash on, such as
// http://192.168.1.2:8420: a page there is refused, because a frame that
// may run scripts and keep its origin could reach out of its sandbox into
// GDash. It returns a *validation.Error naming every invalid field.
func Check(in Input, ownOrigin string) (Page, error) {
	var errs validation.Collector
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || validation.HasControlCharacter(name) {
		errs.Add("name", "use 1 to 60 characters")
	}
	address := strings.TrimSpace(in.URL)
	if msg := urlProblem(address, ownOrigin); msg != "" {
		errs.Add("url", msg)
	}
	if in.RefreshMinutes < 0 || in.RefreshMinutes > MaxRefreshMins {
		errs.Add("refreshMinutes", "use 0 for never, or 1 to 1440 minutes")
	}
	zoom := in.Zoom
	if zoom == 0 {
		zoom = DefaultZoom
	}
	if zoom < MinZoom || zoom > MaxZoom {
		errs.Add("zoom", "use 50 to 200 percent")
	}
	if err := errs.Err(); err != nil {
		return Page{}, err
	}
	allow := in.AllowScripts == nil || *in.AllowScripts
	return Page{Name: name, URL: address, RefreshMinutes: in.RefreshMinutes, Zoom: zoom, AllowScripts: allow}, nil
}

// urlProblem returns why address cannot be framed, or "".
func urlProblem(address, ownOrigin string) string {
	if address == "" || len(address) > maxURLLength || strings.ContainsFunc(address, isSpaceOrControl) {
		return addressHint
	}
	u, err := url.Parse(address)
	switch {
	case err != nil || u.Opaque != "":
		return addressHint
	case u.Scheme != "http" && u.Scheme != "https":
		return "use an http or https address"
	case u.User != nil:
		return "leave the user name and password out of the address; browsers refuse them in a frame"
	case u.Host == "" || !hostName.MatchString(strings.ToLower(u.Hostname())):
		return "use a host name or IPv4 address, such as grafana.local or 192.168.1.10"
	}
	origin, ok := Origin(address)
	if !ok {
		return "use a port from 1 to 65535"
	}
	if IsOwnOrigin(origin, ownOrigin) {
		return "a page of GDash itself cannot be shown in a frame; use another address or port"
	}
	return ""
}

func isSpaceOrControl(r rune) bool { return r <= ' ' || r == 0x7f }

// IsOwnOrigin reports whether origin, as Origin returns it, is GDash's own
// origin as a request reached it. An empty own origin matches nothing.
func IsOwnOrigin(origin, ownOrigin string) bool {
	if ownOrigin == "" {
		return false
	}
	own, ok := Origin(ownOrigin)
	return ok && origin == own
}

// Origin returns the origin of an http or https address as a browser
// writes it: lowercase scheme and host, and the port only when it is not
// the scheme's default. ok is false for any other address.
func Origin(address string) (string, bool) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port, ok := portNumber(u.Port())
	if !hostName.MatchString(host) || !ok {
		return "", false
	}
	if port == 0 || (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return scheme + "://" + host, true
	}
	return scheme + "://" + host + ":" + strconv.Itoa(port), true
}

// portNumber reads the port of an address, 0 for none. url.Parse has
// already made sure it holds only digits.
func portNumber(port string) (int, bool) {
	if port == "" {
		return 0, true
	}
	n, err := strconv.Atoi(port)
	return n, err == nil && n >= 1 && n <= 65535
}
