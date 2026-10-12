package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const customKey = "k3y-0123456789"

// customAPI stands in for a JSON API that wants a key in a header.
func customAPI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != customKey {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"temp": 21.5, "list": [`+strings.Repeat(`{"n": 1},`, 40)+`{"n": 2}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func customBody(url, key string) string {
	return `{"name": "Thermometer", "url": "` + url + `/v1?unit=c", "method": "GET", "intervalSeconds": 60,
		"auth": {"kind": "header", "name": "X-API-Key", "secret": "` + key + `"},
		"headers": [{"name": "X-Tenant", "value": "` + key + `-tenant", "secret": true}, {"name": "Accept-Language", "value": "nl"}],
		"sample": {"temp": 20}}`
}

func TestCustomSourcesNeedLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, route := range [][2]string{
		{"GET", "/api/custom-sources"}, {"POST", "/api/custom-sources"}, {"PUT", "/api/custom-sources/1"},
		{"DELETE", "/api/custom-sources/1"}, {"POST", "/api/custom-sources/1/tests"}, {"POST", "/api/custom-rule-checks"},
	} {
		if status, _ := env.do(t, route[0], route[1], `{}`); status != http.StatusUnauthorized {
			t.Errorf("%s %s without login = %d, want 401", route[0], route[1], status)
		}
	}
}

func TestCustomSourceSecretsNeverLeaveTheServer(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	api := customAPI(t)

	status, body := env.do(t, "POST", "/api/custom-sources", customBody(api.URL, customKey))
	if status != http.StatusCreated || !strings.Contains(body, `"secretSet":true`) || !strings.Contains(body, `"sample":{"temp":20}`) {
		t.Fatalf("create = %d %s", status, body)
	}
	_, listed := env.do(t, "GET", "/api/custom-sources", "")
	_, settingsPage := env.do(t, "GET", "/api/settings", "")
	_, audit := env.do(t, "GET", "/api/audit-events", "")
	for what, text := range map[string]string{"create": body, "list": listed, "settings": settingsPage, "audit": audit, "log": env.logs.String()} {
		if strings.Contains(text, customKey) {
			t.Errorf("the %s holds the key: %s", what, text)
		}
	}
	if !strings.Contains(audit, "custom_source_added") || !strings.Contains(audit, "Thermometer (127.0.0.1") || strings.Contains(audit, "unit=c") {
		t.Errorf("audit = %s, want the name and host only", audit)
	}

	// Saving without the secrets keeps them: the test still gets in.
	update := strings.Replace(customBody(api.URL, ""), "Thermometer", "Inside", 1)
	if status, body := env.do(t, "PUT", "/api/custom-sources/1", update); status != http.StatusOK || !strings.Contains(body, `"name":"Inside"`) {
		t.Fatalf("update = %d %s", status, body)
	}
	status, body = env.do(t, "POST", "/api/custom-sources/1/tests", "")
	if status != http.StatusOK || !strings.Contains(body, `"temp":21.5`) || !strings.Contains(body, `"truncated":true`) {
		t.Fatalf("test = %d %s", status, body)
	}
	if n := env.settingsChanges.Load(); n != 2 {
		t.Errorf("SettingsChanged called %d times, want 2 (create, update)", n)
	}
	if status, _ := env.do(t, "DELETE", "/api/custom-sources/1", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := env.do(t, "POST", "/api/custom-sources/1/tests", ""); status != http.StatusNotFound {
		t.Fatalf("test of a deleted source = %d, want 404", status)
	}
}

func TestCustomSourceTestReportsWhatWentWrong(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	api := customAPI(t)
	if status, body := env.do(t, "POST", "/api/custom-sources", customBody(api.URL, "wrong-"+customKey)); status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, body)
	}
	status, body := env.do(t, "POST", "/api/custom-sources/1/tests", "")
	if status != http.StatusBadGateway || !strings.Contains(body, "status 401") || strings.Contains(body, customKey) {
		t.Fatalf("test with a wrong key = %d %s", status, body)
	}
	if strings.Contains(env.logs.String(), customKey) {
		t.Fatal("the log holds the key")
	}
}

func TestCustomSourceValidation(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/custom-sources",
		`{"name": "", "url": "ftp://x", "method": "GET", "intervalSeconds": 5, "auth": {"kind": "bearer", "secret": "`+customKey+` x"}}`)
	for _, field := range []string{"name", "url", "intervalSeconds", "auth.secret"} {
		if !strings.Contains(body, `"field":"`+field+`"`) {
			t.Errorf("missing field %s in %d %s", field, status, body)
		}
	}
	if status != http.StatusUnprocessableEntity || strings.Contains(body, customKey) {
		t.Fatalf("invalid source = %d %s", status, body)
	}
	if status, _ := env.do(t, "PUT", "/api/custom-sources/x", customBody("https://a", "k")); status != http.StatusNotFound {
		t.Errorf("PUT of a bad id = %d", status)
	}
	if status, _ := env.do(t, "PUT", "/api/custom-sources/9", customBody("https://a", "k")); status != http.StatusNotFound {
		t.Errorf("PUT of an unknown id = %d", status)
	}
	if status, _ := env.do(t, "POST", "/api/custom-sources", `{"name": "x", "extra": 1}`); status != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", status)
	}
}

func TestCustomRuleCheck(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	cases := map[string]string{
		`{"when": "value > 20 and ok", "value": 21, "data": {"ok": true}}`: `{"result":true}`,
		`{"when": "row.n * 2", "row": {"n": 4}}`:                           `{"result":8}`,
		`{"when": "data.missing > 1", "data": {}}`:                         `compares two numbers or two texts, not a missing value (null) and a number"}`,
	}
	for req, want := range cases {
		if status, body := env.do(t, "POST", "/api/custom-rule-checks", req); status != http.StatusOK || !strings.HasSuffix(strings.TrimSpace(body), want) {
			t.Errorf("%s = %d %s, want %s", req, status, body, want)
		}
	}
	status, body := env.do(t, "POST", "/api/custom-rule-checks", `{"when": "value >"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"when"`) {
		t.Errorf("unparsable rule = %d %s", status, body)
	}
}
