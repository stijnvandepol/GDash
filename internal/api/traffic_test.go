package api

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stijnvandepol/gdash/internal/knmi"
	"github.com/stijnvandepol/gdash/internal/traffic"
)

// fakeTrafficGuard reports a stop until it is reset, and counts resets.
type fakeTrafficGuard struct {
	resets atomic.Int32
}

func (g *fakeTrafficGuard) Status() traffic.Status {
	if g.resets.Load() == 0 {
		return traffic.Status{State: traffic.StateStopped, HTTPStatus: http.StatusForbidden}
	}
	return traffic.Status{State: traffic.StateIdle}
}

func (g *fakeTrafficGuard) Reset() { g.resets.Add(1) }

// knmiTestKey is a made-up key that fakeKNMI accepts.
const knmiTestKey = "made-up-knmi-api-test-key"

type fakeKNMI struct{}

func (fakeKNMI) Check(_ context.Context, key string) error {
	if key != knmiTestKey {
		return knmi.ErrUnauthorized
	}
	return nil
}

func TestTrafficInfoNeedsTheAcknowledgement(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "GET", "/api/traffic", "")
	if status != http.StatusOK || !strings.Contains(body, `"enabled":false`) || !strings.Contains(body, `"state":"stopped"`) {
		t.Fatalf("traffic before = %d %s", status, body)
	}
	status, body = env.do(t, "PUT", "/api/traffic", `{"enabled":true}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"acknowledged"`) {
		t.Fatalf("on without acknowledging = %d %s, want 422", status, body)
	}
	if env.settingsChanges.Load() != 0 || env.traffic.resets.Load() != 0 {
		t.Fatal("a refused switch changed something")
	}

	status, body = env.do(t, "PUT", "/api/traffic", `{"enabled":true,"acknowledged":true}`)
	if status != http.StatusOK || !strings.Contains(body, `"enabled":true`) || !strings.Contains(body, `"acknowledgedAt"`) {
		t.Fatalf("on = %d %s", status, body)
	}
	// Turning it on starts the requests afresh after a stop.
	if env.traffic.resets.Load() != 1 || env.settingsChanges.Load() != 1 {
		t.Fatalf("resets %d, settings changes %d", env.traffic.resets.Load(), env.settingsChanges.Load())
	}
	if _, body := env.do(t, "GET", "/api/traffic", ""); !strings.Contains(body, `"state":"idle"`) {
		t.Fatalf("traffic after = %s", body)
	}
	if _, audit := env.do(t, "GET", "/api/audit-events", ""); !strings.Contains(audit, "traffic_info_enabled") {
		t.Fatalf("audit = %s", audit)
	}
	if status, body := env.do(t, "PUT", "/api/traffic", `{"enabled":false}`); status != http.StatusOK || !strings.Contains(body, `"enabled":false`) {
		t.Fatalf("off = %d %s", status, body)
	}
}

func TestKNMIKeyIsTriedAndNeverShown(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "PUT", "/api/knmi", `{"key":"wrong-made-up-key"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"key"`) || !strings.Contains(body, "did not accept") {
		t.Fatalf("wrong key = %d %s, want 422 on key", status, body)
	}
	if env.settingsChanges.Load() != 0 {
		t.Fatal("a refused key changed the settings")
	}
	status, body = env.do(t, "PUT", "/api/knmi", `{"key":" `+knmiTestKey+` "}`)
	if status != http.StatusOK || !strings.Contains(body, `"connected":true`) {
		t.Fatalf("connect = %d %s", status, body)
	}
	_, view := env.do(t, "GET", "/api/knmi", "")
	_, settingsBody := env.do(t, "GET", "/api/settings", "")
	_, audit := env.do(t, "GET", "/api/audit-events", "")
	for name, text := range map[string]string{"answer": body, "knmi": view, "settings": settingsBody, "audit log": audit, "log": env.logs.String()} {
		if strings.Contains(text, knmiTestKey) || strings.Contains(text, "wrong-made-up-key") {
			t.Fatalf("the %s contains a key: %s", name, text)
		}
	}
	if !strings.Contains(view, `"connected":true`) {
		t.Fatalf("knmi = %s", view)
	}
	if status, _ := env.do(t, "DELETE", "/api/knmi", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if _, view := env.do(t, "GET", "/api/knmi", ""); !strings.Contains(view, `"connected":false`) {
		t.Fatalf("knmi after delete = %s", view)
	}
}
