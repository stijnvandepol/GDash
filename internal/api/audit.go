package api

import (
	"net/http"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/store"
)

// Actions in the audit log. web/src/admin/audit.ts describes them.
const (
	auditAccountCreated         = "account_created"
	auditLogin                  = "login"
	auditLoginFailed            = "login_failed"
	auditSetupFailed            = "setup_failed"
	auditAPITokenRejected       = "api_token_rejected"
	auditLogout                 = "logout"
	auditPasswordChanged        = "password_changed"
	auditPasswordWrong          = "password_change_failed"
	auditBackupDownloaded       = "backup_downloaded"
	auditBackupRefused          = "backup_failed"
	auditScreenPaired           = "screen_paired"
	auditScreensUnpaired        = "screens_unpaired"
	auditDisplayCreated         = "display_created"
	auditDisplayUpdated         = "display_updated"
	auditDisplayDeleted         = "display_deleted"
	auditCastTakenOver          = "cast_taken_over"
	auditDashboardCreated       = "dashboard_created"
	auditDashboardUpdated       = "dashboard_updated"
	auditDashboardDeleted       = "dashboard_deleted"
	auditLocationChanged        = "location_changed"
	auditLocationCleared        = "location_cleared"
	auditLanguageChanged        = "language_changed"
	auditNewsFeedAdded          = "news_feed_added"
	auditNewsFeedRemoved        = "news_feed_removed"
	auditCalendarAdded          = "calendar_added"
	auditCalendarRemoved        = "calendar_removed"
	auditCalendarChanged        = "calendar_changed"
	auditPhotoAdded             = "photo_added"
	auditPhotoRemoved           = "photo_removed"
	auditHomeAssistantConnected = "homeassistant_connected"
	auditHomeAssistantRemoved   = "homeassistant_removed"
	auditHomeyConnected         = "homey_connected"
	auditHomeyRemoved           = "homey_removed"
	auditAPITokenCreated        = "api_token_created"
	auditAPITokenRevoked        = "api_token_revoked"
	auditLoginCodeFailed        = "login_code_failed"
	auditRecoveryCodeUsed       = "recovery_code_used"
	auditTwoStepEnabled         = "two_step_enabled"
	auditTwoStepDisabled        = "two_step_disabled"
	auditTwoStepChangeFailed    = "two_step_change_failed"
	auditTwoStepLocked          = "two_step_locked"
	auditCustomSourceAdded      = "custom_source_added"
	auditCustomSourceChanged    = "custom_source_changed"
	auditCustomSourceRemoved    = "custom_source_removed"
	defaultAuditPage            = 100
	maxAuditDetailCharacters    = 200
)

// audit records what actor did. A failure is logged and not returned: the
// action itself has already happened and its response must still go out.
// detail never holds secrets such as calendar addresses.
func (s *Server) audit(r *http.Request, actor, action, detail string) {
	if len([]rune(detail)) > maxAuditDetailCharacters {
		detail = string([]rune(detail)[:maxAuditDetailCharacters])
	}
	e := store.AuditEvent{At: s.Now().UTC(), Actor: actor, Action: action, Detail: detail, IP: clientIP(r)}
	if err := s.Store.AddAuditEvent(r.Context(), e); err != nil {
		s.Logger.Error("could not write the audit log", "action", action, "error", err)
	}
}

// auditUser records an action of the logged-in admin.
func (s *Server) auditUser(r *http.Request, action, detail string) {
	s.audit(r, userFrom(r.Context()).Username, action, detail)
}

// getAuditEvents handles GET /api/audit-events?limit=100: {"events": [...]},
// newest first, at most 200.
func (s *Server) getAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit := defaultAuditPage
	if text := r.URL.Query().Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive whole number.")
			return
		}
		limit = n
	}
	events, err := s.Store.AuditEvents(r.Context(), limit)
	if err != nil {
		s.internalError(w, "list audit events", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
