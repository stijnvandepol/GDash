package api

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// screenReport is a report as the display page sends it.
const screenReport = `{
	"reason": "connected",
	"userAgent": "Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36 CrKey/1.56.500000 DeviceType/Chromecast",
	"viewport": {"width": 1280, "height": 720},
	"devicePixelRatio": 1.5,
	"stageScale": 0.6667,
	"features": {"containerUnits": false, "containerQueries": false, "where": true, "has": false, "colorMix": false,
		"aspectRatio": true, "flexGap": true, "inset": true, "lineClamp": true, "textWrap": false, "nesting": false, "cascadeLayers": true},
	"compat": true,
	"fontsLoaded": true,
	"firstRenderMs": 2400,
	"errors": [{"message": "TypeError: x is not a function", "source": "/assets/App-abc.js:1:200"}],
	"visibility": [{"state": "hidden", "at": "2026-10-08T16:59:00Z"}, {"state": "visible", "at": "2026-10-08T16:59:30Z"}]
}`

// hiddenReport is the report a page sends the moment it is hidden.
var hiddenReport = strings.Replace(screenReport, `"reason": "connected"`, `"reason": "hidden"`, 1)

// visibleReport is the report a page sends when it is on screen again.
var visibleReport = strings.Replace(screenReport, `"reason": "connected"`, `"reason": "visible"`, 1)

type listedReports struct {
	Displays []struct {
		ID            string `json:"id"`
		ScreenReports []struct {
			Screen     string `json:"screen"`
			UserAgent  string `json:"userAgent"`
			ReceivedAt string `json:"receivedAt"`
			Features   struct {
				ContainerUnits bool `json:"containerUnits"`
			} `json:"features"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"screenReports"`
	} `json:"displays"`
}

func reportsOf(t *testing.T, env *testEnv, displayID string) listedReports {
	t.Helper()
	status, body := env.do(t, "GET", "/api/displays", "")
	if status != http.StatusOK {
		t.Fatalf("list displays = %d %s", status, body)
	}
	var list listedReports
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}
	for _, d := range list.Displays {
		if d.ID == displayID && d.ScreenReports == nil {
			t.Fatalf("display %s lists screenReports as null, want a list", displayID)
		}
	}
	return list
}

func TestPairedScreenReportsItsBrowser(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	screen := pairedScreen(t, env)

	if status, body := post(t, screen, env.srv.URL+"/api/displays/keuken/screen-reports", screenReport); status != http.StatusNoContent {
		t.Fatalf("report = %d %s, want 204", status, body)
	}
	list := reportsOf(t, env, "keuken")
	reports := list.Displays[0].ScreenReports
	if len(reports) != 1 {
		t.Fatalf("screenReports = %+v, want one", reports)
	}
	got := reports[0]
	if got.Screen != "screen" || !strings.Contains(got.UserAgent, "CrKey/1.56") || got.Features.ContainerUnits || got.ReceivedAt != "2026-10-08T17:00:00Z" {
		t.Fatalf("report = %+v", got)
	}
	if len(got.Errors) != 1 || got.Errors[0].Message != "TypeError: x is not a function" {
		t.Fatalf("errors = %+v", got.Errors)
	}

	// Unpairing the screens forgets what they reported.
	if status, body := env.do(t, "DELETE", "/api/displays/keuken/pairings", ""); status != http.StatusOK {
		t.Fatalf("unpair = %d %s", status, body)
	}
	if reports := reportsOf(t, env, "keuken").Displays[0].ScreenReports; len(reports) != 0 {
		t.Fatalf("screenReports after unpairing = %+v, want none", reports)
	}
}

func TestScreenReportIsOnlyForTheScreenItself(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	if status, body := env.do(t, "POST", "/api/displays", `{"id":"kantoor","name":"Kantoor"}`); status != http.StatusCreated {
		t.Fatalf("create display = %d %s", status, body)
	}
	screen := pairedScreen(t, env)

	tests := []struct {
		name   string
		client *http.Client
		path   string
		want   int
	}{
		{"an unpaired screen", tv(), "/api/displays/keuken/screen-reports", http.StatusUnauthorized},
		{"an admin without a display cookie", env.client, "/api/displays/keuken/screen-reports", http.StatusUnauthorized},
		{"a screen of another display", screen, "/api/displays/kantoor/screen-reports", http.StatusForbidden},
		{"an invalid display id", screen, "/api/displays/Not_Valid/screen-reports", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status, body := post(t, tt.client, env.srv.URL+tt.path, screenReport); status != tt.want {
				t.Fatalf("report = %d %s, want %d", status, body, tt.want)
			}
		})
	}
	for _, d := range reportsOf(t, env, "keuken").Displays {
		if len(d.ScreenReports) != 0 {
			t.Fatalf("display %s has reports %+v after refused ones", d.ID, d.ScreenReports)
		}
	}
}

func TestScreenReportValidation(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	screen := pairedScreen(t, env)
	url := env.srv.URL + "/api/displays/keuken/screen-reports"

	tests := []struct {
		name string
		body string
		want int
		code string
	}{
		{"an unknown field", strings.Replace(screenReport, `"compat": true`, `"compat": true, "location": "kitchen"`, 1), http.StatusBadRequest, "invalid_json"},
		{"an unknown feature", strings.Replace(screenReport, `"nesting": false`, `"nesting": false, "camera": true`, 1), http.StatusBadRequest, "invalid_json"},
		{"a user agent with a line break", strings.Replace(screenReport, `Mozilla/5.0 (X11`, `Mozilla/5.0\n(X11`, 1), http.StatusUnprocessableEntity, "invalid_input"},
		{"a zero viewport", strings.Replace(screenReport, `"width": 1280`, `"width": 0`, 1), http.StatusUnprocessableEntity, "invalid_input"},
		{"more than 8 KB", strings.Replace(screenReport, `"TypeError`, `"`+strings.Repeat("x", 9000), 1), http.StatusRequestEntityTooLarge, "too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := post(t, screen, url, tt.body)
			if status != tt.want || !strings.Contains(body, `"code":"`+tt.code+`"`) {
				t.Fatalf("report = %d %s, want %d %s", status, body, tt.want, tt.code)
			}
		})
	}
}

func TestScreenReportsAreRateLimited(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	screen := pairedScreen(t, env)
	url := env.srv.URL + "/api/displays/keuken/screen-reports"
	for i := range 30 {
		if status, body := post(t, screen, url, screenReport); status != http.StatusNoContent {
			t.Fatalf("report %d = %d %s, want 204", i+1, status, body)
		}
	}
	if status, body := post(t, screen, url, screenReport); status != http.StatusTooManyRequests {
		t.Fatalf("report over the limit = %d %s, want 429", status, body)
	}
	// Another screen of the same display is not held back.
	if status, body := post(t, pairedScreen(t, env), url, screenReport); status != http.StatusNoContent {
		t.Fatalf("report of a second screen = %d %s, want 204", status, body)
	}
}

func TestDeletingADisplayForgetsItsReports(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	screen := pairedScreen(t, env)
	if status, body := post(t, screen, env.srv.URL+"/api/displays/keuken/screen-reports", screenReport); status != http.StatusNoContent {
		t.Fatalf("report = %d %s", status, body)
	}
	if status, body := env.do(t, "DELETE", "/api/displays/keuken", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d %s", status, body)
	}
	env.createKitchen(t)
	if reports := reportsOf(t, env, "keuken").Displays[0].ScreenReports; len(reports) != 0 {
		t.Fatalf("a new display with the same id has reports %+v", reports)
	}
}

func TestVisibilityReportsOfTheCastTVReachItsKeeper(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	launch, err := env.pairing.NewLaunchToken("keuken", netip.MustParseAddr("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	castTV := tv()
	if status, body := post(t, castTV, env.srv.URL+"/api/display-sessions", `{"launchToken":"`+launch+`"}`); status != http.StatusCreated {
		t.Fatalf("launch exchange = %d %s", status, body)
	}
	url := env.srv.URL + "/api/displays/keuken/screen-reports"

	if status, body := post(t, castTV, url, screenReport); status != http.StatusNoContent {
		t.Fatalf("report = %d %s", status, body)
	}
	if got := env.sources.hiddenDisplays(); len(got) != 0 {
		t.Fatalf("hidden after a connected report = %v, want none", got)
	}
	if status, body := post(t, castTV, url, hiddenReport); status != http.StatusNoContent {
		t.Fatalf("hidden report = %d %s", status, body)
	}
	if got := env.sources.hiddenDisplays(); !slices.Equal(got, []string{"keuken"}) {
		t.Fatalf("hidden = %v, want the kitchen's keeper told", got)
	}
	if status, body := post(t, castTV, url, visibleReport); status != http.StatusNoContent {
		t.Fatalf("visible report = %d %s", status, body)
	}
	if got := env.sources.shownDisplays(); !slices.Equal(got, []string{"keuken"}) {
		t.Fatalf("shown = %v, want the kitchen's keeper told", got)
	}
	if got := env.sources.hiddenDisplays(); len(got) != 1 {
		t.Fatalf("hidden after a visible report = %v, want only the first", got)
	}

	// A tablet paired with a code is hidden when someone puts it away:
	// nothing for the keeper.
	tablet := pairedScreen(t, env)
	if status, body := post(t, tablet, url, hiddenReport); status != http.StatusNoContent {
		t.Fatalf("hidden report of a tablet = %d %s", status, body)
	}
	if status, body := post(t, tablet, url, visibleReport); status != http.StatusNoContent {
		t.Fatalf("visible report of a tablet = %d %s", status, body)
	}
	if got, shown := env.sources.hiddenDisplays(), env.sources.shownDisplays(); len(got) != 1 || len(shown) != 1 {
		t.Fatalf("hidden = %v, shown = %v, want only the TV's reports passed on", got, shown)
	}

	reports := reportsOf(t, env, "keuken").Displays[0].ScreenReports
	if len(reports) != 2 {
		t.Fatalf("reports = %+v, want the TV and the tablet", reports)
	}
}
