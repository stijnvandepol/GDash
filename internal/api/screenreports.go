package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/screenreport"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// reportRetrySeconds is what a screen over its report limit is told to wait.
const reportRetrySeconds = 600

// postScreenReport handles POST /api/displays/{display}/screen-reports with
// what a screen found out about its own browser (screenreport.Report): 204
// when it is kept. Only a screen paired with the display may report, with
// its display cookie; an admin previewing the display may not. 401 for an
// unpaired screen, 403 for a screen of another display, 413 above 8 KB, 422
// naming every invalid field and 429 when the screen reports too often. A
// report with reason "hidden" from the TV a cast keeper opened is passed to
// that keeper, which restarts the receiver when the TV is free.
func (s *Server) postScreenReport(w http.ResponseWriter, r *http.Request) {
	displayID := r.PathValue("display")
	if !validDisplayID.MatchString(displayID) {
		http.NotFound(w, r)
		return
	}
	screen, err := s.screenFromCookie(r)
	if errors.Is(err, pairing.ErrInvalidToken) {
		writeError(w, http.StatusUnauthorized, "not_paired", "Pair this screen with the display first.")
		return
	}
	if err != nil {
		s.internalError(w, "authorize screen report", err)
		return
	}
	if screen.DisplayID != displayID {
		writeError(w, http.StatusForbidden, "other_display", "A screen can only report for the display it shows.")
		return
	}
	if r.ContentLength > screenreport.MaxBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "The report is larger than 8 KB.")
		return
	}
	// Counted before the body is read, so a screen that keeps sending
	// broken reports is stopped too.
	if !s.ScreenReports.Allow(displayID, screen.Key) {
		w.Header().Set("Retry-After", strconv.Itoa(reportRetrySeconds))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "This screen reports too often. Wait 10 minutes and try again.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, screenreport.MaxBodyBytes)
	var report screenreport.Report
	if !decodeJSON(w, r, &report) {
		return
	}
	if err := screenreport.Validate(report); err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	s.ScreenReports.Put(displayID, screen.Key, screen.Kind, report)
	// Only the TV GDash casts to can be brought back by restarting the
	// receiver; a tablet that someone puts away is not a screensaver.
	if report.Reason == screenreport.ReasonHidden && screen.Kind == store.TokenCast && s.ScreenHidden != nil {
		s.ScreenHidden(displayID)
	}
	w.WriteHeader(http.StatusNoContent)
}
