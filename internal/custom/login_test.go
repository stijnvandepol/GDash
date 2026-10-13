package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// piHole stands in for Pi-hole v6: POST /api/auth with {"password": …}
// gives a session ID that /api/stats/summary wants in X-FTL-SID.
type piHole struct {
	password string
	validity float64
	mu       sync.Mutex
	logins   int
	// valid holds the session IDs that are still accepted.
	valid map[string]bool
	// passwords holds every password that a login sent.
	passwords []string
}

func (p *piHole) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/auth":
		var body struct {
			Password string `json:"password"`
		}
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p.passwords = append(p.passwords, body.Password)
		if body.Password != p.password {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"session":{"valid":false,"sid":null}}`))
			return
		}
		p.logins++
		sid := fmt.Sprintf("sid-%d-abcdef", p.logins)
		p.valid[sid] = true
		fmt.Fprintf(w, `{"session":{"valid":true,"sid":%q,"validity":%v},"took":0.01}`, sid, p.validity)
	case "/api/stats/summary":
		sid := r.Header.Get("X-FTL-SID")
		if sid == "" {
			sid = r.URL.Query().Get("sid")
		}
		if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			sid = bearer
		}
		if !p.valid[sid] {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"key":"unauthorized"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"queries":{"total":1234,"blocked":56}}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// endSessions makes the stand-in forget every session, as after a restart.
func (p *piHole) endSessions() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.valid = map[string]bool{}
}

func (p *piHole) loginCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.logins
}

// clock is a settable time for the fetcher.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newPiHole(t *testing.T, validity float64) (*piHole, *httptest.Server) {
	t.Helper()
	p := &piHole{password: testSecret, validity: validity, valid: map[string]bool{}}
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	return p, srv
}

func loginFetcher(t *testing.T) (*Fetcher, *clock) {
	t.Helper()
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{now: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	return &Fetcher{HTTP: outbound.NewClient(5 * time.Second), UserAgent: "GDash/test", Box: box, Now: c.Now}, c
}

func piHoleSource(srv *httptest.Server) Source {
	return Source{
		ID: 7, Name: "Pi-hole", Mode: ModeFetch, URL: srv.URL + "/api/stats/summary", Method: "GET",
		Auth: AuthLogin, AuthName: "X-FTL-SID", AuthSecret: testSecret,
		Login: Login{URL: srv.URL + "/api/auth", Body: `{"password": "{secret}"}`, TokenPath: "session.sid", Place: TokenInHeader, ValidityPath: "session.validity"},
	}
}

func fetchTotal(t *testing.T, f *Fetcher, src Source) {
	t.Helper()
	answer, err := f.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(answer) != `{"queries":{"total":1234,"blocked":56}}` {
		t.Fatalf("answer = %s", answer)
	}
}

func TestLoginOnceAndReuseTheSession(t *testing.T) {
	p, srv := newPiHole(t, 1800)
	f, _ := loginFetcher(t)
	src := piHoleSource(srv)

	fetchTotal(t, f, src)
	fetchTotal(t, f, src)
	fetchTotal(t, f, src)
	if n := p.loginCount(); n != 1 {
		t.Fatalf("%d logins for three fetches, want 1", n)
	}
	// The token is only held sealed.
	s := f.session(src.ID)
	if s.sealed == "" || strings.Contains(s.sealed, "sid-1") {
		t.Fatalf("session holds %q, want the sealed token", s.sealed)
	}
}

func TestLoginAgainWhenTheAPIRefusesTheSession(t *testing.T) {
	p, srv := newPiHole(t, 1800)
	f, _ := loginFetcher(t)
	src := piHoleSource(srv)
	fetchTotal(t, f, src)

	p.endSessions()
	fetchTotal(t, f, src)
	if n := p.loginCount(); n != 2 {
		t.Fatalf("%d logins, want 2: one more after the session was refused", n)
	}
}

func TestLoginAgainWhenTheSessionRunsOut(t *testing.T) {
	p, srv := newPiHole(t, 300)
	f, c := loginFetcher(t)
	src := piHoleSource(srv)
	fetchTotal(t, f, src)

	// 300 seconds, renewed 30 seconds early.
	c.advance(269 * time.Second)
	fetchTotal(t, f, src)
	if n := p.loginCount(); n != 1 {
		t.Fatalf("%d logins before the session ran out, want 1", n)
	}
	c.advance(2 * time.Second)
	fetchTotal(t, f, src)
	if n := p.loginCount(); n != 2 {
		t.Fatalf("%d logins after the session ran out, want 2", n)
	}
}

func TestLoginAgainWhenTheLoginChanges(t *testing.T) {
	p, srv := newPiHole(t, 1800)
	f, _ := loginFetcher(t)
	src := piHoleSource(srv)
	fetchTotal(t, f, src)
	src.Login.Body = `{"password": "{secret}", "totp": null}`
	fetchTotal(t, f, src)
	if n := p.loginCount(); n != 2 {
		t.Fatalf("%d logins, want 2: a changed login does not reuse the old session", n)
	}
}

func TestLoginPutsTheTokenWhereTheAPIWantsIt(t *testing.T) {
	for _, place := range []struct{ place, name string }{{TokenInQuery, "sid"}, {TokenAsBearer, ""}} {
		_, srv := newPiHole(t, 0)
		f, _ := loginFetcher(t)
		src := piHoleSource(srv)
		src.Login.Place, src.AuthName, src.Login.ValidityPath = place.place, place.name, ""
		fetchTotal(t, f, src)
	}
}

func TestLoginEscapesTheSecretInTheBody(t *testing.T) {
	const password = `pa"ss\word {x}`
	p, srv := newPiHole(t, 1800)
	p.password = password
	f, _ := loginFetcher(t)
	src := piHoleSource(srv)
	src.AuthSecret = password
	fetchTotal(t, f, src)
	if len(p.passwords) != 1 || p.passwords[0] != password {
		t.Fatalf("the API got passwords %q, want exactly %q", p.passwords, password)
	}
}

func TestLoginFailuresNeverShowTheSecretOrToken(t *testing.T) {
	p, srv := newPiHole(t, 1800)
	f, _ := loginFetcher(t)

	wrong := piHoleSource(srv)
	wrong.AuthSecret = "not-" + testSecret
	_, err := f.Fetch(context.Background(), wrong)
	mustNotLeak(t, err)
	if !strings.Contains(err.Error(), "the login was refused with status 401; check the password") || strings.Contains(err.Error(), "not-") {
		t.Fatalf("wrong password: err = %v", err)
	}

	noToken := piHoleSource(srv)
	noToken.Login.TokenPath = "session.token"
	_, err = f.Fetch(context.Background(), noToken)
	mustNotLeak(t, err)
	if !strings.Contains(err.Error(), "no token at session.token") {
		t.Fatalf("missing token: err = %v", err)
	}

	// The API refuses every session, also a fresh one: one more login,
	// then the error, without the session ID in it.
	refusing := piHoleSource(srv)
	refusing.AuthName = "X-Not-The-SID"
	before := p.loginCount()
	_, err = f.Fetch(context.Background(), refusing)
	mustNotLeak(t, err)
	if strings.Contains(err.Error(), "sid-") || !strings.Contains(err.Error(), "status 401") || p.loginCount() != before+1 {
		t.Fatalf("a refused fresh session: err = %v after %d logins, want 1 login and the status", err, p.loginCount()-before)
	}

	fetched := piHoleSource(srv)
	fetched.ID = 8
	fetchTotal(t, f, fetched)
	p.endSessions()
	p.password = "changed"
	_, err = f.Fetch(context.Background(), fetched)
	mustNotLeak(t, err)
	if strings.Contains(err.Error(), "sid-") || !strings.Contains(err.Error(), "check the password") {
		t.Fatalf("a refused session and a refused login: err = %v", err)
	}
}

func TestLoginStaysOnTheSameHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the password reached another host")
	}))
	t.Cleanup(other.Close)
	p, srv := newPiHole(t, 1800)
	// Both stand-ins listen on 127.0.0.1; by name, the other one is another host.
	elsewhere := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere+"/api/auth", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirecting.Close)
	f, _ := loginFetcher(t)
	src := piHoleSource(srv)
	src.Login.URL = redirecting.URL + "/api/auth"
	_, err := f.Fetch(context.Background(), src)
	mustNotLeak(t, err)
	if !strings.Contains(err.Error(), "redirected to another host") || p.loginCount() != 0 {
		t.Fatalf("err = %v, logins %d", err, p.loginCount())
	}
}

func TestValidityIsClamped(t *testing.T) {
	doc := map[string]any{"s": map[string]any{"v": 3.0, "long": 1e9, "text": "1800"}}
	for path, want := range map[string]time.Duration{
		"s.v": minValidity, "s.long": maxValidity, "s.text": 0, "s.none": 0, "": 0,
	} {
		if got := validityAt(doc, path); got != want {
			t.Errorf("validityAt(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestCheckLogin(t *testing.T) {
	login := func(mutate func(*Login)) func(*Input) {
		return func(in *Input) {
			in.URL = "http://pi.hole/api/stats/summary"
			in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthLogin, "X-FTL-SID", testSecret
			l := Login{URL: "http://pi.hole/api/auth", Body: `{"password": "{secret}"}`, TokenPath: "session.sid", Place: TokenInHeader, ValidityPath: "session.validity"}
			if mutate != nil {
				mutate(&l)
			}
			in.Auth.Login = &l
		}
	}
	src, err := Check(input(login(nil)), nil)
	if err != nil || src.Login.TokenPath != "session.sid" || src.AuthName != "X-FTL-SID" || src.AuthSecret != testSecret {
		t.Fatalf("Check = %+v, %v", src, err)
	}
	if src, err := Check(input(login(func(l *Login) { l.Place = TokenAsBearer })), nil); err != nil || src.AuthName != "" {
		t.Fatalf("bearer: %+v, %v; want no header name", src, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Input)
		fields []string
	}{
		{"another host", login(func(l *Login) { l.URL = "http://evil.example/api/auth" }), []string{"auth.login.url"}},
		{"https to http", login(func(l *Login) { l.URL = "https://pi.hole/api/auth" }), []string{"auth.login.url"}},
		{"no placeholder", login(func(l *Login) { l.Body = `{"password": "hunter2"}` }), []string{"auth.login.body"}},
		{"placeholder outside a string", login(func(l *Login) { l.Body = `{"password": {secret}}` }), []string{"auth.login.body"}},
		{"bad paths", login(func(l *Login) { l.TokenPath, l.ValidityPath = "", "session..x" }), []string{"auth.login.tokenPath", "auth.login.validityPath"}},
		{"unknown place", login(func(l *Login) { l.Place = "cookie" }), []string{"auth.login.place"}},
		{"reserved header", func(in *Input) { login(nil)(in); in.Auth.Name = "Content-Type" }, []string{"auth.name"}},
		{"no password", func(in *Input) { login(nil)(in); in.Auth.Secret = "" }, []string{"auth.secret"}},
		{"no login", func(in *Input) { login(nil)(in); in.Auth.Login = nil }, []string{"auth.login"}},
	} {
		_, err := Check(input(tc.mutate), nil)
		if got := fieldsOf(err); strings.Join(got, ",") != strings.Join(tc.fields, ",") {
			t.Errorf("%s: invalid fields %v, want %v", tc.name, got, tc.fields)
		}
	}
	// A pushed source has no login.
	pushed := input(func(in *Input) { in.Mode, in.URL = ModePush, ""; in.Auth.Login = &Login{} })
	if got := fieldsOf(func() error { _, err := Check(pushed, nil); return err }()); strings.Join(got, ",") != "auth.kind" {
		t.Errorf("pushed source with a login: fields %v", got)
	}
}

// Logging in needs a session that only custom sources keep: the shared
// check, which cameras use too, refuses the kind.
func TestCheckAuthRefusesLoginForOthers(t *testing.T) {
	var errs validation.Collector
	CheckAuth(&errs, Auth{Kind: AuthLogin, Name: "X-FTL-SID", Secret: testSecret}, Auth{}, "http://pi.hole/api/stats/summary")
	if got := fieldsOf(errs.Err()); strings.Join(got, ",") != "auth.kind" {
		t.Fatalf("fields = %v, want auth.kind", got)
	}
}
