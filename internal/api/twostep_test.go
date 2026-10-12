package api

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/totp"
)

const testPassword = "correct horse battery"

// enableTwoStep turns two-step login on for the logged-in admin through the
// API, and returns the secret, the time step its first code used, and the
// recovery codes.
func (e *testEnv) enableTwoStep(t *testing.T) ([]byte, int64, []string) {
	t.Helper()
	status, body := e.do(t, "POST", "/api/me/two-step/enrolment", `{"password":"`+testPassword+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("enrolment = %d %s", status, body)
	}
	var enrolment struct{ Secret, URI, QR string }
	if err := json.Unmarshal([]byte(body), &enrolment); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enrolment.URI, "otpauth://totp/GDash:stijn?") || !strings.HasPrefix(enrolment.QR, "<svg") {
		t.Fatalf("enrolment = %s", body)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrolment.Secret)
	if err != nil {
		t.Fatal(err)
	}
	step := totp.Step(time.Now())
	status, body = e.do(t, "PUT", "/api/me/two-step", `{"code":"`+totp.Code(secret, step)+`"}`)
	if status != http.StatusOK {
		t.Fatalf("turn on = %d %s", status, body)
	}
	var on struct{ RecoveryCodes []string }
	if err := json.Unmarshal([]byte(body), &on); err != nil || len(on.RecoveryCodes) != 10 {
		t.Fatalf("recovery codes in %s: %v", body, err)
	}
	if strings.Contains(e.logs.String(), enrolment.Secret) {
		t.Fatal("the two-step secret reached the log")
	}
	return secret, step, on.RecoveryCodes
}

// passwordStep logs out and in with the password, and returns the pending
// login.
func (e *testEnv) passwordStep(t *testing.T) string {
	t.Helper()
	e.do(t, "DELETE", "/api/sessions/current", "")
	status, body := e.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"`+testPassword+`"}`)
	if status != http.StatusAccepted {
		t.Fatalf("password step = %d %s, want 202", status, body)
	}
	var pending struct{ Pending string }
	if err := json.Unmarshal([]byte(body), &pending); err != nil || pending.Pending == "" {
		t.Fatalf("pending login in %s: %v", body, err)
	}
	if status, _ := e.do(t, "GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("GET /api/me after the password alone = %d, want 401", status)
	}
	return pending.Pending
}

func (e *testEnv) codeStep(t *testing.T, pending, code string) (int, string) {
	t.Helper()
	return e.do(t, "POST", "/api/sessions/two-step", `{"pending":"`+pending+`","code":"`+code+`"}`)
}

func (e *testEnv) auditActions(t *testing.T) []string {
	t.Helper()
	events, err := e.store.AuditEvents(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, ev := range events {
		actions = append(actions, ev.Action+":"+ev.Detail)
	}
	return actions
}

func TestTwoStepLoginThroughTheAPI(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "GET", "/api/me/two-step", ""); status != http.StatusOK || !strings.Contains(body, `"enabled":false`) {
		t.Fatalf("status before = %d %s", status, body)
	}
	secret, step, codes := env.enableTwoStep(t)

	pending := env.passwordStep(t)
	if status, body := env.codeStep(t, pending, "000000"); status != http.StatusUnauthorized || !strings.Contains(body, "invalid_code") {
		t.Fatalf("wrong code = %d %s", status, body)
	}
	// The first code used this step; the next one is accepted already.
	next := totp.Code(secret, step+1)
	if status, body := env.codeStep(t, pending, next); status != http.StatusCreated || !strings.Contains(body, `"username":"stijn"`) {
		t.Fatalf("code step = %d %s", status, body)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusOK {
		t.Fatalf("GET /api/me after the code = %d", status)
	}

	pending = env.passwordStep(t)
	if status, body := env.codeStep(t, pending, next); status != http.StatusUnauthorized || !strings.Contains(body, "invalid_code") {
		t.Fatalf("replayed code = %d %s", status, body)
	}
	if status, body := env.codeStep(t, pending, strings.ToUpper(codes[0])); status != http.StatusCreated {
		t.Fatalf("recovery code = %d %s", status, body)
	}
	if status, body := env.codeStep(t, "made-up", "123456"); status != http.StatusUnauthorized || !strings.Contains(body, "login_expired") {
		t.Fatalf("unknown pending login = %d %s", status, body)
	}
	if status, body := env.do(t, "GET", "/api/me/two-step", ""); status != http.StatusOK || body != "{\"enabled\":true,\"recoveryCodesLeft\":9}\n" {
		t.Fatalf("status after = %d %s", status, body)
	}

	actions := strings.Join(env.auditActions(t), "\n")
	for _, want := range []string{"two_step_enabled:", "login_code_failed:right password, wrong code", "login:with a two-step code", "recovery_code_used:9 left"} {
		if !strings.Contains(actions, want) {
			t.Errorf("audit log lacks %q:\n%s", want, actions)
		}
	}
}

func TestTwoStepTurnsOffWithPasswordAndCode(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	_, _, codes := env.enableTwoStep(t)
	if status, body := env.do(t, "POST", "/api/me/two-step/enrolment", `{"password":"`+testPassword+`"}`); status != http.StatusConflict {
		t.Fatalf("enrolment while on = %d %s", status, body)
	}
	if status, body := env.do(t, "DELETE", "/api/me/two-step", `{"password":"wrong password!","code":"`+codes[1]+`"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"password"`) {
		t.Fatalf("turn off with a wrong password = %d %s", status, body)
	}
	if status, body := env.do(t, "DELETE", "/api/me/two-step", `{"password":"`+testPassword+`","code":"000000"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"code"`) {
		t.Fatalf("turn off with a wrong code = %d %s", status, body)
	}
	if status, body := env.do(t, "DELETE", "/api/me/two-step", `{"password":"`+testPassword+`","code":"`+codes[1]+`"}`); status != http.StatusNoContent {
		t.Fatalf("turn off = %d %s", status, body)
	}
	env.do(t, "DELETE", "/api/sessions/current", "")
	if status, body := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"`+testPassword+`"}`); status != http.StatusCreated {
		t.Fatalf("login after turning off = %d %s", status, body)
	}
	actions := strings.Join(env.auditActions(t), "\n")
	for _, want := range []string{"two_step_change_failed:wrong password", "two_step_change_failed:wrong code", "two_step_disabled:with a recovery code"} {
		if !strings.Contains(actions, want) {
			t.Errorf("audit log lacks %q:\n%s", want, actions)
		}
	}
}

func TestTwoStepFirstCodeIsChecked(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if status, body := env.do(t, "PUT", "/api/me/two-step", `{"code":"123456"}`); status != http.StatusConflict || !strings.Contains(body, "no_two_step_setup") {
		t.Fatalf("turn on without a setup = %d %s", status, body)
	}
	if status, body := env.do(t, "POST", "/api/me/two-step/enrolment", `{"password":"wrong password!"}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("enrolment with a wrong password = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/me/two-step/enrolment", `{"password":"`+testPassword+`"}`); status != http.StatusCreated {
		t.Fatalf("enrolment = %d", status)
	}
	if status, body := env.do(t, "PUT", "/api/me/two-step", `{"code":"000000"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"code"`) {
		t.Fatalf("turn on with a wrong code = %d %s", status, body)
	}
	// Still off: the password alone logs in.
	env.do(t, "DELETE", "/api/sessions/current", "")
	if status, _ := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"`+testPassword+`"}`); status != http.StatusCreated {
		t.Fatalf("login during setup = %d", status)
	}
}

func TestWrongTwoStepCodesAreRateLimited(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	secret, step, _ := env.enableTwoStep(t)
	pending := env.passwordStep(t)
	for range 5 {
		if status, body := env.codeStep(t, pending, "000000"); status != http.StatusUnauthorized {
			t.Fatalf("wrong code = %d %s", status, body)
		}
	}
	if status, body := env.codeStep(t, pending, totp.Code(secret, step+1)); status != http.StatusTooManyRequests {
		t.Fatalf("right code after 5 wrong ones = %d %s, want 429", status, body)
	}
}

func TestTwoStepRoutesNeedLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.do(t, "DELETE", "/api/sessions/current", "")
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/me/two-step", ""},
		{"POST", "/api/me/two-step/enrolment", `{"password":"x"}`},
		{"PUT", "/api/me/two-step", `{"code":"123456"}`},
		{"DELETE", "/api/me/two-step", `{"password":"x","code":"123456"}`},
	} {
		if status, _ := env.do(t, route.method, route.path, route.body); status != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route.method, route.path, status)
		}
	}
}
