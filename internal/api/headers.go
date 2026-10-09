package api

import (
	"net/http"
	"strings"
)

const (
	// permissionsPolicy switches off browser features GDash never uses, so
	// a script that got in anyway could not ask for them either.
	permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=(), serial=(), bluetooth=(), " +
		"midi=(), display-capture=(), interest-cohort=()"
	// hstsHeader is only sent on HTTPS, which GDash sees behind a trusted
	// proxy; it keeps that host on HTTPS for a year. Subdomains are left
	// alone: they may be other services of the household.
	hstsHeader = "max-age=31536000"
	// receiverCSP is what the Cast receiver page can have: the Cast
	// framework loads its code from gstatic.com at runtime, so scripts are
	// not limited, but the page cannot be framed, change its base address
	// or embed plugins.
	receiverCSP = "base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'"
)

// withSecurityHeaders sets the headers every response gets. API answers
// are never cached: they hold the household's settings, names and the
// activity log. A handler may still set its own Cache-Control, as photos do.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", permissionsPolicy)
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", hstsHeader)
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// withoutDirectoryListing answers 404 for a directory, so the file server
// only hands out files.
func withoutDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
