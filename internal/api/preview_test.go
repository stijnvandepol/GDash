package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
)

const previewLayout = `{"id":"hall","name":"Hall","widgets":[` +
	`{"id":"clock","type":"clock","x":0,"y":0,"w":8,"h":4},` +
	`{"id":"news","type":"news","x":8,"y":0,"w":10,"h":6}]}`

// previewEnv has a stored dashboard "hall" with a clock and news, and data
// for three sources, of which the dashboard uses one.
func previewEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "POST", "/api/dashboards", previewLayout); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	env.sources.set(nil, nil, map[string]datahub.Entry{
		"news.default":     {Value: json.RawMessage(`[{"title":"Headline"}]`)},
		"weather.current":  {Value: json.RawMessage(`{"temperature":12}`)},
		"calendar.default": {Value: json.RawMessage(`[]`)},
	})
	return env
}

func decodeSnapshot(t *testing.T, body string) display.Snapshot {
	t.Helper()
	var s display.Snapshot
	if err := json.Unmarshal([]byte(body), &s); err != nil {
		t.Fatalf("snapshot = %s: %v", body, err)
	}
	return s
}

func TestPreviewPageMayBeFramedByTheWebInterfaceOnly(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Get(env.srv.URL + "/preview/hall")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "<html>preview</html>" {
		t.Fatalf("GET /preview/hall = %d %q", resp.StatusCode, body)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
	// Display pages and the web interface are never framed.
	displayResp, err := http.Get(env.srv.URL + "/d/keuken")
	if err != nil {
		t.Fatal(err)
	}
	displayResp.Body.Close()
	if csp := displayResp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("display CSP = %q, want frame-ancestors 'none'", csp)
	}
	for _, path := range []string{"/preview/Hall", "/preview/..%2fetc", "/preview/" + strings.Repeat("a", 40)} {
		if status, _ := env.do(t, "GET", path, ""); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

func TestPreviewNeedsAnAdmin(t *testing.T) {
	env := previewEnv(t)
	screen := tv()
	resp, err := screen.Get(env.srv.URL + "/api/dashboards/hall/preview")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET without a session = %d, want 401", resp.StatusCode)
	}
	if status, _ := post(t, screen, env.srv.URL+"/api/dashboards/hall/preview", `{"name":"Hall","widgets":[]}`); status != http.StatusUnauthorized {
		t.Errorf("POST without a session = %d, want 401", status)
	}
}

func TestPreviewOfAnUnknownDashboard(t *testing.T) {
	env := previewEnv(t)
	if status, _ := env.do(t, "GET", "/api/dashboards/nope/preview", ""); status != http.StatusNotFound {
		t.Errorf("GET = %d, want 404", status)
	}
	if status, _ := env.do(t, "POST", "/api/dashboards/nope/preview", `{"name":"Nope","widgets":[]}`); status != http.StatusNotFound {
		t.Errorf("POST = %d, want 404", status)
	}
}

func TestPreviewHoldsOnlyTheDataTheDashboardUses(t *testing.T) {
	env := previewEnv(t)
	status, body := env.do(t, "GET", "/api/dashboards/hall/preview", "")
	if status != http.StatusOK {
		t.Fatalf("GET = %d %s", status, body)
	}
	s := decodeSnapshot(t, body)
	if _, ok := s.Data["news.default"]; !ok || len(s.Data) != 1 {
		t.Fatalf("data = %v, want only news.default", s.Data)
	}
	if s.Type != "snapshot" || !s.Active || s.Dashboard.ID != "hall" || len(s.Dashboard.Widgets) != 2 || s.TextScale != 100 || s.Language != "en" {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestPreviewFollowsLanguageAndTextSize(t *testing.T) {
	env := previewEnv(t)
	if status, body := env.do(t, "PUT", "/api/settings/language", `{"language":"nl"}`); status != http.StatusOK {
		t.Fatalf("set language = %d %s", status, body)
	}
	_, body := env.do(t, "GET", "/api/dashboards/hall/preview?textScale=150", "")
	if s := decodeSnapshot(t, body); s.Language != "nl" || s.TextScale != 150 {
		t.Fatalf("language %q, text size %d; want nl and 150", s.Language, s.TextScale)
	}
	for _, size := range []string{"130", "0", "big", "-100"} {
		status, body := env.do(t, "GET", "/api/dashboards/hall/preview?textScale="+size, "")
		if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"textScale"`) {
			t.Errorf("textScale=%s = %d %s, want 422 on textScale", size, status, body)
		}
	}
}

func TestPreviewOfAnUnsavedLayout(t *testing.T) {
	env := previewEnv(t)
	draft := `{"name":"Hall draft","theme":"night","widgets":[{"id":"weather","type":"weather","x":0,"y":0,"w":10,"h":14}]}`
	status, body := env.do(t, "POST", "/api/dashboards/hall/preview?textScale=125", draft)
	if status != http.StatusOK {
		t.Fatalf("POST = %d %s", status, body)
	}
	s := decodeSnapshot(t, body)
	if s.Dashboard.Name != "Hall draft" || s.Dashboard.Theme != "night" || len(s.Dashboard.Widgets) != 1 || s.TextScale != 125 {
		t.Fatalf("dashboard = %+v, text size %d; want the draft", s.Dashboard, s.TextScale)
	}
	if _, ok := s.Data["weather.current"]; !ok || len(s.Data) != 1 {
		t.Fatalf("data = %v, want the weather of the draft and nothing of the stored layout", s.Data)
	}

	// Previewing never saves, and tells no screen.
	changes := env.changes.Load()
	_, stored := env.do(t, "GET", "/api/dashboards/hall", "")
	if strings.Contains(stored, "Hall draft") || !strings.Contains(stored, `"name":"Hall"`) {
		t.Fatalf("stored dashboard = %s, want it unchanged", stored)
	}
	if env.changes.Load() != changes {
		t.Fatal("a preview told the displays about a change")
	}
}

func TestPreviewRejectsAnInvalidLayout(t *testing.T) {
	env := previewEnv(t)
	overlap := `{"name":"Hall","widgets":[{"id":"a","type":"clock","x":0,"y":0,"w":8,"h":4},{"id":"b","type":"clock","x":4,"y":2,"w":8,"h":4}]}`
	status, body := env.do(t, "POST", "/api/dashboards/hall/preview", overlap)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"widgets[1]"`) {
		t.Errorf("overlapping layout = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/dashboards/hall/preview", `{"name":"Hall","colour":"red"}`); status != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", status)
	}
}

func TestPreviewShowsTheNightThemeAfterSunset(t *testing.T) {
	env := previewEnv(t)
	// The test clock says 17:00 UTC on 8 October, 19:00 where the weather is.
	sun := func(sunset string) map[string]datahub.Entry {
		report := `{"isDay":true,"utcOffsetSeconds":7200,"sun":[{"date":"2026-10-08","sunrise":"2026-10-08T07:48","sunset":"2026-10-08T` + sunset + `"}]}`
		return map[string]datahub.Entry{"weather.current": {Value: json.RawMessage(report)}}
	}
	draft := `{"name":"Hall","theme":"frost","font":"lora","nightTheme":"frost-night","widgets":[{"id":"clock","type":"clock","x":0,"y":0,"w":8,"h":4}]}`
	preview := func(method, body string) display.Dashboard {
		status, got := env.do(t, method, "/api/dashboards/hall/preview", body)
		if status != http.StatusOK {
			t.Fatalf("%s preview = %d %s", method, status, got)
		}
		return decodeSnapshot(t, got).Dashboard
	}
	if status, body := env.do(t, "PUT", "/api/dashboards/hall", draft); status != http.StatusOK {
		t.Fatalf("save = %d %s", status, body)
	}

	env.sources.set(nil, nil, sun("18:30"))
	for method, body := range map[string]string{"GET": "", "POST": draft} {
		if d := preview(method, body); d.Theme != "frost-night" || d.Font != "lora" {
			t.Errorf("%s after sunset: theme %q font %q, want frost-night and lora", method, d.Theme, d.Font)
		}
	}
	env.sources.set(nil, nil, sun("20:00"))
	if d := preview("GET", ""); d.Theme != "frost" {
		t.Errorf("before sunset the preview shows %q, want frost", d.Theme)
	}
	// Without a weather location it never switches.
	env.sources.set(nil, nil, nil)
	if d := preview("GET", ""); d.Theme != "frost" {
		t.Errorf("without a location the preview shows %q, want frost", d.Theme)
	}

	bad := `{"name":"Hall","nightTheme":"sunset","widgets":[]}`
	if status, body := env.do(t, "POST", "/api/dashboards/hall/preview", bad); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"nightTheme"`) {
		t.Errorf("unknown night theme = %d %s, want 422 on nightTheme", status, body)
	}
}
