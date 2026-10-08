package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stijnvandepol/daycast/internal/auth"
	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/store"
)

const testSetupToken = "test-setup-token"

type fixedSubscriber []byte

func (f fixedSubscriber) Subscribe(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, 1)
	ch <- f
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

type testEnv struct {
	srv    *httptest.Server
	client *http.Client
}

func newTestEnv(t *testing.T, discover func(context.Context) ([]cast.Device, error)) *testEnv {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "daycast.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	authService, err := auth.NewService(st, testSetupToken)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		Snapshots: fixedSubscriber(`{"type":"snapshot","version":1}`),
		Auth:      authService,
		Store:     st,
		Web: fstest.MapFS{
			"admin/index.html":    {Data: []byte("<html>admin</html>")},
			"display/index.html":  {Data: []byte("<html>display</html>")},
			"receiver/index.html": {Data: []byte("<html>receiver</html>")},
			"assets/app.js":       {Data: []byte("console.log(1)")},
		},
		Discover: discover,
		Now:      func() time.Time { return time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC) },
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &testEnv{srv: srv, client: &http.Client{Jar: jar}}
}

// do sends a request with a JSON body and returns status and body.
func (e *testEnv) do(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *testEnv) setUp(t *testing.T) {
	t.Helper()
	status, body := e.do(t, "POST", "/api/setup", `{"setupToken":"`+testSetupToken+`","username":"stijn","password":"correct horse battery"}`)
	if status != http.StatusCreated {
		t.Fatalf("setup = %d %s", status, body)
	}
}

func TestDisplayPageHasStrictCSP(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Get(env.srv.URL + "/d/keuken")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("CSP = %q", csp)
	}
}

func TestAdminPageIsServedAtRoot(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, body := env.do(t, "GET", "/", ""); status != http.StatusOK || body != "<html>admin</html>" {
		t.Fatalf("GET / = %d %q", status, body)
	}
	if status, _ := env.do(t, "GET", "/does-not-exist", ""); status != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist = %d, want 404", status)
	}
}

func TestInvalidDisplayIDIsNotFound(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, path := range []string{"/d/Keuken", "/d/..%2fetc", "/api/displays/a_b/events", "/d/" + strings.Repeat("a", 40)} {
		if status, _ := env.do(t, "GET", path, ""); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

func TestEventStreamSendsSnapshot(t *testing.T) {
	env := newTestEnv(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, env.srv.URL+"/api/displays/keuken/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if strings.HasPrefix(scanner.Text(), "data: ") {
			break
		}
	}
	got := strings.Join(lines, "\n")
	want := "retry: 5000\n\nevent: snapshot\ndata: {\"type\":\"snapshot\",\"version\":1}"
	if got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

func TestSetupThenLoginLogout(t *testing.T) {
	env := newTestEnv(t, nil)
	if _, body := env.do(t, "GET", "/api/setup", ""); body != "{\"needed\":true}\n" {
		t.Fatalf("setup state before = %s", body)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("/api/me before setup = %d, want 401", status)
	}
	env.setUp(t)
	if status, body := env.do(t, "GET", "/api/me", ""); status != http.StatusOK || !strings.Contains(body, `"username":"stijn"`) {
		t.Fatalf("/api/me after setup = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/setup", `{"setupToken":"`+testSetupToken+`","username":"x","password":"another long password"}`); status != http.StatusConflict {
		t.Fatalf("second setup = %d, want 409", status)
	}

	if status, _ := env.do(t, "DELETE", "/api/sessions/current", ""); status != http.StatusNoContent {
		t.Fatalf("logout = %d", status)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("/api/me after logout = %d, want 401", status)
	}
	if status, _ := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"wrong password!"}`); status != http.StatusUnauthorized {
		t.Fatalf("login with wrong password = %d, want 401", status)
	}
	if status, _ := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"correct horse battery"}`); status != http.StatusCreated {
		t.Fatalf("login = %d", status)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusOK {
		t.Fatalf("/api/me after login = %d", status)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Post(env.srv.URL+"/api/setup", "application/json",
		strings.NewReader(`{"setupToken":"`+testSetupToken+`","username":"stijn","password":"correct horse battery"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	header := resp.Header.Get("Set-Cookie")
	for _, want := range []string{"daycast_session=", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(header, want) {
			t.Errorf("Set-Cookie %q lacks %q", header, want)
		}
	}
}

func TestSetupRejectsWrongToken(t *testing.T) {
	env := newTestEnv(t, nil)
	status, body := env.do(t, "POST", "/api/setup", `{"setupToken":"guess","username":"stijn","password":"correct horse battery"}`)
	if status != http.StatusForbidden || !strings.Contains(body, "invalid_setup_token") {
		t.Fatalf("setup with wrong token = %d %s", status, body)
	}
}

func TestRequestBodyIsStrict(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, body := range []string{`{"username":"stijn","passwrd":"x"}`, `not json`, `{"username":"a"}{"username":"b"}`} {
		if status, _ := env.do(t, "POST", "/api/sessions", body); status != http.StatusBadRequest {
			t.Errorf("POST /api/sessions %s = %d, want 400", body, status)
		}
	}
}

func TestCrossSiteRequestIsRejected(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	req, _ := http.NewRequest("POST", env.srv.URL+"/api/displays", strings.NewReader(`{"id":"keuken","name":"Keuken"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", resp.StatusCode)
	}
}

func TestDisplayAPIRequiresLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/displays", ""},
		{"POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`},
		{"PATCH", "/api/displays/keuken", `{"name":"x"}`},
		{"DELETE", "/api/displays/keuken", ""},
		{"GET", "/api/cast-devices", ""},
	} {
		if status, _ := env.do(t, r.method, r.path, r.body); status != http.StatusUnauthorized {
			t.Errorf("%s %s without login = %d, want 401", r.method, r.path, status)
		}
	}
}

func TestDisplayCRUD(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken TV","castDeviceId":"e542c41e4d94baa86e46f36dfdebf26e"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"createdAt":"2026-10-08T17:00:00Z"`) {
		t.Fatalf("create = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Dubbel"}`); status != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409", status)
	}
	status, body = env.do(t, "POST", "/api/displays", `{"id":"Bad ID","name":""}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"id"`) || !strings.Contains(body, `"field":"name"`) {
		t.Fatalf("invalid create = %d %s", status, body)
	}

	status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"name":"Keuken"}`)
	if status != http.StatusOK || !strings.Contains(body, `"name":"Keuken"`) || !strings.Contains(body, "e542c41e") {
		t.Fatalf("rename = %d %s (cast device must stay)", status, body)
	}
	if status, _ := env.do(t, "PATCH", "/api/displays/bestaat-niet", `{"name":"x"}`); status != http.StatusNotFound {
		t.Fatalf("patch unknown = %d, want 404", status)
	}

	_, body = env.do(t, "GET", "/api/displays", "")
	var list struct {
		Displays []struct{ ID, Name string }
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Displays) != 1 || list.Displays[0].Name != "Keuken" {
		t.Fatalf("list = %s (err %v)", body, err)
	}

	if status, _ := env.do(t, "DELETE", "/api/displays/keuken", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := env.do(t, "DELETE", "/api/displays/keuken", ""); status != http.StatusNotFound {
		t.Fatalf("delete again = %d, want 404", status)
	}
}

func TestCastDevicesListsDiscoveredDevices(t *testing.T) {
	env := newTestEnv(t, func(context.Context) ([]cast.Device, error) {
		return []cast.Device{{ID: "abc", Name: "Keuken TV", Addr: "192.168.1.40:8009"}}, nil
	})
	env.setUp(t)
	status, body := env.do(t, "GET", "/api/cast-devices", "")
	if status != http.StatusOK || !strings.Contains(body, `"name":"Keuken TV"`) {
		t.Fatalf("status %d body %s", status, body)
	}
}

func TestCastDevicesErrorHasStableShape(t *testing.T) {
	env := newTestEnv(t, func(context.Context) ([]cast.Device, error) {
		return nil, errors.New("socket: permission denied at /proc/net")
	})
	env.setUp(t)
	status, body := env.do(t, "GET", "/api/cast-devices", "")
	if status != http.StatusInternalServerError || !strings.Contains(body, `"code":"discovery_failed"`) {
		t.Fatalf("status %d body %s", status, body)
	}
	if strings.Contains(body, "/proc") {
		t.Fatalf("internal error details leaked: %s", body)
	}
}
