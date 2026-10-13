package webpage

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// What forbids a page to be framed, in Framing.Blocked.
const (
	BlockedByFrameOptions   = "x-frame-options"
	BlockedByFrameAncestors = "frame-ancestors"
)

// maxShownHeader keeps a header that is shown in the web interface short.
const maxShownHeader = 300

// Framing is what a page's answer says about being shown in a frame on
// GDash's pages.
type Framing struct {
	// Status is the HTTP status the page answered with, after redirects.
	Status int `json:"status"`
	// Blocked is empty when the browser may frame the page, or the header
	// that forbids it: BlockedByFrameOptions or BlockedByFrameAncestors.
	Blocked string `json:"blocked,omitempty"`
	// Header is the value of that header, shortened.
	Header string `json:"header,omitempty"`
}

// CheckError is a page that could not be fetched. Its message names the
// host but never the path or query, which may hold a token.
type CheckError struct {
	Message string
	cause   error
}

func (e *CheckError) Error() string { return e.Message + ": " + e.cause.Error() }

func (e *CheckError) Unwrap() error { return e.cause }

// Checker fetches a page to see whether it may be framed.
type Checker struct {
	// HTTP must refuse the addresses that internal/outbound refuses and
	// have a timeout.
	HTTP      *http.Client
	UserAgent string
}

// Check requests address once, following redirects as a frame would, and
// reads the headers that decide whether ownOrigin may frame it. The body
// is never read.
func (c *Checker) Check(ctx context.Context, address, ownOrigin string) (Framing, error) {
	pageOrigin, ok := Origin(address)
	if !ok {
		return Framing{}, &CheckError{Message: "the address is not valid", cause: errors.New("no origin")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return Framing{}, &CheckError{Message: "the address is not valid", cause: err}
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			// url.Error repeats the whole address, query and all.
			err = urlErr.Err
		}
		return Framing{}, &CheckError{Message: req.URL.Host + " could not be reached", cause: err}
	}
	defer resp.Body.Close()
	framing := Evaluate(resp.Header, pageOrigin, ownOrigin)
	framing.Status = resp.StatusCode
	return framing, nil
}

// Evaluate decides from a page's headers whether a browser frames it on a
// page of ownOrigin, as the HTML and CSP specifications say: a
// frame-ancestors directive in any enforced policy wins and
// X-Frame-Options is then ignored. Paths in frame-ancestors sources are
// not compared; GDash's pages all live on its origin.
func Evaluate(header http.Header, pageOrigin, ownOrigin string) Framing {
	own, err := url.Parse(ownOrigin)
	if err != nil || own.Host == "" {
		return Framing{Blocked: BlockedByFrameAncestors}
	}
	page, _ := url.Parse(pageOrigin)
	policies := header.Values("Content-Security-Policy")
	found := false
	for _, value := range policies {
		for _, policy := range strings.Split(value, ",") {
			sources, ok := frameAncestors(policy)
			if !ok {
				continue
			}
			found = true
			if !anySourceMatches(sources, page, own) {
				return Framing{Blocked: BlockedByFrameAncestors, Header: shorten(strings.Join(policies, ", "))}
			}
		}
	}
	if found {
		return Framing{}
	}
	if frameOptionsForbid(header.Values("X-Frame-Options")) {
		return Framing{Blocked: BlockedByFrameOptions, Header: shorten(strings.Join(header.Values("X-Frame-Options"), ", "))}
	}
	return Framing{}
}

// frameAncestors returns the sources of a policy's frame-ancestors
// directive; ok is false when it has none. Only the first directive of a
// name counts.
func frameAncestors(policy string) ([]string, bool) {
	for _, directive := range strings.Split(policy, ";") {
		fields := strings.Fields(directive)
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], "frame-ancestors") {
			return fields[1:], true
		}
	}
	return nil, false
}

// frameOptionsForbid applies X-Frame-Options to a frame on another origin:
// DENY and SAMEORIGIN forbid it, as do conflicting values; ALLOW-FROM and
// unknown values are ignored by browsers.
func frameOptionsForbid(values []string) bool {
	set := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
				set[part] = true
			}
		}
	}
	if len(set) > 1 {
		return set["deny"] || set["sameorigin"] || set["allowall"]
	}
	return set["deny"] || set["sameorigin"]
}

func anySourceMatches(sources []string, page, own *url.URL) bool {
	for _, source := range sources {
		if sourceMatches(strings.ToLower(source), page, own) {
			return true
		}
	}
	return false
}

// hostSource is a CSP host-source: an optional scheme, a host that may
// start with a wildcard, an optional port or * and an optional path.
var hostSource = regexp.MustCompile(`^(?:([a-z][a-z0-9+.-]*)://)?(\*|(?:\*\.)?[a-z0-9.-]+)(?::(\*|[0-9]+))?(/.*)?$`)

func sourceMatches(source string, page, own *url.URL) bool {
	switch {
	case source == "*":
		return own.Scheme == "http" || own.Scheme == "https"
	case source == "'self'":
		return page != nil && strings.EqualFold(page.Scheme+"://"+page.Host, own.Scheme+"://"+own.Host)
	case strings.HasPrefix(source, "'"):
		// 'none' and keywords that frame-ancestors ignores.
		return false
	case strings.HasSuffix(source, ":") && !strings.Contains(source, "/"):
		return schemeMatches(strings.TrimSuffix(source, ":"), own.Scheme)
	}
	m := hostSource.FindStringSubmatch(source)
	if m == nil {
		return false
	}
	scheme, host, port := m[1], m[2], m[3]
	if scheme == "" && page != nil {
		scheme = page.Scheme
	}
	return schemeMatches(scheme, own.Scheme) && hostMatches(host, own.Hostname()) && portMatches(port, scheme, own)
}

// schemeMatches allows the upgrade from http to https, as CSP does.
func schemeMatches(source, scheme string) bool {
	return source == scheme || (source == "http" && scheme == "https")
}

func hostMatches(source, host string) bool {
	host = strings.ToLower(host)
	switch {
	case source == "*":
		return true
	case strings.HasPrefix(source, "*."):
		return strings.HasSuffix(host, source[1:])
	}
	return source == host
}

func portMatches(source, sourceScheme string, own *url.URL) bool {
	port := own.Port()
	if port == "" {
		port = defaultPort(own.Scheme)
	}
	switch source {
	case "*":
		return true
	case "":
		source = defaultPort(sourceScheme)
	}
	return source == port || (source == "80" && port == "443" && own.Scheme == "https")
}

func defaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func shorten(s string) string {
	if len(s) <= maxShownHeader {
		return s
	}
	return s[:maxShownHeader] + "…"
}
