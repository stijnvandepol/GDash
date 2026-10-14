package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/casting"
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

func TestCastTakeoverAsksTheKeeperToShowTheDashboard(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	tv := `{"id":"woonkamer","name":"Woonkamer","castDeviceId":"0123456789abcdef0123456789abcdef"}`
	if status, body := env.do(t, "POST", "/api/displays", tv); status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, body)
	}

	if status, body := env.do(t, "POST", "/api/displays/keuken/cast-takeovers", ""); status != http.StatusConflict || !strings.Contains(body, "not_cast") {
		t.Fatalf("takeover of a browser display = %d %s, want 409 not_cast", status, body)
	}
	if status, body := env.do(t, "POST", "/api/displays/woonkamer/cast-takeovers", ""); status != http.StatusConflict || !strings.Contains(body, "not_connected") {
		t.Fatalf("takeover before the keeper runs = %d %s, want 409 not_connected", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/displays/zolder/cast-takeovers", ""); status != http.StatusNotFound {
		t.Fatalf("takeover of an unknown display = %d, want 404", status)
	}

	env.sources.mu.Lock()
	env.sources.cast = map[string]casting.Status{"woonkamer": {State: cast.StateYielded}}
	env.sources.mu.Unlock()
	if status, body := env.do(t, "POST", "/api/displays/woonkamer/cast-takeovers", ""); status != http.StatusNoContent {
		t.Fatalf("takeover = %d %s, want 204", status, body)
	}
	env.sources.mu.Lock()
	takeOvers := env.sources.takeOvers
	env.sources.mu.Unlock()
	if len(takeOvers) != 1 || takeOvers[0] != "woonkamer" {
		t.Fatalf("keeper requests = %v, want one for woonkamer", takeOvers)
	}
	if _, body := env.do(t, "GET", "/api/audit-events", ""); !strings.Contains(body, `"action":"cast_taken_over","detail":"woonkamer"`) {
		t.Fatalf("audit log lacks the takeover:\n%s", body)
	}

	req, _ := http.NewRequest("POST", env.srv.URL+"/api/displays/woonkamer/cast-takeovers", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site takeover = %d, want 403", resp.StatusCode)
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

func TestDisplayScreensaverSetting(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"screensaverMinutes":15`) {
		t.Fatalf("create = %d %s; want restarts every 15 minutes", status, body)
	}
	if status, body = env.do(t, "POST", "/api/displays", `{"id":"hal","name":"Hal","screensaverMinutes":0}`); status != http.StatusCreated || !strings.Contains(body, `"screensaverMinutes":0`) {
		t.Fatalf("create without restarts = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"screensaverMinutes":5}`); status != http.StatusOK || !strings.Contains(body, `"screensaverMinutes":5`) {
		t.Fatalf("every 5 minutes = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"screensaverMinutes":7}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"screensaverMinutes"`) {
		t.Fatalf("every 7 minutes = %d %s", status, body)
	}
	if _, body = env.do(t, "GET", "/api/displays", ""); !strings.Contains(body, `"screensaverMinutes":5`) {
		t.Fatalf("list = %s, want the stored setting", body)
	}
}

func TestDisplayScreenCareAndDimming(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"screenCare":"off"`) || !strings.Contains(body, `"dimAtNight":"off"`) {
		t.Fatalf("create = %d %s; want screen care and dimming off", status, body)
	}
	if status, body = env.do(t, "POST", "/api/displays", `{"id":"slaapkamer","name":"Slaapkamer","screenCare":"oled","dimAtNight":"sunset"}`); status != http.StatusCreated ||
		!strings.Contains(body, `"screenCare":"oled"`) || !strings.Contains(body, `"dimAtNight":"sunset"`) {
		t.Fatalf("create with care = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"screenCare":"oled"}`); status != http.StatusOK || !strings.Contains(body, `"screenCare":"oled"`) {
		t.Fatalf("oled care = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"dimAtNight":"hours","dimFrom":"22:30","dimUntil":"06:45"}`); status != http.StatusOK ||
		!strings.Contains(body, `"dimFrom":"22:30"`) || !strings.Contains(body, `"screenCare":"oled"`) {
		t.Fatalf("dim during hours = %d %s; want the hours, with the care kept", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"dimFrom":"","dimUntil":""}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"dimFrom"`) {
		t.Fatalf("dim during no hours = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"screenCare":"plasma"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"screenCare"`) {
		t.Fatalf("unknown care = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"dimAtNight":"always"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"dimAtNight"`) {
		t.Fatalf("unknown dimming = %d %s", status, body)
	}
	if status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"dimAtNight":"off"}`); status != http.StatusOK || !strings.Contains(body, `"dimUntil":"06:45"`) {
		t.Fatalf("dimming off = %d %s; want the hours kept for next time", status, body)
	}
	if _, body = env.do(t, "GET", "/api/displays", ""); !strings.Contains(body, `"screenCare":"oled"`) || !strings.Contains(body, `"dimFrom":"22:30"`) {
		t.Fatalf("list = %s, want the stored settings", body)
	}
}

func TestNewDisplayShowsTheChosenDashboard(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"kitchen","name":"Kitchen","widgets":[]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken","dashboardId":"kitchen"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"dashboardId":"kitchen"`) {
		t.Fatalf("create = %d %s", status, body)
	}
	status, body = env.do(t, "POST", "/api/displays", `{"id":"hal","name":"Hal","dashboardId":"missing"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"dashboardId"`) {
		t.Fatalf("unknown dashboard = %d %s, want 422 on dashboardId", status, body)
	}
	if status, body = env.do(t, "POST", "/api/displays", `{"id":"zolder","name":"Zolder"}`); status != http.StatusCreated || !strings.Contains(body, `"dashboardId":""`) {
		t.Fatalf("without a dashboard = %d %s, want the default", status, body)
	}
}
