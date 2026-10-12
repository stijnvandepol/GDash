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

// Audit actions of a reset from the command line; web/src/admin/audit.ts
// describes them.
const (
	auditPasswordReset = "password_reset"
	auditTwoStepReset  = "two_step_reset"
)

// resetPassword gives username a new random password, turns two-step login
// off and prints the password to out, for an admin who lost the password,
// or the phone and the recovery codes. It needs the data directory, so only
// someone who can already run commands on the server can use it.
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
	// The key file is not opened: the reset does not need it, and this
	// command may run as another OS user than the server.
	password, twoStepWasOn, err := auth.ResetPassword(ctx, db, username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("there is no user %q", username)
	}
	if err != nil {
		return err
	}
	events := []store.AuditEvent{{At: time.Now().UTC(), Actor: "command line", Action: auditPasswordReset, Detail: username}}
	if twoStepWasOn {
		events = append(events, store.AuditEvent{At: time.Now().UTC(), Actor: "command line", Action: auditTwoStepReset, Detail: username})
	}
	for _, event := range events {
		if err := db.AddAuditEvent(ctx, event); err != nil {
			return fmt.Errorf("the password was reset, but the activity log could not record it: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "New password for %s: %s\nEvery browser was logged out.", username, password); err != nil {
		return err
	}
	if twoStepWasOn {
		if _, err := fmt.Fprint(out, " Two-step login is off; turn it on again under Settings, Account."); err != nil {
			return err
		}
	}
	_, err = fmt.Fprint(out, " Log in and choose a password of your own under Settings, Account.\n")
	return err
}
