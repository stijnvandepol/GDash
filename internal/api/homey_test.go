package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/display"
)

const homeyTestKey = "homey-made-up-key-0123456789"

// fakeHomey answers the calls the web interface makes. withUsers is false
// for a key without View users.
func fakeHomey(t *testing.T, withUsers bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+homeyTestKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/manager/zones/zone":
			w.Write([]byte(`{"z1":{"id":"z1","name":"Kitchen"}}`))
		case "/api/manager/devices/device":
			w.Write([]byte(`{"d1":{"id":"d1","name":"Thermostat","zone":"z1","available":true,"capabilitiesObj":{
				"measure_temperature":{"id":"measure_temperature","title":"Temperature","value":21.5,"units":"°C","getable":true}}}}`))
		case "/api/manager/users/user":
			if !withUsers {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Write([]byte(`{"u1":{"id":"u1","name":"Anna","present":true}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHomeyConnection(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	hy := fakeHomey(t, true)

	for _, path := range []string{"/api/homey/capabilities", "/api/homey/users"} {
		if status, body := env.do(t, "GET", path, ""); status != http.StatusConflict {
			t.Fatalf("%s before connecting = %d %s, want 409", path, status, body)
		}
	}
	status, body := env.do(t, "PUT", "/api/homey", `{"url":"`+hy.URL+`","key":"wrong-key"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"key"`) {
		t.Fatalf("wrong key = %d %s, want 422 on key", status, body)
	}
	status, body = env.do(t, "PUT", "/api/homey", `{"url":"http://127.0.0.1:1","key":"`+homeyTestKey+`"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("unreachable address = %d %s, want 422 on url", status, body)
	}
	status, body = env.do(t, "PUT", "/api/homey", `{"url":"192.168.1.20","key":"two words"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) || !strings.Contains(body, `"field":"key"`) {
		t.Fatalf("invalid input = %d %s, want 422 on url and key", status, body)
	}
	if env.settingsChanges.Load() != 0 {
		t.Fatal("a failed connection test changed the settings")
	}

	status, body = env.do(t, "PUT", "/api/homey", `{"url":"`+hy.URL+`/","key":"`+homeyTestKey+`"}`)
	if status != http.StatusOK || !strings.Contains(body, `"url":"`+hy.URL+`"`) {
		t.Fatalf("connect = %d %s", status, body)
	}
	_, settingsBody := env.do(t, "GET", "/api/settings", "")
	_, audit := env.do(t, "GET", "/api/audit-events", "")
	for name, text := range map[string]string{"connect answer": body, "settings": settingsBody, "audit log": audit} {
		if strings.Contains(text, homeyTestKey) {
			t.Fatalf("the %s contains the key: %s", name, text)
		}
	}
	if !strings.Contains(settingsBody, `"homey":{"mode":"local","url":"`+hy.URL+`"`) || !strings.Contains(audit, "homey_connected") {
		t.Fatalf("settings = %s, audit = %s; want the connection's address and its event", settingsBody, audit)
	}

	status, body = env.do(t, "GET", "/api/homey/capabilities", "")
	if status != http.StatusOK || !strings.Contains(body, `"key":"d1/measure_temperature"`) || !strings.Contains(body, `"zone":"Kitchen"`) {
		t.Fatalf("capabilities = %d %s", status, body)
	}
	status, body = env.do(t, "GET", "/api/homey/users", "")
	if status != http.StatusOK || body != `{"users":[{"id":"u1","name":"Anna"}]}`+"\n" {
		t.Fatalf("users = %d %q; want the user without whether she is home", status, body)
	}

	if status, _ := env.do(t, "DELETE", "/api/homey", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if _, body := env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"homey":null`) {
		t.Fatalf("settings after delete = %s", body)
	}
}

func TestHomeyUsersNeedTheirPermission(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	hy := fakeHomey(t, false)
	if status, body := env.do(t, "PUT", "/api/homey", `{"url":"`+hy.URL+`","key":"`+homeyTestKey+`"}`); status != http.StatusOK {
		t.Fatalf("connect without View users = %d %s, want 200: users are only needed for Who's home", status, body)
	}
	status, body := env.do(t, "GET", "/api/homey/users", "")
	if status != http.StatusBadGateway || !strings.Contains(body, "View users") {
		t.Fatalf("users = %d %s, want 502 that names the permission", status, body)
	}
}

func TestHomeyNeedsLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.do(t, "DELETE", "/api/sessions/current", "")
	for _, route := range []string{"PUT /api/homey", "DELETE /api/homey", "GET /api/homey/capabilities", "GET /api/homey/users"} {
		method, path, _ := strings.Cut(route, " ")
		if status, _ := env.do(t, method, path, `{}`); status != http.StatusUnauthorized {
			t.Errorf("%s without login = %d, want 401", route, status)
		}
	}
}

// homeySnapshot is what the HomeyScript from Integrations, Homey sends.
const homeySnapshot = `{"at": "2026-10-08T17:00:00.000Z",
	"devices": [
		{"id": "d1", "name": "Thermostat", "zone": "Kitchen", "available": true,
		 "caps": {"measure_temperature": {"title": "Temperature", "value": 21.5, "units": "°C"}}},
		{"id": "d2", "name": "Front door", "zone": "Hall", "available": true,
		 "caps": {"alarm_contact": {"title": "Contact", "value": true, "units": null}}}
	],
	"users": [{"id": "u1", "name": "Anna", "present": true, "asleep": false}, {"id": "u2", "name": "Bram", "present": false, "asleep": true}]}`

// homeyPushEnv is logged in, with Homey set to send its data and an API
// token, whose secret it returns.
func homeyPushEnv(t *testing.T) (*testEnv, string) {
	t.Helper()
	env := newTestEnv(t, nil)
	env.setUp(t)
	_, secret := env.makeToken(t, "HomeyScript")
	return env, secret
}

func TestHomeyPush(t *testing.T) {
	env, secret := homeyPushEnv(t)
	if status, body := env.bearer(t, "PUT", "/api/homey/snapshot", homeySnapshot, secret, nil); status != http.StatusConflict || !strings.Contains(body, "not_pushed") {
		t.Fatalf("snapshot before push mode = %d %s, want 409", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/homey", `{"mode": "push", "url": "http://192.168.1.20"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("push with an address = %d %s, want 422 on url", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/homey", `{"mode": "push"}`); status != http.StatusOK || !strings.Contains(body, `"mode":"push"`) {
		t.Fatalf("push mode = %d %s", status, body)
	}
	if _, body := env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"homey":{"mode":"push","url":""`) {
		t.Fatalf("settings = %s, want Homey in push mode", body)
	}
	if status, body := env.do(t, "GET", "/api/homey/capabilities", ""); status != http.StatusConflict || !strings.Contains(body, "nothing_received") {
		t.Fatalf("capabilities before a snapshot = %d %s, want 409", status, body)
	}

	if status, body := env.bearer(t, "PUT", "/api/homey/snapshot", homeySnapshot, secret, nil); status != http.StatusNoContent {
		t.Fatalf("snapshot = %d %s, want 204", status, body)
	}
	status, body := env.do(t, "GET", "/api/homey/capabilities", "")
	if status != http.StatusOK || !strings.Contains(body, `"key":"d1/measure_temperature","zone":"Kitchen"`) || !strings.Contains(body, `"value":21.5`) {
		t.Fatalf("capabilities from the snapshot = %d %s", status, body)
	}
	status, body = env.do(t, "GET", "/api/homey/users", "")
	if status != http.StatusOK || body != `{"users":[{"id":"u1","name":"Anna"},{"id":"u2","name":"Bram"}]}`+"\n" {
		t.Fatalf("users from the snapshot = %d %q; want them without whether they are home", status, body)
	}
	if status, _ := env.bearer(t, "PUT", "/api/homey/snapshot", homeySnapshot, secret, nil); status != http.StatusTooManyRequests {
		t.Fatalf("a second snapshot at once = %d, want 429", status)
	}

	env.advance(pushGap)
	cases := []struct {
		name, body, contentType, secret string
		status                          int
	}{
		{"no token", homeySnapshot, "application/json", "", http.StatusUnauthorized},
		{"not a snapshot", `{"home": {"Anna": true}}`, "application/json", secret, http.StatusUnprocessableEntity},
		{"not JSON", `{"devices": [`, "application/json", secret, http.StatusUnprocessableEntity},
		{"over 1 MB", `{"devices": [], "pad": "` + strings.Repeat("x", 1<<20) + `"}`, "application/json", secret, http.StatusRequestEntityTooLarge},
		{"text", homeySnapshot, "text/plain", secret, http.StatusUnsupportedMediaType},
	}
	for _, c := range cases {
		if status, body := env.bearer(t, "PUT", "/api/homey/snapshot", c.body, c.secret, map[string]string{"Content-Type": c.contentType}); status != c.status {
			t.Errorf("%s = %d %s, want %d", c.name, status, body, c.status)
		}
	}
	// None of those counted against the limit.
	if status, body := env.bearer(t, "PUT", "/api/homey/snapshot", homeySnapshot, secret, nil); status != http.StatusNoContent {
		t.Fatalf("snapshot after 5 seconds = %d %s", status, body)
	}
	if _, audit := env.do(t, "GET", "/api/audit-events", ""); strings.Count(audit, "homey_connected") != 1 {
		t.Errorf("audit = %s, want the switch to push mode only, no snapshots", audit)
	}
}

func TestHomeySnapshotReachesScreensNarrowedAndGoesStale(t *testing.T) {
	env, secret := homeyPushEnv(t)
	if status, body := env.do(t, "PUT", "/api/homey", `{"mode": "push"}`); status != http.StatusOK {
		t.Fatalf("push mode = %d %s", status, body)
	}
	hall := `{"id":"hall","name":"Hall","widgets":[
		{"id":"temp","type":"homey","x":0,"y":0,"w":8,"h":4,"settings":{"capabilities":["d1/measure_temperature"]}},
		{"id":"home","type":"homeypresence","x":8,"y":0,"w":8,"h":4,"settings":{"users":[{"id":"u1","label":"AN"},{"id":"u2","label":"BR"}]}}]}`
	living := `{"id":"living","name":"Living","widgets":[
		{"id":"door","type":"homey","x":0,"y":0,"w":8,"h":4,"settings":{"capabilities":["d2/alarm_contact"]}}]}`
	for _, layout := range []string{hall, living} {
		if status, body := env.do(t, "POST", "/api/dashboards", layout); status != http.StatusCreated {
			t.Fatalf("create dashboard = %d %s", status, body)
		}
	}
	if status, body := env.bearer(t, "PUT", "/api/homey/snapshot", homeySnapshot, secret, nil); status != http.StatusNoContent {
		t.Fatalf("snapshot = %d %s", status, body)
	}
	status, body := env.do(t, "GET", "/api/dashboards/hall/preview", "")
	if status != http.StatusOK {
		t.Fatalf("preview = %d %s", status, body)
	}
	snap := decodeSnapshot(t, body)
	values := string(snap.Data[display.HomeySource].Value)
	if !strings.Contains(values, `"d1/measure_temperature"`) || strings.Contains(values, "Front door") {
		t.Errorf("hall values = %s, want the thermostat only, not the living room's door", values)
	}
	if got := string(snap.Data[display.HomeyPresenceSource].Value); got != `{"hall/home":["AN"]}` {
		t.Errorf("hall presence = %s, want only the label of who is home", got)
	}
	for _, key := range []string{display.HomeySnapshotSource, display.HomeyUsersSource} {
		if _, sent := snap.Data[key]; sent {
			t.Errorf("a screen received %s", key)
		}
	}
	if strings.Contains(body, "Bram") || strings.Contains(body, `"u2"`) {
		t.Errorf("a screen learned who is away: %s", body)
	}
	// Three minutes without a snapshot and the widgets show that it is old.
	received := time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)
	if got := snap.Data[display.HomeySource].StaleAfter; !got.Equal(received.Add(3 * time.Minute)) {
		t.Errorf("values stale after %v, want three minutes after the snapshot", got)
	}
	if got := snap.Data[display.HomeyPresenceSource].StaleAfter; !got.Equal(received.Add(3 * time.Minute)) {
		t.Errorf("presence stale after %v, want three minutes after the snapshot", got)
	}
}
