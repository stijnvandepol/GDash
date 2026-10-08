package api

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/stijnvandepol/daycast/internal/auth"
	"github.com/stijnvandepol/daycast/internal/validation"
)

const sessionCookie = "daycast_session"

type userKey struct{}

// userFrom returns the user that requireUser stored in the request context.
func userFrom(ctx context.Context) auth.User {
	u, _ := ctx.Value(userKey{}).(auth.User)
	return u
}

// requireUser lets a request through only with a valid session.
func (s *Server) requireUser(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not_logged_in", "Log in to continue.")
			return
		}
		u, err := s.Auth.Authenticate(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrNoSession) {
			writeError(w, http.StatusUnauthorized, "not_logged_in", "Your session has ended. Log in again.")
			return
		}
		if err != nil {
			s.internalError(w, "authenticate", err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

// setSessionCookie stores the session token. Secure is only set on HTTPS,
// because browsers drop Secure cookies on the plain-HTTP home network where
// most installs run.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// clientIP is the address the request came from. Proxy headers are not
// trusted: on a home network they are trivial to forge.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// getSetup handles GET /api/setup: {"needed": true} while no account exists.
func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	needed, err := s.Auth.SetupNeeded(r.Context())
	if err != nil {
		s.internalError(w, "check setup", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needed": needed})
}

// postSetup handles POST /api/setup {setupToken, username, password}: creates
// the first account, logs it in and answers 201 with the user. 403 for a
// wrong token, 409 when setup is done, 422 for invalid fields, 429 after
// too many attempts.
func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SetupToken string `json:"setupToken"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, user, err := s.Auth.Setup(r.Context(), body.SetupToken, auth.Credentials{Username: body.Username, Password: body.Password}, clientIP(r))
	if !s.writeAuthError(w, "setup", err) {
		return
	}
	s.audit(r, user.Username, auditAccountCreated, "")
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, user)
}

// postSession handles POST /api/sessions {username, password}: logs in, sets
// the session cookie and answers 201 with the user. 401 for wrong
// credentials, 429 after too many attempts.
func (s *Server) postSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, user, err := s.Auth.Login(r.Context(), auth.Credentials{Username: body.Username, Password: body.Password}, clientIP(r))
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		// No username: people sometimes type their password there.
		s.audit(r, "", auditLoginFailed, "wrong username or password")
	case errors.Is(err, auth.ErrRateLimited):
		s.audit(r, "", auditLoginFailed, "blocked after too many attempts")
	}
	if !s.writeAuthError(w, "login", err) {
		return
	}
	s.audit(r, user.Username, auditLogin, "")
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, user)
}

// deleteSession handles DELETE /api/sessions/current: logs out (204).
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		if err := s.Auth.Logout(r.Context(), cookie.Value); err != nil {
			s.internalError(w, "logout", err)
			return
		}
	}
	s.auditUser(r, auditLogout, "")
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// getMe handles GET /api/me: the logged-in user.
func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

// putPassword handles PUT /api/me/password {currentPassword, newPassword}:
// 204, and every other session of the account ends. A wrong current
// password is a 422 on that field, not a 401: the session itself is fine.
func (s *Server) putPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not_logged_in", "Log in to continue.")
		return
	}
	err = s.Auth.ChangePassword(r.Context(), cookie.Value, body.CurrentPassword, body.NewPassword, clientIP(r))
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		s.auditUser(r, auditPasswordWrong, "wrong current password")
		writeValidationError(w, []validation.FieldError{{Field: "currentPassword", Message: "this is not your current password"}})
	case errors.Is(err, auth.ErrNoSession):
		writeError(w, http.StatusUnauthorized, "not_logged_in", "Your session has ended. Log in again.")
	case s.writeAuthError(w, "change password", err):
		s.auditUser(r, auditPasswordChanged, "other sessions logged out")
		w.WriteHeader(http.StatusNoContent)
	}
}

// getSystem handles GET /api/system: {"version"}. Only for admins, so the
// version is no help to someone scanning the network for old releases.
func (s *Server) getSystem(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.Version})
}

// writeAuthError translates auth errors to responses. It returns true when
// err is nil and the handler may continue.
func (s *Server) writeAuthError(w http.ResponseWriter, op string, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "The username or password is not correct.")
	case errors.Is(err, auth.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many failed attempts. Wait 15 minutes and try again.")
	case errors.Is(err, auth.ErrInvalidSetupToken):
		writeError(w, http.StatusForbidden, "invalid_setup_token", "The setup token is not correct. It is printed in the Daycast log.")
	case errors.Is(err, auth.ErrSetupDone):
		writeError(w, http.StatusConflict, "setup_done", "Daycast is already set up. Log in instead.")
	case validation.Fields(err) != nil:
		writeValidationError(w, validation.Fields(err))
	default:
		s.internalError(w, op, err)
	}
	return false
}
