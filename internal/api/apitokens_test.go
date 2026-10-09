package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// bearer sends a request the way an automation does: no cookies, a bearer
// token and any extra headers.
func (e *testEnv) bearer(t *testing.T, method, path, body, secret string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// makeToken creates an API token as the logged-in admin and returns its id
// and secret.
func (e *testEnv) makeToken(t *testing.T, name string) (int64, string) {
	t.Helper()
	status, body := e.do(t, "POST", "/api/api-tokens", `{"name":"`+name+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("create token = %d %s", status, body)
	}
	var made struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &made); err != nil || made.Token == "" {
		t.Fatalf("create token body = %s, %v", body, err)
	}
	return made.ID, made.Token
}

func TestAPITokensNeedLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, req := range []struct{ method, path, body string }{
		{"GET", "/api/api-tokens", ""},
		{"POST", "/api/api-tokens", `{"name":"x"}`},
		{"DELETE", "/api/api-tokens/1", ""},
	} {
		if status, _ := env.do(t, req.method, req.path, req.body); status != http.StatusUnauthorized {
			t.Errorf("%s %s without login = %d, want 401", req.method, req.path, status)
		}
	}
}

func TestAPITokenLifecycleAndAudit(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	id, secret := env.makeToken(t, "Home Assistant")

	// The list never shows the secret.
	status, body := env.do(t, "GET", "/api/api-tokens", "")
	if status != http.StatusOK || !strings.Contains(body, `"name":"Home Assistant"`) || !strings.Contains(body, `"lastUsedAt":null`) {
		t.Fatalf("list = %d %s", status, body)
	}
	if strings.Contains(body, secret) || strings.Contains(body, `"token"`) {
		t.Fatalf("the list shows a token: %s", body)
	}

	if status, body = env.do(t, "DELETE", "/api/api-tokens/999", ""); status != http.StatusNotFound {
		t.Fatalf("revoke unknown = %d %s, want 404", status, body)
	}
	if status, _ = env.do(t, "DELETE", "/api/api-tokens/abc", ""); status != http.StatusNotFound {
		t.Fatalf("revoke with a bad id = %d, want 404", status)
	}
	if status, body = env.do(t, "DELETE", "/api/api-tokens/"+strconv.FormatInt(id, 10), ""); status != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", status, body)
	}
	if _, body = env.do(t, "GET", "/api/api-tokens", ""); strings.Contains(body, "Home Assistant") {
		t.Fatalf("revoked token still listed: %s", body)
	}

	_, log := env.do(t, "GET", "/api/audit-events", "")
	for _, want := range []string{
		`"actor":"stijn","action":"api_token_created","detail":"Home Assistant"`,
		`"actor":"stijn","action":"api_token_revoked","detail":"Home Assistant"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("audit log lacks %s:\n%s", want, log)
		}
	}
	if strings.Contains(log, secret) {
		t.Errorf("audit log contains the token")
	}
}

func TestAPITokenValidationAndLimit(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/api-tokens", `{"name":""}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"name"`) {
		t.Fatalf("empty name = %d %s", status, body)
	}
	if status, _ = env.do(t, "POST", "/api/api-tokens", `{"name":"x","extra":1}`); status != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", status)
	}
	for i := range 10 {
		env.makeToken(t, "token "+strconv.Itoa(i))
	}
	if status, body = env.do(t, "POST", "/api/api-tokens", `{"name":"eleventh"}`); status != http.StatusConflict {
		t.Fatalf("eleventh token = %d %s, want 409", status, body)
	}
}
