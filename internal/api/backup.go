package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// BackupWriter writes a backup of the data directory as a zip file.
type BackupWriter interface {
	WriteZip(ctx context.Context, w io.Writer) error
}

// postBackup handles POST /api/backup {password}: a zip with the database,
// the secret key and the photos, to download. The key decrypts the
// calendar addresses and the Home Assistant token, so the password is asked
// again and every download is in the activity log. 422 on "password" for a
// wrong one, 429 after too many.
func (s *Server) postBackup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not_logged_in", "Log in to continue.")
		return
	}
	_, err = s.Auth.Confirm(r.Context(), cookie.Value, body.Password, clientIP(r))
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		s.auditUser(r, auditBackupRefused, "wrong password")
		writeValidationError(w, []validation.FieldError{{Field: "password", Message: "this is not your password"}})
		return
	case errors.Is(err, auth.ErrNoSession):
		writeError(w, http.StatusUnauthorized, "not_logged_in", "Your session has ended. Log in again.")
		return
	case !s.writeAuthError(w, "confirm password", err):
		return
	}

	s.auditUser(r, auditBackupDownloaded, "")
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", `attachment; filename="gdash-backup-`+s.Now().Format("2006-01-02")+`.zip"`)
	// The answer streams: photos can add up to hundreds of megabytes. A
	// failure halfway can no longer change the status, so it ends the
	// download, and the browser reports it as failed.
	if err := s.Backup.WriteZip(r.Context(), w); err != nil {
		s.Logger.Error("backup download failed", "error", err)
		panic(http.ErrAbortHandler)
	}
}
