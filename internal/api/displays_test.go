package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestDisplayActiveHours(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken","activeFrom":"06:30","activeUntil":"23:00"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"activeFrom":"06:30"`) || !strings.Contains(body, `"activeUntil":"23:00"`) {
		t.Fatalf("create = %d %s", status, body)
	}

	// A half-filled window is rejected with a field error.
	status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"activeUntil":""}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"activeHours"`) {
		t.Fatalf("half window = %d %s", status, body)
	}

	// Clearing both means always on.
	status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"activeFrom":"","activeUntil":""}`)
	if status != http.StatusOK || !strings.Contains(body, `"activeFrom":""`) {
		t.Fatalf("clear = %d %s", status, body)
	}
	_, list := env.do(t, "GET", "/api/displays", "")
	if !strings.Contains(list, `"activeFrom":"","activeUntil":""`) {
		t.Fatalf("list = %s", list)
	}
}
