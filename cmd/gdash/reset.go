package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/store"
)

// auditPasswordReset is the audit action of a reset from the command line;
// web/src/admin/audit.ts describes it.
const auditPasswordReset = "password_reset"

// resetPassword gives username a new random password and prints it to out,
// for an admin who lost theirs. It needs the data directory, so only someone
// who can already run commands on the server can use it.
func resetPassword(ctx context.Context, getenv func(string) string, username string, out io.Writer) error {
	dataDir := valueOr(getenv("GDASH_DATA_DIR"), "data")
	path := filepath.Join(dataDir, databaseName)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no GDash database at %s; set GDASH_DATA_DIR to the data directory", path)
	}
	db, err := store.Open(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()
	// The setup token is not used: an account exists, or there is nothing
	// to reset.
	authService, err := auth.NewService(db, "")
	if err != nil {
		return err
	}
	password, err := authService.ResetPassword(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("there is no user %q", username)
	}
	if err != nil {
		return err
	}
	event := store.AuditEvent{At: time.Now().UTC(), Actor: "command line", Action: auditPasswordReset, Detail: username}
	if err := db.AddAuditEvent(ctx, event); err != nil {
		return fmt.Errorf("the password was reset, but the activity log could not record it: %w", err)
	}
	_, err = fmt.Fprintf(out, "New password for %s: %s\nEvery browser was logged out. Log in and choose a password of your own under Settings, Account.\n", username, password)
	return err
}
