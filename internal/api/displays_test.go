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
	if status != http.StatusOK || !strings.Contains(body, `"schedule":[{"from":"06:30","until":"09:00","days":[],"dashboardId":"morning"}]`) {
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

	weekend := `{"schedule":[{"from":"08:00","until":"12:00","days":["sat","sun"],"dashboardId":"morning"}]}`
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", weekend); status != http.StatusOK || !strings.Contains(body, `"days":["sat","sun"]`) {
		t.Fatalf("weekend slot = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"schedule":[{"from":"08:00","until":"12:00","days":["zondag"],"dashboardId":"morning"}]}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown day = %d %s", status, body)
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

func TestDisplayRotation(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"news","name":"News","widgets":[]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"rotation":[],"rotateSeconds":0`) {
		t.Fatalf("create = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"rotation":["news"],"rotateSeconds":30}`); status != http.StatusOK || !strings.Contains(body, `"rotation":["news"],"rotateSeconds":30`) {
		t.Fatalf("set rotation = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"rotation":["missing"]}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"rotation"`) {
		t.Fatalf("unknown dashboard = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"rotation":[],"rotateSeconds":0}`); status != http.StatusOK {
		t.Fatalf("clear = %d %s", status, body)
	}
}

func TestDisplayTextScale(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"textScale":100`) {
		t.Fatalf("create = %d %s; want the default text size", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"textScale":150}`); status != http.StatusOK || !strings.Contains(body, `"textScale":150`) {
		t.Fatalf("larger = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"textScale":175}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"textScale"`) {
		t.Fatalf("odd size = %d %s", status, body)
	}
}
