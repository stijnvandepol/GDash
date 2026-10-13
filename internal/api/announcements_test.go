package api

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a log destination that the server and the test can use at
// the same time.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// withDisplays logs in, creates the kitchen and hall displays and makes an
// API token. It returns the token's secret.
func (e *testEnv) withDisplays(t *testing.T) string {
	t.Helper()
	e.setUp(t)
	e.createKitchen(t)
	if status, body := e.do(t, "POST", "/api/displays", `{"id":"hall","name":"Hall"}`); status != http.StatusCreated {
		t.Fatalf("create hall = %d %s", status, body)
	}
	_, secret := e.makeToken(t, "Home Assistant")
	return secret
}

func TestAnnouncementNeedsATokenOrALogin(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	_, secret := env.makeToken(t, "Home Assistant")
	const body = `{"message":"The washing machine is done"}`

	if status, _ := env.bearer(t, "POST", "/api/announcements", body, "", nil); status != http.StatusUnauthorized {
		t.Errorf("without credentials = %d, want 401", status)
	}
	for name, header := range map[string]string{
		"wrong token":    "Bearer gdk_" + strings.Repeat("A", 43),
		"another scheme": "Basic " + secret,
		"no token":       "Bearer ",
		"token only":     secret,
	} {
		status, resp := env.bearer(t, "POST", "/api/announcements", body, "", map[string]string{"Authorization": header})
		if status != http.StatusUnauthorized || !strings.Contains(resp, `"invalid_token"`) {
			t.Errorf("%s = %d %s, want 401 invalid_token", name, status, resp)
		}
	}
	if env.board.For("keuken", time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)) != nil {
		t.Fatal("a refused request is shown")
	}
}

func TestAnnouncementWithAnAPIToken(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)

	status, body := env.bearer(t, "POST", "/api/announcements", `{"title":"Laundry","message":"The washing machine is done","seconds":30}`, secret, nil)
	if status != http.StatusCreated {
		t.Fatalf("post = %d %s", status, body)
	}
	for _, want := range []string{`"title":"Laundry"`, `"message":"The washing machine is done"`, `"expiresAt":"2026-10-08T17:00:30Z"`, `"displays":["hall","keuken"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("response lacks %s: %s", want, body)
		}
	}
	got := env.board.For("keuken", time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC))
	if got == nil || got.Message != "The washing machine is done" {
		t.Fatalf("the kitchen shows %+v", got)
	}
	if env.announced.Load() != 1 {
		t.Fatalf("screens were told %d times, want once", env.announced.Load())
	}
	// The use of the token shows in the list.
	if _, list := env.do(t, "GET", "/api/api-tokens", ""); strings.Contains(list, `"lastUsedAt":null`) {
		t.Fatalf("the token has no last use: %s", list)
	}
}

func TestAnnouncementWithAnAdminSession(t *testing.T) {
	env := newTestEnv(t, nil)
	env.withDisplays(t)
	status, body := env.do(t, "POST", "/api/announcements", `{"message":"Dinner is ready","displays":["keuken"]}`)
	if status != http.StatusCreated || !strings.Contains(body, `"displays":["keuken"]`) {
		t.Fatalf("post = %d %s", status, body)
	}
	now := time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)
	if env.board.For("keuken", now) == nil || env.board.For("hall", now) != nil {
		t.Fatal("the message is not on the kitchen only")
	}
}

func TestAnnouncementValidation(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)
	tests := []struct{ name, body, field string }{
		{"no message", `{}`, "message"},
		{"line break", `{"message":"a\nb"}`, "message"},
		{"too long", `{"message":"` + strings.Repeat("a", 281) + `"}`, "message"},
		{"seconds too low", `{"message":"x","seconds":4}`, "seconds"},
		{"seconds too high", `{"message":"x","seconds":3601}`, "seconds"},
		{"unknown display", `{"message":"x","displays":["garage"]}`, "displays"},
		{"empty display list", `{"message":"x","displays":[]}`, "displays"},
		{"long title", `{"message":"x","title":"` + strings.Repeat("t", 61) + `"}`, "title"},
	}
	for _, tt := range tests {
		status, body := env.bearer(t, "POST", "/api/announcements", tt.body, secret, nil)
		if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"code":"invalid_input"`) || !strings.Contains(body, `"field":"`+tt.field+`"`) {
			t.Errorf("%s = %d %s, want 422 on %s", tt.name, status, body, tt.field)
		}
	}
	for name, body := range map[string]string{
		"unknown field": `{"message":"x","colour":"red"}`,
		"not json":      `message=hello`,
		"wrong type":    `{"message":"x","seconds":"ten"}`,
	} {
		if status, resp := env.bearer(t, "POST", "/api/announcements", body, secret, nil); status != http.StatusBadRequest {
			t.Errorf("%s = %d %s, want 400", name, status, resp)
		}
	}
	if env.announced.Load() != 0 {
		t.Fatalf("screens were told %d times about invalid requests", env.announced.Load())
	}
}

func TestAnnouncementIsRateLimitedPerToken(t *testing.T) {
	env := newTestEnv(t, nil)
	first := env.withDisplays(t)
	_, second := env.makeToken(t, "Other automation")
	for i := range 30 {
		if status, body := env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, first, nil); status != http.StatusCreated {
			t.Fatalf("post %d = %d %s", i, status, body)
		}
	}
	req, _ := http.NewRequest("POST", env.srv.URL+"/api/announcements", strings.NewReader(`{"message":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+first)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != strconv.Itoa(60) {
		t.Fatalf("31st post = %d, Retry-After %q; want 429 and 60", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	if status, body := env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, second, nil); status != http.StatusCreated {
		t.Fatalf("another token = %d %s, want it unaffected", status, body)
	}
}

func TestRevokedTokenStopsWorking(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	id, secret := env.makeToken(t, "Home Assistant")
	if status, body := env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, secret, nil); status != http.StatusCreated {
		t.Fatalf("before revoking = %d %s", status, body)
	}
	env.do(t, "DELETE", "/api/api-tokens/"+strconv.FormatInt(id, 10), "")
	if status, _ := env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, secret, nil); status != http.StatusUnauthorized {
		t.Fatalf("after revoking = %d, want 401", status)
	}
}

// A token opens the announcement route and a pushed source's data, nothing
// else: not even the source itself.
func TestAPITokenOpensNoOtherRoute(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)
	if status, body := env.do(t, "POST", "/api/custom-sources", pushBody); status != http.StatusCreated {
		t.Fatalf("create pushed source = %d %s", status, body)
	}
	for _, req := range []struct{ method, path, body string }{
		{"GET", "/api/custom-sources", ""},
		{"PUT", "/api/custom-sources/1", pushBody},
		{"PUT", "/api/custom-sources/1/values", `{"values":[],"lists":[]}`},
		{"POST", "/api/custom-sources/1/tests", ""},
		{"DELETE", "/api/custom-sources/1", ""},
		{"GET", "/api/displays", ""},
		{"POST", "/api/displays", `{"id":"x","name":"X"}`},
		{"GET", "/api/api-tokens", ""},
		{"POST", "/api/api-tokens", `{"name":"mine"}`},
		{"GET", "/api/audit-events", ""},
		{"PUT", "/api/homeassistant", `{"url":"http://x","token":"y"}`},
		{"PUT", "/api/homey", `{"url":"http://x","key":"y"}`},
	} {
		if status, _ := env.bearer(t, req.method, req.path, req.body, secret, nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s with a token = %d, want 401", req.method, req.path, status)
		}
	}
}

// Cross-site protection covers the announcement route, for sessions and for
// tokens: a browser that sends one from another site is refused, a script on
// the server (no Origin, no Sec-Fetch-Site) is not.
func TestAnnouncementCrossSiteRequestsAreRefused(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)
	const body = `{"message":"x"}`
	crossSite := []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "http://evil.example"},
	}
	for _, headers := range crossSite {
		if status, _ := env.bearer(t, "POST", "/api/announcements", body, secret, headers); status != http.StatusForbidden {
			t.Errorf("token with %v = %d, want 403", headers, status)
		}
		req, _ := http.NewRequest("POST", env.srv.URL+"/api/announcements", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := env.client.Do(req) // carries the admin's session cookie
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("session with %v = %d, want 403", headers, resp.StatusCode)
		}
	}
	if status, _ := env.bearer(t, "POST", "/api/announcements", body, secret, map[string]string{"Sec-Fetch-Site": "same-origin"}); status != http.StatusCreated {
		t.Errorf("same-origin request = %d, want 201", status)
	}
	if env.announced.Load() != 1 {
		t.Fatalf("screens were told %d times, want only for the allowed request", env.announced.Load())
	}
}

// A cookie that comes along with a bad token must not rescue the request.
func TestSessionCookieDoesNotRescueABadToken(t *testing.T) {
	env := newTestEnv(t, nil)
	env.withDisplays(t)
	req, _ := http.NewRequest("POST", env.srv.URL+"/api/announcements", strings.NewReader(`{"message":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer gdk_wrong")
	resp, err := env.client.Do(req) // carries the admin's session cookie
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status = %d, WWW-Authenticate %q; want 401 and Bearer", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
}

func TestWrongTokensAreRateLimitedPerAddress(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)
	for range 20 {
		env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, "gdk_wrong", nil)
	}
	status, body := env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, secret, nil)
	if status != http.StatusTooManyRequests || !strings.Contains(body, `"rate_limited"`) {
		t.Fatalf("after 20 wrong tokens = %d %s, want 429", status, body)
	}
}

func TestTokensNeverReachTheLog(t *testing.T) {
	env := newTestEnv(t, nil)
	secret := env.withDisplays(t)
	env.bearer(t, "POST", "/api/announcements", `{"message":"private message"}`, secret, nil)
	env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, "gdk_a-wrong-token-value", nil)
	env.bearer(t, "POST", "/api/announcements", `{"message":"x","seconds":0}`, secret, nil)
	logs := env.logs.String()
	if !strings.Contains(logs, "announcement posted") {
		t.Fatalf("nothing is logged about the announcement:\n%s", logs)
	}
	for _, leak := range []string{secret, "a-wrong-token-value", "Bearer", "private message"} {
		if strings.Contains(logs, leak) {
			t.Errorf("the log contains %q:\n%s", leak, logs)
		}
	}
}
