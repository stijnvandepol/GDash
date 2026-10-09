package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/nookboard/internal/pairing"
)

func TestEventStreamRequiresPairing(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	if status, _ := readFirstEvent(t, tv(), env.srv.URL+"/api/displays/keuken/events"); status != http.StatusUnauthorized {
		t.Fatalf("unpaired stream = %d, want 401", status)
	}
	// A logged-in admin may preview.
	if status, got := readFirstEvent(t, env.client, env.srv.URL+"/api/displays/keuken/events"); status != http.StatusOK || got != wantFirstEvent {
		t.Fatalf("admin preview = %d %q", status, got)
	}
}

func TestCodePairingGivesScreenItsDisplay(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	screen := tv()

	status, body := post(t, screen, env.srv.URL+"/api/pairing-requests", `{"displayId":"keuken"}`)
	if status != http.StatusCreated {
		t.Fatalf("pairing request = %d %s", status, body)
	}
	var req pairing.Request
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	claim := `{"pollToken":"` + req.PollToken + `"}`
	if status, _ := post(t, screen, env.srv.URL+"/api/display-sessions", claim); status != http.StatusAccepted {
		t.Fatalf("claim before approval = %d, want 202", status)
	}
	if status, _ := env.do(t, "POST", "/api/displays/keuken/pairings", `{"code":"WRONG-CODE"}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("approve with wrong code = %d, want 422", status)
	}
	if status, body := env.do(t, "POST", "/api/displays/keuken/pairings", `{"code":"`+req.Code+`"}`); status != http.StatusCreated || !strings.Contains(body, `"displayId":"keuken"`) {
		t.Fatalf("approve = %d %s", status, body)
	}
	if status, body := post(t, screen, env.srv.URL+"/api/display-sessions", claim); status != http.StatusCreated || !strings.Contains(body, `"displayId":"keuken"`) {
		t.Fatalf("claim = %d %s", status, body)
	}

	if status, got := readFirstEvent(t, screen, env.srv.URL+"/api/displays/keuken/events"); status != http.StatusOK || got != wantFirstEvent {
		t.Fatalf("paired stream = %d %q", status, got)
	}
	_, list := env.do(t, "GET", "/api/displays", "")
	if !strings.Contains(list, `"pairedScreens":1`) {
		t.Fatalf("display list = %s, want one paired screen", list)
	}

	if status, body := env.do(t, "DELETE", "/api/displays/keuken/pairings", ""); status != http.StatusOK || !strings.Contains(body, `"removed":1`) {
		t.Fatalf("unpair = %d %s", status, body)
	}
	if status, _ := readFirstEvent(t, screen, env.srv.URL+"/api/displays/keuken/events"); status != http.StatusUnauthorized {
		t.Fatalf("stream after unpairing = %d, want 401", status)
	}
}

func TestOpenStreamEndsWhenScreenIsUnpaired(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	launch, err := env.pairing.NewLaunchToken("keuken")
	if err != nil {
		t.Fatal(err)
	}
	screen := tv()
	if status, _ := post(t, screen, env.srv.URL+"/api/display-sessions", `{"launchToken":"`+launch+`"}`); status != http.StatusCreated {
		t.Fatal("launch exchange failed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, env.srv.URL+"/api/displays/keuken/events", nil)
	resp, err := screen.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("open stream: %v %v", resp.StatusCode, err)
	}
	defer resp.Body.Close()

	if status, _ := env.do(t, "DELETE", "/api/displays/keuken/pairings", ""); status != http.StatusOK {
		t.Fatal("unpair failed")
	}
	// The stream must end by itself, well before the test deadline.
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("stream did not end after unpairing: %v", err)
	}
}

func TestPairedScreenCannotShowAnotherDisplay(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	if status, _ := env.do(t, "POST", "/api/displays", `{"id":"kantoor","name":"Kantoor"}`); status != http.StatusCreated {
		t.Fatal("create second display failed")
	}
	launch, err := env.pairing.NewLaunchToken("keuken")
	if err != nil {
		t.Fatal(err)
	}
	screen := tv()
	if status, body := post(t, screen, env.srv.URL+"/api/display-sessions", `{"launchToken":"`+launch+`"}`); status != http.StatusCreated {
		t.Fatalf("launch exchange = %d %s", status, body)
	}
	if status, _ := readFirstEvent(t, screen, env.srv.URL+"/api/displays/kantoor/events"); status != http.StatusUnauthorized {
		t.Fatalf("stream of another display = %d, want 401", status)
	}
	if status, _ := post(t, screen, env.srv.URL+"/api/display-sessions", `{"launchToken":"`+launch+`"}`); status != http.StatusUnauthorized {
		t.Fatalf("reused launch token = %d, want 401", status)
	}
}

func TestDisplaySessionRequestShape(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, body := range []string{`{}`, `{"pollToken":"a","launchToken":"b"}`} {
		if status, _ := post(t, tv(), env.srv.URL+"/api/display-sessions", body); status != http.StatusBadRequest {
			t.Errorf("POST /api/display-sessions %s = %d, want 400", body, status)
		}
	}
	if status, _ := env.do(t, "GET", "/api/display-sessions/current", ""); status != http.StatusUnauthorized {
		t.Fatalf("current session without cookie = %d, want 401", status)
	}
}
