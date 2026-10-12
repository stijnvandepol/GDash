package main

import (
	"bytes"
	"context"
	"encoding/base32"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/totp"
)

const resetTestPassword = "correct horse battery"

// newResetTestAccount makes a data directory with the account "stijn", and
// with two-step login on when twoStep is set.
func newResetTestAccount(t *testing.T, twoStep bool) string {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	box, err := secret.LoadOrCreateKey(filepath.Join(dir, secretKeyName))
	if err != nil {
		t.Fatal(err)
	}
	authService, err := auth.NewService(db, box, "setup-token")
	if err != nil {
		t.Fatal(err)
	}
	session, _, err := authService.Setup(ctx, "setup-token", auth.Credentials{Username: "stijn", Password: resetTestPassword}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !twoStep {
		return dir
	}
	enrolment, err := authService.StartTwoStep(ctx, session, resetTestPassword, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrolment.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authService.EnableTwoStep(ctx, session, totp.Code(key, totp.Step(time.Now())), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func dataDirEnv(dir string) func(string) string {
	return func(key string) string {
		if key == "GDASH_DATA_DIR" {
			return dir
		}
		return ""
	}
}

// loginAfterReset logs in with the password the reset printed, and returns
// the newest audit events.
func loginAfterReset(t *testing.T, dir, output string) []store.AuditEvent {
	t.Helper()
	ctx := context.Background()
	first := strings.SplitN(output, "\n", 2)[0]
	password := strings.TrimPrefix(first, "New password for stijn: ")
	if password == first || password == "" {
		t.Fatalf("output %q does not give the new password", output)
	}
	db, err := store.Open(ctx, filepath.Join(dir, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	box, err := secret.LoadOrCreateKey(filepath.Join(dir, secretKeyName))
	if err != nil {
		t.Fatal(err)
	}
	authService, _ := auth.NewService(db, box, "")
	if _, _, err := authService.Login(ctx, auth.Credentials{Username: "stijn", Password: password}, "127.0.0.1"); err != nil {
		t.Fatalf("login with the printed password: %v", err)
	}
	events, err := db.AuditEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestResetPasswordCommand(t *testing.T) {
	ctx := context.Background()
	dir := newResetTestAccount(t, false)
	var out bytes.Buffer
	if err := resetPassword(ctx, dataDirEnv(dir), "stijn", &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Two-step") {
		t.Fatalf("output %q mentions two-step login, which was never on", out.String())
	}
	events := loginAfterReset(t, dir, out.String())
	if len(events) == 0 || events[0].Action != auditPasswordReset || events[0].Detail != "stijn" {
		t.Fatalf("newest audit event = %+v; want the reset", events)
	}

	if err := resetPassword(ctx, dataDirEnv(dir), "nobody", &out); err == nil || !strings.Contains(err.Error(), "no user") {
		t.Fatalf("reset of an unknown user: err = %v", err)
	}
	empty := func(string) string { return filepath.Join(dir, "missing") }
	if err := resetPassword(ctx, empty, "stijn", &out); err == nil || !strings.Contains(err.Error(), "GDASH_DATA_DIR") {
		t.Fatalf("reset without a database: err = %v", err)
	}
}

// The reset is the way back in for an admin who lost the phone and the
// recovery codes, so it turns two-step login off.
func TestResetPasswordCommandTurnsTwoStepOff(t *testing.T) {
	dir := newResetTestAccount(t, true)
	var out bytes.Buffer
	if err := resetPassword(context.Background(), dataDirEnv(dir), "stijn", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Two-step login is off") {
		t.Fatalf("output %q does not say two-step login is off", out.String())
	}
	events := loginAfterReset(t, dir, out.String())
	if len(events) < 2 || events[0].Action != auditTwoStepReset || events[1].Action != auditPasswordReset {
		t.Fatalf("newest audit events = %+v; want the reset and two-step login turned off", events)
	}
}
