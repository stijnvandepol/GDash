package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if !strings.Contains(settingsBody, `"homey":{"url":"`+hy.URL+`"`) || !strings.Contains(audit, "homey_connected") {
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
