package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/store"
)

func TestResetPasswordCommand(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	authService, err := auth.NewService(db, "setup-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authService.Setup(ctx, "setup-token", auth.Credentials{Username: "stijn", Password: "correct horse battery"}, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	env := func(key string) string {
		if key == "GDASH_DATA_DIR" {
			return dir
		}
		return ""
	}
	var out bytes.Buffer
	if err := resetPassword(ctx, env, "stijn", &out); err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(out.String(), "\n", 2)[0]
	password := strings.TrimPrefix(first, "New password for stijn: ")
	if password == first || password == "" {
		t.Fatalf("output %q does not give the new password", out.String())
	}

	db, err = store.Open(ctx, filepath.Join(dir, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	authService, _ = auth.NewService(db, "")
	if _, _, err := authService.Login(ctx, auth.Credentials{Username: "stijn", Password: password}, "127.0.0.1"); err != nil {
		t.Fatalf("login with the printed password: %v", err)
	}
	events, err := db.AuditEvents(ctx, 10)
	if err != nil || len(events) == 0 || events[0].Action != auditPasswordReset || events[0].Detail != "stijn" {
		t.Fatalf("newest audit event = %+v, %v; want the reset", events, err)
	}

	if err := resetPassword(ctx, env, "nobody", &out); err == nil || !strings.Contains(err.Error(), "no user") {
		t.Fatalf("reset of an unknown user: err = %v", err)
	}
	empty := func(string) string { return filepath.Join(dir, "missing") }
	if err := resetPassword(ctx, empty, "stijn", &out); err == nil || !strings.Contains(err.Error(), "GDASH_DATA_DIR") {
		t.Fatalf("reset without a database: err = %v", err)
	}
}
