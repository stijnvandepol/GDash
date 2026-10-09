package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAuditLogRecordsSecurityEvents(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/audit-events", ""); status != http.StatusUnauthorized {
		t.Fatalf("audit log without login = %d, want 401", status)
	}
	env.do(t, "POST", "/api/setup", `{"setupToken":"a wrong guess","username":"stijn","password":"correct horse battery"}`)
	env.setUp(t)
	env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"my secret guess"}`)
	env.bearer(t, "POST", "/api/announcements", `{"message":"x"}`, "gdk_not-a-real-token", nil)
	env.createKitchen(t)
	env.do(t, "POST", "/api/calendars", `{"name":"Family","url":"https://calendar.example.org/private-0123456789/basic.ics"}`)

	status, body := env.do(t, "GET", "/api/audit-events", "")
	if status != http.StatusOK {
		t.Fatalf("audit log = %d %s", status, body)
	}
	for _, want := range []string{
		`"actor":"stijn","action":"account_created"`,
		`"actor":"","action":"login_failed","detail":"wrong username or password"`,
		`"actor":"stijn","action":"display_created","detail":"keuken"`,
		`"actor":"stijn","action":"calendar_added","detail":"Family"`,
		`"actor":"","action":"setup_failed","detail":"wrong setup token"`,
		`"actor":"","action":"api_token_rejected"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("audit log lacks %s:\n%s", want, body)
		}
	}
	for _, secret := range []string{"my secret guess", "private-0123456789", "a wrong guess", "gdk_not-a-real-token"} {
		if strings.Contains(body, secret) {
			t.Errorf("audit log contains %q", secret)
		}
	}
	// Newest first.
	if strings.Index(body, "calendar_added") > strings.Index(body, "account_created") {
		t.Fatalf("events are not newest first:\n%s", body)
	}

	if status, _ = env.do(t, "GET", "/api/audit-events?limit=abc", ""); status != http.StatusBadRequest {
		t.Fatalf("bad limit = %d, want 400", status)
	}
	if _, body = env.do(t, "GET", "/api/audit-events?limit=1", ""); strings.Count(body, `"action"`) != 1 {
		t.Fatalf("limit=1 = %s", body)
	}
}
