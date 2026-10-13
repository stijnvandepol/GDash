package custom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// maxTokenBytes bounds a session token; a JWT is a few hundred bytes.
	maxTokenBytes = 8 << 10
	// A validity outside these bounds is clamped: a few seconds would mean
	// a login per request, and a day is longer than any session needs.
	minValidity = 10 * time.Second
	maxValidity = 24 * time.Hour
	// renewMargin logs in again a little before a token runs out, so a
	// request never carries one that expires on the way.
	renewMargin = 30 * time.Second
	// subjectLogin is what the messages about a login call it.
	subjectLogin = "the login"
)

// Where a login's session token goes.
const (
	TokenInHeader = "header"
	TokenInQuery  = "query"
	TokenAsBearer = "bearer"
)

// SecretPlaceholder marks where the secret goes in a login body.
const SecretPlaceholder = "{secret}"

// Login is how a source with AuthLogin gets its session token. None of it
// is secret: the password is the authentication's secret, which is put in
// Body's placeholder when logging in.
type Login struct {
	// URL is on the same scheme, host and port as the source's address.
	URL string `json:"url"`
	// Body is a JSON template such as {"password": "{secret}"}, posted to URL.
	Body string `json:"body"`
	// TokenPath is where the token is in the login's answer, such as session.sid.
	TokenPath string `json:"tokenPath"`
	// Place is TokenInHeader or TokenInQuery, named by the authentication's
	// Name, or TokenAsBearer.
	Place string `json:"place"`
	// ValidityPath is where the answer says how many seconds the token
	// lasts, such as session.validity, or empty when it does not.
	ValidityPath string `json:"validityPath"`
}

// checkLoginAuth checks the authentication of a source that logs in
// first, like CheckAuth does for the other kinds: an empty password keeps
// old's when old logged in too.
func checkLoginAuth(errs *validation.Collector, in Input, old Auth, address string) Auth {
	secret := in.Auth.Secret
	if secret == "" && old.Kind == AuthLogin {
		secret = old.Secret
	}
	login := checkLogin(errs, in, address)
	if !plainSecret(secret, true) {
		errs.Add("auth.secret", "enter the password")
	}
	name := in.Auth.Name
	if login.Place == TokenAsBearer {
		name = ""
	}
	return Auth{Kind: AuthLogin, Name: name, Secret: secret, Login: login}
}

// checkLogin checks how a source logs in. The login address must be on the
// source's own scheme, host and port: the password is only ever sent where
// the data comes from.
func checkLogin(errs *validation.Collector, in Input, sourceURL string) Login {
	if in.Auth.Login == nil {
		errs.Add("auth.login", "fill in how GDash logs in")
		return Login{}
	}
	login := *in.Auth.Login
	login.URL = strings.TrimSpace(login.URL)
	var loginErrs validation.Collector
	if u := checkURL(&loginErrs, login.URL); u != "" {
		login.URL = u
		if sourceURL != "" && !sameOrigin(u, sourceURL) {
			errs.Add("auth.login.url", "use a login address on the same host as the API address, with the same http or https and port")
		}
	}
	for _, f := range validation.Fields(loginErrs.Err()) {
		errs.Add("auth.login."+f.Field, f.Message)
	}
	if !validLoginBody(login.Body) {
		errs.Add("auth.login.body", `use a JSON body of at most 8 KB with {secret} where the password goes, such as {"password": "{secret}"}`)
	}
	if _, err := ParsePath(login.TokenPath); err != nil {
		errs.Add("auth.login.tokenPath", "use the path of the token in the login's answer, such as session.sid")
	}
	if login.ValidityPath != "" {
		if _, err := ParsePath(login.ValidityPath); err != nil {
			errs.Add("auth.login.validityPath", "use the path of the seconds the token lasts, such as session.validity, or leave it empty")
		}
	}
	switch login.Place {
	case TokenInHeader:
		if !headerName.MatchString(in.Auth.Name) || reservedHeaders[strings.ToLower(in.Auth.Name)] && !strings.EqualFold(in.Auth.Name, "authorization") {
			errs.Add("auth.name", "use a header name such as X-FTL-SID")
		}
	case TokenInQuery:
		if !paramName.MatchString(in.Auth.Name) {
			errs.Add("auth.name", "use a parameter name such as sid")
		} else if u, err := url.Parse(sourceURL); err == nil && sourceURL != "" && u.Query().Has(in.Auth.Name) {
			errs.Add("auth.name", "the address already has this parameter; remove it from the address")
		}
	case TokenAsBearer:
	default:
		errs.Add("auth.login.place", "choose header, query or bearer")
	}
	return login
}

// validLoginBody reports whether body is a JSON template of at most
// maxBodyBytes that becomes valid JSON with the secret in it.
func validLoginBody(body string) bool {
	return len(body) <= maxBodyBytes && strings.Contains(body, SecretPlaceholder) &&
		json.Valid([]byte(strings.ReplaceAll(body, SecretPlaceholder, "x")))
}

// sameOrigin reports whether two checked addresses share scheme, host and port.
func sameOrigin(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	return errA == nil && errB == nil && ua.Scheme == ub.Scheme && strings.EqualFold(ua.Host, ub.Host)
}

// applyToken puts a session token on req where the login says it goes.
func (a Auth) applyToken(req *http.Request, token string) {
	switch a.Login.Place {
	case TokenInHeader:
		req.Header.Set(a.Name, token)
	case TokenInQuery:
		q := req.URL.Query()
		q.Set(a.Name, token)
		req.URL.RawQuery = q.Encode()
	case TokenAsBearer:
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// session is the session token of one source that logs in first. The
// token is only kept sealed, in memory: never stored, logged or returned.
type session struct {
	// mu allows one login at a time per source, so two refreshes at once
	// do not each take one of the API's sessions.
	mu sync.Mutex
	// key identifies the login the token came from; a changed address,
	// body or password logs in again.
	key     string
	sealed  string
	expires time.Time // zero: until the API refuses it
}

// session returns the session of source id, empty before its first login.
func (f *Fetcher) session(id int64) *session {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions == nil {
		f.sessions = map[int64]*session{}
	}
	s, ok := f.sessions[id]
	if !ok {
		s = &session{}
		f.sessions[id] = s
	}
	return s
}

// token returns the session token for src. It logs in when there is no
// token for src's current login or it ran out; fresh is then true.
func (s *session) token(ctx context.Context, f *Fetcher, src Source) (token string, fresh bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := f.Box.Fingerprint(fmt.Appendf(nil, "%s\x00%+v\x00%s", src.AuthName, src.Login, src.AuthSecret))
	if s.sealed != "" && s.key == key && (s.expires.IsZero() || f.Now().Before(s.expires)) {
		if token, err := f.Box.Open(s.sealed); err == nil {
			return token, false, nil
		}
	}
	s.sealed = ""
	token, validity, err := f.logIn(ctx, src)
	if err != nil {
		return "", false, err
	}
	if s.sealed, err = f.Box.Seal(token); err != nil {
		return "", false, fmt.Errorf("seal session token: %w", err)
	}
	s.key, s.expires = key, time.Time{}
	if validity > 0 {
		s.expires = f.Now().Add(validity - min(renewMargin, validity/10))
	}
	return token, true, nil
}

// forget drops the token, so the next request logs in again.
func (s *session) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = ""
}

// logIn posts src's login body with the secret in it and reads the token,
// and how long it lasts when the answer says so (0 otherwise), from the
// answer. Errors name the host and the paths, never the secret, the body
// or the answer.
func (f *Fetcher) logIn(ctx context.Context, src Source) (string, time.Duration, error) {
	body := strings.ReplaceAll(src.Login.Body, SecretPlaceholder, jsonStringContent(src.AuthSecret))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, src.Login.URL, strings.NewReader(body))
	if err != nil {
		return "", 0, &FetchError{Message: "the login address of this source is invalid"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.do(req)
	if err != nil {
		return "", 0, RequestError(subjectLogin, req.URL.Host, err)
	}
	defer resp.Body.Close()
	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return "", 0, &FetchError{Message: fmt.Sprintf("the login was refused with status %d; check the password", code)}
	case code == http.StatusTooManyRequests:
		return "", 0, &FetchError{Message: "the login answered with status 429; it is asked too often, so choose a longer refresh"}
	case code < 200 || code > 299:
		return "", 0, &FetchError{Message: fmt.Sprintf("the login answered with status %d; check the login address", code)}
	}
	answer, err := ReadAnswer(resp.Body)
	switch {
	case errors.Is(err, ErrAnswerTooLarge) || errors.Is(err, ErrAnswerNotJSON):
		return "", 0, &FetchError{Message: "the login's answer is not JSON of at most 1 MB"}
	case err != nil:
		return "", 0, RequestError(subjectLogin, req.URL.Host, err)
	}
	var doc any
	if err := json.Unmarshal(answer, &doc); err != nil {
		return "", 0, &FetchError{Message: "the login's answer is not JSON of at most 1 MB"}
	}
	token, ok := tokenAt(doc, src.Login.TokenPath)
	if !ok {
		return "", 0, &FetchError{Message: "the login's answer has no token at " + src.Login.TokenPath + "; check the token path"}
	}
	return token, validityAt(doc, src.Login.ValidityPath), nil
}

// tokenAt reads a token: text that fits in a header.
func tokenAt(doc any, path string) (string, bool) {
	p, err := ParsePath(path)
	if err != nil {
		return "", false
	}
	v, found := p.Resolve(doc)
	token, ok := v.(string)
	return token, found && ok && token != "" && len(token) <= maxTokenBytes && !validation.HasControlCharacter(token)
}

// validityAt reads how many seconds a token lasts, clamped to the bounds,
// or 0 when the answer does not say: the token is then used until the API
// refuses it.
func validityAt(doc any, path string) time.Duration {
	if path == "" {
		return 0
	}
	p, err := ParsePath(path)
	if err != nil {
		return 0
	}
	v, _ := p.Resolve(doc)
	seconds, ok := v.(float64)
	if !ok || seconds <= 0 {
		return 0
	}
	return min(max(time.Duration(seconds*float64(time.Second)), minValidity), maxValidity)
}

// jsonStringContent is s escaped for the inside of a JSON string, so a
// password with a quote or backslash cannot change the body around it.
func jsonStringContent(s string) string {
	quoted, _ := json.Marshal(s)
	return string(quoted[1 : len(quoted)-1])
}
