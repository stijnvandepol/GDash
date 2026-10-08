package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/display"
)

const morningLayout = `{"id":"morning","name":"Morning","widgets":[` +
	`{"id":"clock","type":"clock","x":0,"y":0,"w":8,"h":4},` +
	`{"id":"news","type":"news","x":8,"y":0,"w":10,"h":6,"settings":{"maxItems":4},"source":"secrets.everything"}]}`

func TestDashboardCRUD(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "POST", "/api/dashboards", morningLayout)
	if status != http.StatusCreated || !strings.Contains(body, `"source":"news.default"`) || strings.Contains(body, "secrets") {
		t.Fatalf("create = %d %s; want the source taken from the widget type", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/dashboards", morningLayout); status != http.StatusConflict {
		t.Fatalf("duplicate = %d, want 409", status)
	}

	overlap := `{"name":"Morning","widgets":[{"id":"a","type":"clock","x":0,"y":0,"w":8,"h":4},{"id":"b","type":"clock","x":4,"y":2,"w":8,"h":4}]}`
	status, body = env.do(t, "PUT", "/api/dashboards/morning", overlap)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"widgets[1]"`) {
		t.Fatalf("overlapping layout = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/dashboards/morning", `{"name":"Mornings","theme":"comic","widgets":[]}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"theme"`) {
		t.Fatalf("unknown theme = %d %s", status, body)
	}
	status, body = env.do(t, "PUT", "/api/dashboards/morning", `{"name":"Mornings","theme":"glass","widgets":[]}`)
	if status != http.StatusOK || !strings.Contains(body, `"widgets":[]`) || !strings.Contains(body, `"theme":"glass"`) {
		t.Fatalf("update = %d %s", status, body)
	}

	_, body = env.do(t, "GET", "/api/dashboards", "")
	var list struct{ Dashboards []display.Dashboard }
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Dashboards) != 1 || list.Dashboards[0].Name != "Mornings" {
		t.Fatalf("list = %s", body)
	}
	if status, _ := env.do(t, "DELETE", "/api/dashboards/morning", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := env.do(t, "GET", "/api/dashboards/morning", ""); status != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", status)
	}
}

func TestDefaultDashboardIsProtected(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if err := env.store.EnsureDashboard(t.Context(), display.DefaultDashboard(false), time.Now()); err != nil {
		t.Fatal(err)
	}
	if status, _ := env.do(t, "DELETE", "/api/dashboards/default", ""); status != http.StatusConflict {
		t.Fatalf("delete default = %d, want 409", status)
	}
}

func TestDisplayCanSwitchDashboard(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	if status, body := env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"missing"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"dashboardId"`) {
		t.Fatalf("unknown dashboard = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/dashboards", morningLayout); status != http.StatusCreated {
		t.Fatal("create dashboard failed")
	}
	if status, body := env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"morning"}`); status != http.StatusOK || !strings.Contains(body, `"dashboardId":"morning"`) {
		t.Fatalf("switch = %d %s", status, body)
	}
}

func TestDashboardRoutesNeedLoginAndWidgetTypesAreListed(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/dashboards", ""); status != http.StatusUnauthorized {
		t.Fatalf("list without login = %d", status)
	}
	if status, _ := env.do(t, "PUT", "/api/dashboards/default", `{"name":"x","widgets":[]}`); status != http.StatusUnauthorized {
		t.Fatalf("update without login = %d", status)
	}
	env.setUp(t)
	status, body := env.do(t, "GET", "/api/widget-types", "")
	if status != http.StatusOK || !strings.Contains(body, `"calendar":{"minW":6,"minH":4,"hasMaxItems":true}`) || !strings.Contains(body, `"columns":24`) || !strings.Contains(body, `"glass"`) {
		t.Fatalf("widget types = %d %s", status, body)
	}
}
