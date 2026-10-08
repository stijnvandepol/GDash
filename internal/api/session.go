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

func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	needed, err := s.Auth.SetupNeeded(r.Context())
	if err != nil {
		s.internalError(w, "check setup", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needed": needed})
}

func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SetupToken string `json:"setupToken"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, user, err := s.Auth.Setup(r.Context(), body.SetupToken, body.Username, body.Password, clientIP(r))
	if !s.writeAuthError(w, "setup", err) {
		return
	}
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) postSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, user, err := s.Auth.Login(r.Context(), body.Username, body.Password, clientIP(r))
	if !s.writeAuthError(w, "login", err) {
		return
	}
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		if err := s.Auth.Logout(r.Context(), cookie.Value); err != nil {
			s.internalError(w, "logout", err)
			return
		}
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
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
