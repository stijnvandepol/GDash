package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/display"
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
		{"PUT", "/api/custom-sources/1/values"},
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

func TestCustomSourceValues(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/custom-sources", customBody("https://api.example.com", customKey)); status != http.StatusCreated || !strings.Contains(body, `"values":[],"lists":[]`) {
		t.Fatalf("create = %d %s", status, body)
	}
	values := `{"values": [{"name": "Inside", "path": "temp", "unit": "°C", "decimals": 1}],
		"lists": [{"name": "Rooms", "path": "list", "columns": [{"name": "Number", "path": "n"}]}]}`
	status, body := env.do(t, "PUT", "/api/custom-sources/1/values", values)
	if status != http.StatusOK || !strings.Contains(body, `"values":[{"name":"Inside","path":"temp","unit":"°C","decimals":1}]`) || strings.Contains(body, customKey) {
		t.Fatalf("set values = %d %s", status, body)
	}
	if _, listed := env.do(t, "GET", "/api/custom-sources", ""); !strings.Contains(listed, `"columns":[{"name":"Number","path":"n"}]`) {
		t.Errorf("list = %s, want the named list", listed)
	}
	if n := env.settingsChanges.Load(); n != 1 {
		t.Errorf("SettingsChanged called %d times, want 1 (create): names do not change fetching", n)
	}

	status, body = env.do(t, "PUT", "/api/custom-sources/1/values", `{"values": [{"name": "", "path": "a..b"}], "lists": []}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"values[0].name"`) || !strings.Contains(body, `"field":"values[0].path"`) {
		t.Fatalf("invalid values = %d %s", status, body)
	}
	if status, _ := env.do(t, "PUT", "/api/custom-sources/9/values", `{"values": []}`); status != http.StatusNotFound {
		t.Errorf("values of an unknown source = %d, want 404", status)
	}
}

const pushBody = `{"name": "Homey", "mode": "push", "intervalSeconds": 60}`

// pushEnv is logged in, with pushed source 1, fetched source 2 and an API
// token, whose secret it returns.
func pushEnv(t *testing.T) (*testEnv, string) {
	t.Helper()
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/custom-sources", pushBody)
	if status != http.StatusCreated || !strings.Contains(body, `"mode":"push"`) || !strings.Contains(body, `"url":""`) {
		t.Fatalf("create pushed source = %d %s", status, body)
	}
	if status, body := env.do(t, "POST", "/api/custom-sources", customBody("https://api.example.com", customKey)); status != http.StatusCreated || !strings.Contains(body, `"mode":"fetch"`) {
		t.Fatalf("create fetched source = %d %s", status, body)
	}
	_, secret := env.makeToken(t, "Homey")
	return env, secret
}

func TestCustomPushWithATokenOrALogin(t *testing.T) {
	env, secret := pushEnv(t)
	if status, body := env.do(t, "POST", "/api/custom-sources/1/tests", ""); status != http.StatusConflict || !strings.Contains(body, "nothing_received") {
		t.Fatalf("test before any data = %d %s, want 409", status, body)
	}
	_, auditBefore := env.do(t, "GET", "/api/audit-events", "")

	status, body := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{"home": {"Anna": true}, "power": 1200}`, secret, nil)
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("push with a token = %d %s, want 204", status, body)
	}
	status, body = env.do(t, "POST", "/api/custom-sources/1/tests", "")
	if status != http.StatusOK || !strings.Contains(body, `"power":1200`) || !strings.Contains(body, `"receivedAt":"2026-10-08T17:00:00Z"`) {
		t.Fatalf("last received = %d %s", status, body)
	}
	if _, listed := env.do(t, "GET", "/api/custom-sources", ""); !strings.Contains(listed, `"lastPushAt":"2026-10-08T17:00:00Z"`) ||
		strings.Count(listed, "lastPushAt") != 1 {
		t.Errorf("list = %s, want lastPushAt on the pushed source only", listed)
	}

	// An admin session may send data too, such as from a test in a browser
	// console; the browser's CSRF check still applies to it.
	env.advance(pushGap)
	if status, body := env.do(t, "PUT", "/api/custom-sources/1/data", `[1, 2]`); status != http.StatusNoContent {
		t.Fatalf("push with a session = %d %s", status, body)
	}
	if _, auditAfter := env.do(t, "GET", "/api/audit-events", ""); auditAfter != auditBefore {
		t.Errorf("a push was audited: %s", auditAfter)
	}

	env.advance(pushGap)
	for name, header := range map[string]string{"no credentials": "", "wrong token": "Bearer gdk_" + strings.Repeat("A", 43)} {
		headers := map[string]string{}
		if header != "" {
			headers["Authorization"] = header
		}
		if status, _ := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{}`, "", headers); status != http.StatusUnauthorized {
			t.Errorf("%s = %d, want 401", name, status)
		}
	}
	// A cross-site browser request never gets to the token check.
	if status, _ := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{}`, secret, map[string]string{"Sec-Fetch-Site": "cross-site"}); status != http.StatusForbidden {
		t.Errorf("cross-site push = %d, want 403", status)
	}
}

func TestCustomPushRefusals(t *testing.T) {
	env, secret := pushEnv(t)
	cases := []struct {
		name, path, body, contentType string
		status                        int
	}{
		{"a fetched source", "/api/custom-sources/2/data", `{}`, "application/json", http.StatusConflict},
		{"an unknown source", "/api/custom-sources/9/data", `{}`, "application/json", http.StatusNotFound},
		{"a bad id", "/api/custom-sources/x/data", `{}`, "application/json", http.StatusNotFound},
		{"over 1 MB", "/api/custom-sources/1/data", `"` + strings.Repeat("x", 1<<20) + `"`, "application/json", http.StatusRequestEntityTooLarge},
		{"not JSON", "/api/custom-sources/1/data", `{"a":`, "application/json", http.StatusUnprocessableEntity},
		{"empty", "/api/custom-sources/1/data", ``, "application/json", http.StatusUnprocessableEntity},
		{"text", "/api/custom-sources/1/data", `{}`, "text/plain", http.StatusUnsupportedMediaType},
	}
	for _, c := range cases {
		status, body := env.bearer(t, "PUT", c.path, c.body, secret, map[string]string{"Content-Type": c.contentType})
		if status != c.status {
			t.Errorf("%s = %d %s, want %d", c.name, status, body, c.status)
		}
	}
	// None of those counted: the first good answer is taken, the next one
	// within 5 seconds is not.
	if status, body := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{"n": 1}`, secret, nil); status != http.StatusNoContent {
		t.Fatalf("first push = %d %s", status, body)
	}
	env.advance(pushGap - time.Second)
	status, body := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{"n": 2}`, secret, nil)
	if status != http.StatusTooManyRequests || !strings.Contains(body, "rate_limited") {
		t.Fatalf("push within 5 seconds = %d %s, want 429", status, body)
	}
	env.advance(time.Second)
	if status, body := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{"n": 3}`, secret, nil); status != http.StatusNoContent {
		t.Fatalf("push after 5 seconds = %d %s", status, body)
	}
	if _, body := env.do(t, "POST", "/api/custom-sources/1/tests", ""); !strings.Contains(body, `"n":3`) {
		t.Fatalf("last received = %s, want the third answer", body)
	}
}

func TestCustomPushInputChecks(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/custom-sources",
		`{"name": "Homey", "mode": "push", "url": "https://x", "intervalSeconds": 60, "auth": {"kind": "bearer", "secret": "t"}}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) || !strings.Contains(body, `"field":"auth.kind"`) {
		t.Fatalf("pushed source with an address = %d %s, want 422 on url and auth.kind", status, body)
	}
	if status, body := env.do(t, "POST", "/api/custom-sources", pushBody); status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, body)
	}
	// Switching to fetching needs an address like any fetched source.
	status, body = env.do(t, "PUT", "/api/custom-sources/1", `{"name": "Homey", "mode": "fetch", "method": "GET", "intervalSeconds": 60, "auth": {"kind": "none"}}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("switch without an address = %d %s", status, body)
	}
}

func TestPushedDataReachesScreensNarrowedAndGoesStale(t *testing.T) {
	env, secret := pushEnv(t)
	layout := `{"id":"hall","name":"Hall","widgets":[{"id":"home","type":"custom","x":0,"y":0,"w":8,"h":4,"settings":{
		"source":1,"mode":"value","title":"Power","path":"power"}}]}`
	if status, body := env.do(t, "POST", "/api/dashboards", layout); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	if status, body := env.bearer(t, "PUT", "/api/custom-sources/1/data", `{"power": 1200, "alarmCode": "4711"}`, secret, nil); status != http.StatusNoContent {
		t.Fatalf("push = %d %s", status, body)
	}
	shown := func() string {
		status, body := env.do(t, "GET", "/api/dashboards/hall/preview", "")
		if status != http.StatusOK {
			t.Fatalf("preview = %d %s", status, body)
		}
		return string(decodeSnapshot(t, body).Data[display.CustomSource].Value)
	}
	fresh := shown()
	if !strings.Contains(fresh, `"value":1200`) || strings.Contains(fresh, "4711") || strings.Contains(fresh, `"stale"`) {
		t.Fatalf("screen gets %s, want only the power, fresh", fresh)
	}
	// Two intervals of a minute and 30 seconds later it is still current;
	// a moment after that it is stale.
	env.advance(150 * time.Second)
	if got := shown(); strings.Contains(got, `"stale"`) {
		t.Fatalf("after 2m30s = %s, want it still fresh", got)
	}
	env.advance(time.Second)
	if got := shown(); !strings.Contains(got, `"stale":true`) || !strings.Contains(got, `"value":1200`) {
		t.Fatalf("after 2m31s = %s, want the last value, stale", got)
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
