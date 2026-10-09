package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestResponsesCarrySecurityHeaders(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	resp, err := env.client.Get(env.srv.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for header, want := range map[string]string{
		"Cache-Control":                "no-store",
		"X-Content-Type-Options":       "nosniff",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if !strings.Contains(resp.Header.Get("Permissions-Policy"), "camera=()") {
		t.Errorf("Permissions-Policy = %q", resp.Header.Get("Permissions-Policy"))
	}
	if resp.Header.Get("Strict-Transport-Security") != "" {
		t.Error("HSTS was sent over plain HTTP")
	}
}

func TestReceiverPageCannotBeFramed(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Get(env.srv.URL + "/receiver/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("receiver CSP = %q", csp)
	}
}

func TestAssetsAreNotListed(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/assets/", ""); status != http.StatusNotFound {
		t.Fatalf("GET /assets/ = %d, want 404", status)
	}
	if status, _ := env.do(t, "GET", "/assets/app.js", ""); status != http.StatusOK {
		t.Fatalf("GET /assets/app.js = %d, want 200", status)
	}
}

func TestJSONBodiesNeedTheJSONType(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	req, _ := http.NewRequest(http.MethodPut, env.srv.URL+"/api/settings/language", strings.NewReader(`{"language":"nl"}`))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain body = %d, want 415", resp.StatusCode)
	}
	if status, _ := env.do(t, "PUT", "/api/settings/language", `{"language":"nl"}`); status != http.StatusOK {
		t.Fatalf("application/json body = %d, want 200", status)
	}
}
