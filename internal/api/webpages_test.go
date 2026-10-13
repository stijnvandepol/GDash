package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

const grafanaPage = `{"name":"Grafana","url":"http://192.168.1.10:3000/d/abc?kiosk","refreshMinutes":15,"zoom":80}`

func (e *testEnv) addWebPage(t *testing.T, body string) webpage.Page {
	t.Helper()
	status, resp := e.do(t, "POST", "/api/web-pages", body)
	if status != http.StatusCreated {
		t.Fatalf("add web page = %d %s", status, resp)
	}
	var page webpage.Page
	if err := json.Unmarshal([]byte(resp), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

// pairScreen pairs a new screen with a display by its code and returns it.
func (e *testEnv) pairScreen(t *testing.T, displayID string) *http.Client {
	t.Helper()
	screen := tv()
	_, body := post(t, screen, e.srv.URL+"/api/pairing-requests", `{"displayId":"`+displayID+`"}`)
	var req pairing.Request
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	if status, body := e.do(t, "POST", "/api/displays/"+displayID+"/pairings", `{"code":"`+req.Code+`"}`); status != http.StatusCreated {
		t.Fatalf("approve = %d %s", status, body)
	}
	if status, body := post(t, screen, e.srv.URL+"/api/display-sessions", `{"pollToken":"`+req.PollToken+`"}`); status != http.StatusCreated {
		t.Fatalf("claim = %d %s", status, body)
	}
	return screen
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func cspOf(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", url, resp.StatusCode)
	}
	return resp.Header.Get("Content-Security-Policy")
}

func frameSrc(csp string) string {
	for _, directive := range strings.Split(csp, ";") {
		if d := strings.TrimSpace(directive); strings.HasPrefix(d, "frame-src") {
			return d
		}
	}
	return ""
}

func TestWebPagesAreManagedByAnAdmin(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/web-pages", ""); status != http.StatusUnauthorized {
		t.Fatalf("list without login = %d, want 401", status)
	}
	env.setUp(t)
	page := env.addWebPage(t, grafanaPage)
	if page.ID == 0 || page.Zoom != 80 || !page.AllowScripts || page.RefreshMinutes != 15 {
		t.Fatalf("added page = %+v", page)
	}
	if env.settingsChanges.Load() != 1 {
		t.Errorf("settings changes = %d, want 1", env.settingsChanges.Load())
	}

	for _, body := range []string{
		`{"name":"Evil","url":"javascript:alert(1)"}`,
		`{"name":"Evil","url":"data:text/html,hi"}`,
		`{"name":"Evil","url":"file:///etc/passwd"}`,
		// GDash itself, on the origin this request came in on.
		`{"name":"Self","url":"` + env.srv.URL + `/d/keuken"}`,
	} {
		status, resp := env.do(t, "POST", "/api/web-pages", body)
		if status != http.StatusUnprocessableEntity || !strings.Contains(resp, `"field":"url"`) {
			t.Errorf("POST %s = %d %s, want 422 on url", body, status, resp)
		}
	}

	status, resp := env.do(t, "PUT", "/api/web-pages/"+itoa(page.ID), `{"name":"Grafana TV","url":"https://grafana.example.com/d/x","allowScripts":false}`)
	if status != http.StatusOK || !strings.Contains(resp, `"allowScripts":false`) || !strings.Contains(resp, `"zoom":100`) {
		t.Fatalf("update = %d %s", status, resp)
	}
	_, list := env.do(t, "GET", "/api/web-pages", "")
	if !strings.Contains(list, `"name":"Grafana TV"`) {
		t.Fatalf("list = %s", list)
	}
	if status, _ := env.do(t, "PUT", "/api/web-pages/999", grafanaPage); status != http.StatusNotFound {
		t.Errorf("update of an unknown page = %d, want 404", status)
	}
	if status, _ := env.do(t, "DELETE", "/api/web-pages/"+itoa(page.ID), ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := env.do(t, "DELETE", "/api/web-pages/"+itoa(page.ID), ""); status != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", status)
	}
	// The audit log names the host, never the path or query.
	want := []string{"web_page_removed:id 1", "web_page_changed:Grafana TV (grafana.example.com)", "web_page_added:Grafana (192.168.1.10:3000)"}
	if got := env.auditActions(t); !slices.Equal(got[:3], want) {
		t.Errorf("audit = %v, want %v first", got, want)
	}
}

func TestDisplayPageFramesOnlyTheOriginsOfItsOwnWebPages(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	grafana := env.addWebPage(t, grafanaPage)
	radar := env.addWebPage(t, `{"name":"Radar","url":"https://gadgets.buienradar.nl/gadget/zoommap"}`)
	dashboards := map[string]string{
		"kitchen": `{"id":"kitchen","name":"Kitchen","widgets":[{"id":"web","type":"webpage","x":0,"y":0,"w":8,"h":6,"settings":{"page":` + itoa(grafana.ID) + `}}]}`,
		"night":   `{"id":"night","name":"Night","widgets":[{"id":"web","type":"webpage","x":0,"y":0,"w":8,"h":6,"settings":{"page":` + itoa(radar.ID) + `}}]}`,
		"hall":    `{"id":"hall","name":"Hall","widgets":[{"id":"clock","type":"clock","x":0,"y":0,"w":8,"h":6}]}`,
	}
	for id, body := range dashboards {
		if status, resp := env.do(t, "POST", "/api/dashboards", body); status != http.StatusCreated {
			t.Fatalf("create dashboard %s = %d %s", id, status, resp)
		}
	}
	for _, body := range []string{`{"id":"keuken","name":"Keuken","dashboardId":"kitchen"}`, `{"id":"hal","name":"Hal","dashboardId":"hall"}`} {
		if status, resp := env.do(t, "POST", "/api/displays", body); status != http.StatusCreated {
			t.Fatalf("create display = %d %s", status, resp)
		}
	}
	screen := env.pairScreen(t, "keuken")

	if got := frameSrc(cspOf(t, screen, env.srv.URL+"/d/keuken")); got != "frame-src http://192.168.1.10:3000" {
		t.Errorf("paired screen: %q, want only Grafana's origin", got)
	}
	// Another dashboard in the display's rotation adds its pages.
	if status, resp := env.do(t, "PATCH", "/api/displays/keuken", `{"rotation":["night"],"rotateSeconds":60}`); status != http.StatusOK {
		t.Fatalf("rotation = %d %s", status, resp)
	}
	if got := frameSrc(cspOf(t, screen, env.srv.URL+"/d/keuken")); got != "frame-src http://192.168.1.10:3000 https://gadgets.buienradar.nl" {
		t.Errorf("with rotation: %q", got)
	}
	// A device that may not show the display learns nothing about its pages.
	if got := frameSrc(cspOf(t, tv(), env.srv.URL+"/d/keuken")); got != "frame-src 'none'" {
		t.Errorf("unpaired device: %q", got)
	}
	if got := frameSrc(cspOf(t, screen, env.srv.URL+"/d/hal")); got != "frame-src 'none'" {
		t.Errorf("screen of another display: %q", got)
	}
	// A display without a Web page widget frames nothing, even for an admin.
	if got := frameSrc(cspOf(t, env.client, env.srv.URL+"/d/hal")); got != "frame-src 'none'" {
		t.Errorf("display without web pages: %q", got)
	}
	// The web interface keeps its own policy.
	if csp := cspOf(t, env.client, env.srv.URL+"/"); frameSrc(csp) != "" {
		t.Errorf("web interface CSP = %q, want it unchanged", csp)
	}
}

func TestPreviewFramesEveryWebPageForAnAdminOnly(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.addWebPage(t, grafanaPage)
	env.addWebPage(t, `{"name":"Radar","url":"https://gadgets.buienradar.nl/gadget/zoommap"}`)
	csp := cspOf(t, env.client, env.srv.URL+"/preview/hall")
	if got := frameSrc(csp); got != "frame-src http://192.168.1.10:3000 https://gadgets.buienradar.nl" {
		t.Errorf("admin preview: %q", got)
	}
	if !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Errorf("preview CSP = %q", csp)
	}
	if got := frameSrc(cspOf(t, tv(), env.srv.URL+"/preview/hall")); got != "frame-src 'none'" {
		t.Errorf("anonymous preview: %q", got)
	}
}

func TestWebPageCheckReportsHeadersThatForbidFraming(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deny":
			w.Header().Set("X-Frame-Options", "DENY")
		case "/ancestors":
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'self' https://other.example")
		}
	}))
	defer page.Close()
	env := newTestEnv(t, nil)
	env.setUp(t)
	for path, want := range map[string]string{
		"/":          `{"status":200}`,
		"/deny":      `{"status":200,"blocked":"x-frame-options","header":"DENY"}`,
		"/ancestors": `{"status":200,"blocked":"frame-ancestors","header":"frame-ancestors 'self' https://other.example"}`,
	} {
		p := env.addWebPage(t, `{"name":"Page","url":"`+page.URL+path+`?token=abc"}`)
		status, body := env.do(t, "POST", "/api/web-pages/"+itoa(p.ID)+"/checks", "")
		if status != http.StatusOK || strings.TrimSpace(body) != want {
			t.Errorf("check %s = %d %s, want %s", path, status, body, want)
		}
	}
	if status, _ := env.do(t, "POST", "/api/web-pages/999/checks", ""); status != http.StatusNotFound {
		t.Errorf("check of an unknown page = %d, want 404", status)
	}

	gone := httptest.NewServer(http.NotFoundHandler())
	address := gone.URL + "/?token=secret"
	gone.Close()
	p := env.addWebPage(t, `{"name":"Gone","url":"`+address+`"}`)
	status, body := env.do(t, "POST", "/api/web-pages/"+itoa(p.ID)+"/checks", "")
	if status != http.StatusBadGateway || !strings.Contains(body, "could not be reached") || strings.Contains(body, "secret") {
		t.Errorf("check of an unreachable page = %d %s", status, body)
	}
}
