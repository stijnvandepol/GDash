package custom

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Limits of the sources. Every source is a request per refresh, so a
// household needs a handful; the interval keeps a free API's quota safe.
const (
	MaxSources     = 20
	MinInterval    = 30 * time.Second
	MaxInterval    = 24 * time.Hour
	maxHeaders     = 10
	maxNameLength  = 60
	maxURLLength   = 2048
	maxBodyBytes   = 8 << 10
	maxSecretBytes = 2048
	// MaxSampleBytes leaves room in a request body of 64 KiB.
	MaxSampleBytes = 48 << 10
	maxHeaderValue = 1024
)

// How GDash gets the answer of a source.
const (
	// ModeFetch asks the source's address every interval.
	ModeFetch = "fetch"
	// ModePush waits for the household's own system, such as a
	// HomeyScript or Home Assistant, to send the answer.
	ModePush = "push"
)

// pushGrace is added to two intervals before a pushed answer counts as
// stale: a script that runs every minute is late now and then.
const pushGrace = 30 * time.Second

// PushFreshFor is how long a pushed answer counts as current: two
// intervals, so one missed push does not show at once, and a margin.
func PushFreshFor(interval time.Duration) time.Duration { return 2*interval + pushGrace }

// Header is an extra request header. A secret one is sealed and never
// shown again.
type Header struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// Source is a source with its secrets, for the server only.
type Source struct {
	ID   int64
	Name string
	// Mode is ModeFetch or ModePush. A pushed source has no address,
	// authentication or headers, and Interval is how often an answer is
	// expected.
	Mode   string
	URL    string
	Method string
	Body   string
	Auth   string
	// AuthName is the header or query parameter that carries an API key,
	// or the user name for basic auth.
	AuthName string
	// AuthSecret is the token, API key or password.
	AuthSecret string
	Headers    []Header
	Interval   time.Duration
	// Sample is a pasted answer to design widgets against, or empty.
	Sample string
}

// Input is a source as the web interface sends it. An empty secret keeps
// the one already stored, so the form never needs to show it.
type Input struct {
	Name string `json:"name"`
	// Mode is "fetch" (also when empty, as before modes existed) or "push".
	Mode   string `json:"mode"`
	URL    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`
	Auth   struct {
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Secret string `json:"secret"`
	} `json:"auth"`
	Headers         []Header        `json:"headers"`
	IntervalSeconds int             `json:"intervalSeconds"`
	Sample          json.RawMessage `json:"sample"`
}

var (
	// headerName is an HTTP token (RFC 9110).
	headerName = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]{1,64}$")
	paramName  = regexp.MustCompile(`^[A-Za-z0-9_.\-\[\]]{1,64}$`)
	// reservedHeaders are set by Go's HTTP client or by the authentication
	// choice; setting them by hand breaks the request or hides the secret.
	reservedHeaders = map[string]bool{
		"host": true, "content-length": true, "transfer-encoding": true, "connection": true, "upgrade": true,
		"te": true, "trailer": true, "keep-alive": true, "proxy-authorization": true, "proxy-connection": true,
		"authorization": true, "content-type": true,
	}
)

// Check validates an input, merges in the secrets of old (the stored source
// when it is edited, or nil) that the input keeps, and returns the source
// to store. It returns a *validation.Error naming every invalid field;
// messages never contain a secret.
func Check(in Input, old *Source) (Source, error) {
	var errs validation.Collector
	src := Source{Name: in.Name}
	if old != nil {
		src.ID = old.ID
	}
	if msg := nameProblem(in.Name); msg != "" {
		errs.Add("name", msg)
	}
	switch in.Mode {
	case "", ModeFetch:
		src.Mode = ModeFetch
		checkRequest(&errs, in, old, &src)
	case ModePush:
		src.Mode = ModePush
		checkPushed(&errs, in, &src)
	default:
		errs.Add("mode", "choose fetch or push")
	}
	interval := time.Duration(in.IntervalSeconds) * time.Second
	if interval < MinInterval || interval > MaxInterval {
		errs.Add("intervalSeconds", "use 30 seconds to 24 hours (30 to 86400)")
	}
	src.Interval = interval
	src.Sample = checkSample(&errs, in.Sample)
	if err := errs.Err(); err != nil {
		return Source{}, err
	}
	return src, nil
}

// checkRequest checks the request of a source that GDash fetches.
func checkRequest(errs *validation.Collector, in Input, old *Source, src *Source) {
	src.Method, src.Auth = strings.ToUpper(strings.TrimSpace(in.Method)), in.Auth.Kind
	src.URL = checkURL(errs, in.URL)
	src.Body = checkBody(errs, src.Method, in.Body)
	var oldAuth Auth
	if old != nil {
		oldAuth = old.auth()
	}
	auth := CheckAuth(errs, Auth{Kind: in.Auth.Kind, Name: in.Auth.Name, Secret: in.Auth.Secret}, oldAuth, src.URL)
	src.AuthName, src.AuthSecret = auth.Name, auth.Secret
	src.Headers = checkHeaders(errs, in.Headers, old)
}

// auth is the source's authentication.
func (s Source) auth() Auth {
	return Auth{Kind: s.Auth, Name: s.AuthName, Secret: s.AuthSecret}
}

// checkPushed refuses a request for a source that is sent to GDash, rather
// than ignoring it: an address that is never fetched would look like it
// is. Its secrets, if it fetched before, are dropped with it.
func checkPushed(errs *validation.Collector, in Input, src *Source) {
	src.Method, src.Auth = "GET", AuthNone
	const unused = "a source that your system sends to GDash has none; leave it empty"
	if strings.TrimSpace(in.URL) != "" {
		errs.Add("url", unused)
	}
	if method := strings.ToUpper(strings.TrimSpace(in.Method)); method != "" && method != "GET" {
		errs.Add("method", unused)
	}
	if in.Body != "" {
		errs.Add("body", unused)
	}
	if (in.Auth.Kind != "" && in.Auth.Kind != AuthNone) || in.Auth.Name != "" || in.Auth.Secret != "" {
		errs.Add("auth.kind", unused)
	}
	if len(in.Headers) > 0 {
		errs.Add("headers", unused)
	}
}

func nameProblem(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed != name || utf8.RuneCountInString(name) > maxNameLength || validation.HasControlCharacter(name) {
		return "use 1 to 60 characters without leading or trailing spaces"
	}
	return ""
}

func checkURL(errs *validation.Collector, raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	switch {
	case err != nil || u.Host == "" || len(raw) > maxURLLength:
		errs.Add("url", "paste the full address of the API, starting with https://")
	case u.Scheme != "http" && u.Scheme != "https":
		errs.Add("url", "use an http or https address")
	case u.User != nil:
		// The address is shown in the web interface; a password belongs
		// in the authentication, where it is sealed.
		errs.Add("url", "use basic auth instead of a user name and password in the address")
	case u.Fragment != "":
		errs.Add("url", "use the address without a # part")
	default:
		return u.String()
	}
	return ""
}

func checkBody(errs *validation.Collector, method, body string) string {
	switch method {
	case "GET":
		if body != "" {
			errs.Add("body", "a GET request has no body; choose POST to send one")
		}
	case "POST":
		if len(body) > maxBodyBytes || (body != "" && !json.Valid([]byte(body))) {
			errs.Add("body", "use a JSON body of at most 8 KB, or leave it empty")
		}
	default:
		errs.Add("method", "choose GET or POST")
	}
	return body
}

func checkHeaders(errs *validation.Collector, headers []Header, old *Source) []Header {
	if len(headers) > maxHeaders {
		errs.Add("headers", fmt.Sprintf("use at most %d headers", maxHeaders))
		return nil
	}
	out := make([]Header, 0, len(headers))
	seen := map[string]bool{}
	for i, h := range headers {
		field := fmt.Sprintf("headers[%d]", i)
		lower := strings.ToLower(h.Name)
		switch {
		case !headerName.MatchString(h.Name):
			errs.Add(field+".name", "use a header name such as Accept-Language")
			continue
		case reservedHeaders[lower]:
			errs.Add(field+".name", "GDash sets this header itself; use the authentication for a token")
			continue
		case seen[lower]:
			errs.Add(field+".name", "this header is already in the list")
			continue
		}
		seen[lower] = true
		if h.Secret && h.Value == "" && old != nil {
			h.Value = oldSecretHeader(old, h.Name)
		}
		if h.Value == "" || len(h.Value) > maxHeaderValue || validation.HasControlCharacter(h.Value) {
			errs.Add(field+".value", "enter a value of at most 1024 characters on one line")
			continue
		}
		out = append(out, h)
	}
	return out
}

func oldSecretHeader(old *Source, name string) string {
	for _, h := range old.Headers {
		if h.Secret && strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func checkSample(errs *validation.Collector, sample json.RawMessage) string {
	trimmed := strings.TrimSpace(string(sample))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if len(trimmed) > MaxSampleBytes || !json.Valid([]byte(trimmed)) {
		errs.Add("sample", "paste a JSON answer of at most 48 KB")
		return ""
	}
	return trimmed
}
