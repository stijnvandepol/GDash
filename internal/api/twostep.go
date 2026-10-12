package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// postSessionCode handles POST /api/sessions/two-step {pending, code}:
// finishes a login that POST /api/sessions answered with 202, with a code
// from the authenticator app or a recovery code. 201 with the user and the
// session cookie; 401 invalid_code (the pending login stays, try again),
// 401 login_expired (enter the password again), 429 after too many wrong
// codes.
func (s *Server) postSessionCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pending string `json:"pending"`
		Code    string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, user, use, err := s.Auth.FinishLogin(r.Context(), body.Pending, body.Code, clientIP(r))
	switch {
	case errors.Is(err, auth.ErrWrongCode):
		// The password was right, so this is either a slip or someone who
		// knows it: worth seeing in the activity log.
		s.audit(r, user.Username, auditLoginCodeFailed, "right password, wrong code")
		switch {
		case errors.Is(err, auth.ErrTwoStepLocked):
			s.audit(r, user.Username, auditTwoStepLocked, twoStepLockedDetail)
			writeCodeRefused(w, err)
		case errors.Is(err, auth.ErrLoginExpired):
			writeError(w, http.StatusUnauthorized, "login_expired", "Too many wrong codes. Enter your password again.")
		default:
			writeError(w, http.StatusUnauthorized, "invalid_code", "The code is not correct. Use the newest code from your app, or a recovery code.")
		}
		return
	case errors.Is(err, auth.ErrLoginExpired):
		writeError(w, http.StatusUnauthorized, "login_expired", "The login took too long. Enter your password again.")
		return
	case writeCodeRefused(w, err), !s.writeAuthError(w, "finish login", err):
		return
	}
	if use.Recovery {
		s.audit(r, user.Username, auditRecoveryCodeUsed, fmt.Sprintf("%d left", use.RecoveryLeft))
	} else {
		s.audit(r, user.Username, auditLogin, "with a two-step code")
	}
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, user)
}

// getTwoStep handles GET /api/me/two-step: {enabled, recoveryCodesLeft}.
func (s *Server) getTwoStep(w http.ResponseWriter, r *http.Request) {
	status, err := s.Auth.TwoStep(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.internalError(w, "read two-step login", err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// postTwoStepEnrolment handles POST /api/me/two-step/enrolment {password}:
// 201 {secret, uri, qr} with a new secret for the authenticator app, as
// text, as an otpauth:// address and as an SVG QR code. The secret is not
// returned again. Two-step login stays off until PUT /api/me/two-step.
// 422 on "password", 409 two_step_on, 429 rate_limited.
func (s *Server) postTwoStepEnrolment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	enrolment, err := s.Auth.StartTwoStep(r.Context(), sessionToken(r), body.Password, clientIP(r))
	if !s.writeTwoStepError(w, r, "start two-step login", err) {
		return
	}
	writeJSON(w, http.StatusCreated, enrolment)
}

// putTwoStep handles PUT /api/me/two-step {code}: turns two-step login on
// with a first code of the new secret and answers 200 {recoveryCodes}, the
// only time they are shown. Every other session of the account ends. 422
// on "code", 409 no_two_step_setup, two_step_setup_replaced (a newer setup
// replaced the secret meanwhile) or two_step_on, 429 rate_limited.
func (s *Server) putTwoStep(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	codes, err := s.Auth.EnableTwoStep(r.Context(), sessionToken(r), body.Code, clientIP(r))
	if !s.writeTwoStepError(w, r, "turn on two-step login", err) {
		return
	}
	s.auditUser(r, auditTwoStepEnabled, "other sessions logged out")
	writeJSON(w, http.StatusOK, map[string][]string{"recoveryCodes": codes})
}

// deleteTwoStep handles DELETE /api/me/two-step {password, code}: turns
// two-step login off (204). The code may be a recovery code. 422 on
// "password" or "code", 409 two_step_off, 429 rate_limited.
func (s *Server) deleteTwoStep(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	use, err := s.Auth.DisableTwoStep(r.Context(), sessionToken(r), body.Password, body.Code, clientIP(r))
	if !s.writeTwoStepError(w, r, "turn off two-step login", err) {
		return
	}
	detail := ""
	if use.Recovery {
		detail = "with a recovery code"
	}
	s.auditUser(r, auditTwoStepDisabled, detail)
	w.WriteHeader(http.StatusNoContent)
}

// writeTwoStepError answers the errors of changing two-step login, and
// records wrong passwords and codes. It returns true when err is nil.
func (s *Server) writeTwoStepError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		s.auditUser(r, auditTwoStepChangeFailed, "wrong password")
		writeValidationError(w, []validation.FieldError{{Field: "password", Message: "this is not your password"}})
	case errors.Is(err, auth.ErrWrongCode):
		s.auditUser(r, auditTwoStepChangeFailed, "wrong code")
		if errors.Is(err, auth.ErrTwoStepLocked) {
			s.auditUser(r, auditTwoStepLocked, twoStepLockedDetail)
			writeCodeRefused(w, err)
			break
		}
		writeValidationError(w, []validation.FieldError{{Field: "code", Message: "this code is not correct; use the newest one from your app"}})
	case writeCodeRefused(w, err):
	case errors.Is(err, auth.ErrNoSession):
		writeError(w, http.StatusUnauthorized, "not_logged_in", "Your session has ended. Log in again.")
	case errors.Is(err, auth.ErrTwoStepOn):
		writeError(w, http.StatusConflict, "two_step_on", "Two-step login is already on.")
	case errors.Is(err, auth.ErrTwoStepOff):
		writeError(w, http.StatusConflict, "two_step_off", "Two-step login is already off.")
	case errors.Is(err, auth.ErrNoSetup):
		writeError(w, http.StatusConflict, "no_two_step_setup", "Start setting up two-step login again.")
	case errors.Is(err, auth.ErrSetupReplaced):
		writeError(w, http.StatusConflict, "two_step_setup_replaced",
			"Two-step login was set up again in another window. Start again, and scan only the newest QR code.")
	default:
		return s.writeAuthError(w, op, err)
	}
	return false
}

// twoStepLockedDetail is the audit detail of reaching the stop on wrong
// codes.
const twoStepLockedDetail = "too many wrong codes in a row; a recovery code or gdash reset-password lifts it"

// writeCodeRefused answers a code that was turned away because of the wrong
// codes before it: 429 rate_limited with Retry-After while it has to wait,
// 429 two_step_locked at the stop. It returns false for any other error.
func writeCodeRefused(w http.ResponseWriter, err error) bool {
	var wait *auth.CodeWait
	switch {
	case errors.As(err, &wait):
		seconds := int(math.Ceil(wait.Wait.Seconds()))
		minutes := (seconds + 59) / 60
		unit := "minutes"
		if minutes == 1 {
			unit = "minute"
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeError(w, http.StatusTooManyRequests, "rate_limited", fmt.Sprintf("Too many wrong codes. Try again in %d %s.", minutes, unit))
	case errors.Is(err, auth.ErrTwoStepLocked):
		writeError(w, http.StatusTooManyRequests, "two_step_locked",
			"Too many wrong codes: codes from the app are refused. Use a recovery code, or reset the password on the server with gdash reset-password.")
	default:
		return false
	}
	return true
}

// sessionToken is the session cookie of a request that requireUser let
// through.
func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}
