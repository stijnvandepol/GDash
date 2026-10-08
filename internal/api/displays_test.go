package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/cast"
	"github.com/stijnvandepol/daycast/internal/casting"
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

func TestDisplaySchedule(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"morning","name":"Morning","widgets":[]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"schedule":[]`) {
		t.Fatalf("create = %d %s, want an empty schedule", status, body)
	}

	slot := `{"schedule":[{"from":"06:30","until":"09:00","dashboardId":"morning"}]}`
	status, body = env.do(t, "PATCH", "/api/displays/keuken", slot)
	if status != http.StatusOK || !strings.Contains(body, `"schedule":[{"from":"06:30","until":"09:00","dashboardId":"morning"}]`) {
		t.Fatalf("set schedule = %d %s", status, body)
	}

	// Changing another field keeps the schedule.
	if _, body = env.do(t, "PATCH", "/api/displays/keuken", `{"name":"Kitchen"}`); !strings.Contains(body, `"dashboardId":"morning"}]`) {
		t.Fatalf("rename dropped the schedule: %s", body)
	}

	for _, tt := range []struct{ body, field string }{
		{`{"schedule":[{"from":"06:30","until":"","dashboardId":"morning"}]}`, `"field":"schedule[0]"`},
		{`{"schedule":[{"from":"06:30","until":"09:00","dashboardId":"missing"}]}`, `"field":"schedule"`},
	} {
		if status, body := env.do(t, "PATCH", "/api/displays/keuken", tt.body); status != http.StatusUnprocessableEntity || !strings.Contains(body, tt.field) {
			t.Fatalf("PATCH %s = %d %s, want 422 with %s", tt.body, status, body, tt.field)
		}
	}

	if _, body = env.do(t, "PATCH", "/api/displays/keuken", `{"schedule":[]}`); !strings.Contains(body, `"schedule":[]`) {
		t.Fatalf("clear = %s", body)
	}
}

func TestDisplayListShowsCastStatus(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	since := time.Date(2026, 10, 9, 7, 0, 0, 0, time.UTC)
	env.sources.mu.Lock()
	env.sources.cast = map[string]casting.Status{"keuken": {State: cast.StateStandby, Since: since}}
	env.sources.mu.Unlock()
	_, body := env.do(t, "GET", "/api/displays", "")
	if !strings.Contains(body, `"cast":{"state":"standby","since":"2026-10-09T07:00:00Z"}`) {
		t.Fatalf("list = %s", body)
	}
}
