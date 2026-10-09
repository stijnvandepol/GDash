package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const haTestToken = "ha-long-lived-token-0123456789"

// fakeHomeAssistant answers the two calls the web interface makes.
func fakeHomeAssistant(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+haTestToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/":
			w.Write([]byte(`{"message":"API running."}`))
		case "/api/states":
			w.Write([]byte(`[{"entity_id":"sensor.temp","state":"21.5","attributes":{"friendly_name":"Living room","unit_of_measurement":"°C"}}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHomeAssistantConnection(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	ha := fakeHomeAssistant(t)

	if status, body := env.do(t, "GET", "/api/homeassistant/entities", ""); status != http.StatusConflict {
		t.Fatalf("entities before connecting = %d %s, want 409", status, body)
	}
	status, body := env.do(t, "PUT", "/api/homeassistant", `{"url":"`+ha.URL+`","token":"wrong-token"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"token"`) {
		t.Fatalf("wrong token = %d %s, want 422 on token", status, body)
	}
	status, body = env.do(t, "PUT", "/api/homeassistant", `{"url":"http://127.0.0.1:1","token":"`+haTestToken+`"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("unreachable address = %d %s, want 422 on url", status, body)
	}
	if env.settingsChanges.Load() != 0 {
		t.Fatal("a failed connection test changed the settings")
	}

	status, body = env.do(t, "PUT", "/api/homeassistant", `{"url":"`+ha.URL+`/","token":"`+haTestToken+`"}`)
	if status != http.StatusOK || !strings.Contains(body, `"url":"`+ha.URL+`"`) {
		t.Fatalf("connect = %d %s", status, body)
	}
	_, settingsBody := env.do(t, "GET", "/api/settings", "")
	_, audit := env.do(t, "GET", "/api/audit-events", "")
	for name, text := range map[string]string{"connect answer": body, "settings": settingsBody, "audit log": audit} {
		if strings.Contains(text, haTestToken) {
			t.Fatalf("the %s contains the token: %s", name, text)
		}
	}
	if !strings.Contains(settingsBody, `"homeAssistant":{"url":"`+ha.URL+`"`) {
		t.Fatalf("settings = %s, want the connection's address", settingsBody)
	}

	status, body = env.do(t, "GET", "/api/homeassistant/entities", "")
	if status != http.StatusOK || !strings.Contains(body, `"id":"sensor.temp"`) || !strings.Contains(body, `"name":"Living room"`) {
		t.Fatalf("entities = %d %s", status, body)
	}

	if status, _ := env.do(t, "DELETE", "/api/homeassistant", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if _, body := env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"homeAssistant":null`) {
		t.Fatalf("settings after delete = %s", body)
	}
}

func TestHomeAssistantNeedsLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.do(t, "DELETE", "/api/sessions/current", "")
	for _, route := range []string{"PUT /api/homeassistant", "DELETE /api/homeassistant", "GET /api/homeassistant/entities"} {
		method, path, _ := strings.Cut(route, " ")
		if status, _ := env.do(t, method, path, `{}`); status != http.StatusUnauthorized {
			t.Errorf("%s without login = %d, want 401", route, status)
		}
	}
}
