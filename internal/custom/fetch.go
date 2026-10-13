package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

const (
	// MaxResponseBytes bounds an answer: the server keeps the last one of
	// every source in memory and on disk.
	MaxResponseBytes = 1 << 20
	// maxRedirects is low: an API redirects once, if at all.
	maxRedirects = 3
)

// FetchError is a failed fetch. Its message is safe to show an admin: it
// never holds the address's query, a header or a secret, only the host.
type FetchError struct {
	Message string
	// cause stays in the server log; for a network error it can hold the
	// address that was dialled, never the request's secrets.
	cause error
}

func (e *FetchError) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *FetchError) Unwrap() error { return e.cause }

// Fetcher fetches the sources.
type Fetcher struct {
	// HTTP must refuse the addresses that outbound refuses and have a
	// timeout; its redirect policy is replaced by the stricter one below.
	HTTP      *http.Client
	UserAgent string
}

// Fetch requests src once and returns its answer as compact JSON.
func (f *Fetcher) Fetch(ctx context.Context, src Source) (json.RawMessage, error) {
	req, err := buildRequest(ctx, src)
	if err != nil {
		return nil, err
	}
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	client := *f.HTTP
	client.CheckRedirect = sameHostRedirects
	resp, err := client.Do(req)
	if err != nil {
		return nil, requestError(req.URL.Host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &FetchError{Message: statusMessage(resp)}
	}
	if !jsonContentType(resp.Header.Get("Content-Type")) {
		return nil, &FetchError{Message: "the API answered with " + mediaType(resp.Header.Get("Content-Type")) + ", not JSON"}
	}
	answer, err := ReadAnswer(resp.Body)
	switch {
	case errors.Is(err, ErrAnswerTooLarge):
		return nil, &FetchError{Message: "the answer is larger than 1 MB; use an address that returns less, such as with a filter or a limit"}
	case errors.Is(err, ErrAnswerNotJSON):
		return nil, &FetchError{Message: "the answer is not valid JSON"}
	case err != nil:
		return nil, requestError(req.URL.Host, err)
	}
	return answer, nil
}

// Errors of ReadAnswer.
var (
	ErrAnswerTooLarge = errors.New("the answer is larger than 1 MB")
	ErrAnswerNotJSON  = errors.New("the answer is not valid JSON")
)

// ReadAnswer reads an answer of at most MaxResponseBytes and returns it as
// compact JSON. A fetched answer and one that a household's system sends
// to GDash both pass through it, so they are held to the same limits.
func ReadAnswer(r io.Reader) (json.RawMessage, error) {
	body, err := io.ReadAll(io.LimitReader(r, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxResponseBytes {
		return nil, ErrAnswerTooLarge
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return nil, ErrAnswerNotJSON
	}
	return compact.Bytes(), nil
}

// buildRequest puts the authentication and headers on the request. The
// key of query authentication is added here and never stored in the URL.
func buildRequest(ctx context.Context, src Source) (*http.Request, error) {
	u, err := url.Parse(src.URL)
	if err != nil {
		return nil, &FetchError{Message: "the address of this source is invalid"}
	}
	if src.Auth == AuthQuery {
		q := u.Query()
		q.Set(src.AuthName, src.AuthSecret)
		u.RawQuery = q.Encode()
	}
	var body io.Reader
	if src.Method == http.MethodPost && src.Body != "" {
		body = strings.NewReader(src.Body)
	}
	req, err := http.NewRequestWithContext(ctx, src.Method, u.String(), body)
	if err != nil {
		return nil, &FetchError{Message: "the request for this source cannot be made"}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, h := range src.Headers {
		req.Header.Set(h.Name, h.Value)
	}
	switch src.Auth {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+src.AuthSecret)
	case AuthHeader:
		req.Header.Set(src.AuthName, src.AuthSecret)
	case AuthBasic:
		req.SetBasicAuth(src.AuthName, src.AuthSecret)
	}
	return req, nil
}

// errRedirectElsewhere stops a redirect to another host. Go drops the
// Authorization header there, but not an API key in another header, so
// following it could hand the key to whoever the API points at.
var errRedirectElsewhere = errors.New("the API redirected to another host")

func sameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	first := via[0].URL
	if first.Scheme == "https" && req.URL.Scheme != "https" {
		return errors.New("refused a redirect from https to http")
	}
	if !strings.EqualFold(req.URL.Hostname(), first.Hostname()) {
		return errRedirectElsewhere
	}
	return nil
}

// requestError describes a failed request by host only. A *url.Error
// carries the full address, which may hold the API key of query
// authentication, so only its cause is kept.
func requestError(host string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return &FetchError{Message: "the API at " + host + " did not answer in time"}
	case errors.Is(err, outbound.ErrBlockedAddress):
		return &FetchError{Message: "GDash does not fetch from " + host + ": link-local and cloud metadata addresses are refused"}
	case errors.Is(err, errRedirectElsewhere):
		return &FetchError{Message: "the API redirected to another host; use the address it redirects to"}
	}
	return &FetchError{Message: "the API at " + host + " cannot be reached from the GDash server", cause: err}
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

func statusMessage(resp *http.Response) string {
	msg := fmt.Sprintf("the API answered with status %d", resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return msg + "; check the authentication"
	case http.StatusNotFound:
		return msg + "; check the address"
	case http.StatusTooManyRequests:
		return msg + "; it is asked too often, so choose a longer refresh"
	}
	return msg
}

// jsonContentType accepts JSON media types, and text/plain and none at
// all, which some APIs send with JSON; the body is checked as JSON anyway.
func jsonContentType(header string) bool {
	if header == "" {
		return true
	}
	t := mediaType(header)
	return t == "application/json" || strings.HasSuffix(t, "+json") || t == "text/json" || t == "text/plain"
}

func mediaType(header string) string {
	t, _, err := mime.ParseMediaType(header)
	if err != nil || len(t) > 100 {
		return "an unknown content type"
	}
	return t
}
