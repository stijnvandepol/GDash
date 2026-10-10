package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestBackupNeedsThePasswordAgain(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "POST", "/api/backup", `{"password":"not my password"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"password"`) {
		t.Fatalf("wrong password = %d %s, want 422 on password", status, body)
	}

	req, _ := http.NewRequest(http.MethodPost, env.srv.URL+"/api/backup", strings.NewReader(`{"password":"correct horse battery"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("backup = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="gdash-backup-2026-10-08.zip"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	_, audit := env.do(t, "GET", "/api/audit-events", "")
	for _, want := range []string{`"action":"backup_failed","detail":"wrong password"`, `"action":"backup_downloaded"`} {
		if !strings.Contains(audit, want) {
			t.Errorf("activity log lacks %s", want)
		}
	}
	if strings.Contains(audit, "not my password") || strings.Contains(audit, "correct horse") {
		t.Error("the activity log holds a password")
	}
}

func TestBackupNeedsLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.do(t, "DELETE", "/api/sessions/current", "")
	if status, _ := env.do(t, "POST", "/api/backup", `{"password":"correct horse battery"}`); status != http.StatusUnauthorized {
		t.Fatalf("backup without login = %d, want 401", status)
	}
}
