package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/pairing"
)

const displayCookie = "gdash_display"

// displayCookieMaxAge keeps a paired screen paired for a year; unpairing in
// the web interface revokes it earlier.
const displayCookieMaxAge = 365 * 24 * 60 * 60

// postPairingRequest handles POST /api/pairing-requests {displayId}: an
// unpaired screen asks for a code to show. 201 with {code, pollToken,
// expiresAt}; 404 for an unknown display; 429 when the address asks too
// often.
func (s *Server) postPairingRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayID string `json:"displayId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	req, err := s.Pairing.Begin(r.Context(), body.DisplayID, clientIP(r))
	if !s.writePairingError(w, "begin pairing", err) {
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

// postDisplaySession handles POST /api/display-sessions with {pollToken}
// (after a code was approved) or {launchToken} (from the URL a cast keeper
// opened). 201 with {displayId} and the display cookie; 202 while a code
// waits for approval; 401 for an unknown or expired token.
func (s *Server) postDisplaySession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PollToken   string `json:"pollToken"`
		LaunchToken string `json:"launchToken"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var displayToken, displayID string
	var err error
	switch {
	case body.PollToken != "" && body.LaunchToken == "":
		displayToken, displayID, err = s.Pairing.Claim(body.PollToken)
	case body.LaunchToken != "" && body.PollToken == "":
		displayToken, displayID, err = s.Pairing.Exchange(r.Context(), body.LaunchToken, clientIP(r))
	default:
		writeError(w, http.StatusBadRequest, "invalid_json", "Send either pollToken or launchToken.")
		return
	}
	if errors.Is(err, pairing.ErrPending) {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}
	if !s.writePairingError(w, "start display session", err) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: displayCookie, Value: displayToken, Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: displayCookieMaxAge,
	})
	writeJSON(w, http.StatusCreated, map[string]string{"displayId": displayID})
}

// getDisplaySession handles GET /api/display-sessions/current: {displayId}
// of the screen's cookie, or 401 when the screen is not paired.
func (s *Server) getDisplaySession(w http.ResponseWriter, r *http.Request) {
	displayID, err := s.displayFromCookie(r)
	if errors.Is(err, pairing.ErrInvalidToken) {
		writeError(w, http.StatusUnauthorized, "not_paired", "This screen is not paired.")
		return
	}
	if err != nil {
		s.internalError(w, "check display session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"displayId": displayID})
}

// postPairing handles POST /api/displays/{display}/pairings {code}: the admin
// approves the code a screen shows. 201 with {displayId}; 422 for a wrong or
// expired code.
func (s *Server) postPairing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	displayID := r.PathValue("display")
	err := s.Pairing.Approve(r.Context(), displayID, body.Code)
	if !s.writePairingError(w, "approve pairing", err) {
		return
	}
	s.auditUser(r, auditScreenPaired, displayID)
	writeJSON(w, http.StatusCreated, map[string]string{"displayId": displayID})
}

// deletePairings handles DELETE /api/displays/{display}/pairings: unpairs
// every screen of the display. 200 with {removed}.
func (s *Server) deletePairings(w http.ResponseWriter, r *http.Request) {
	n, err := s.Pairing.Unpair(r.Context(), r.PathValue("display"))
	if err != nil {
		s.internalError(w, "unpair screens", err)
		return
	}
	s.auditUser(r, auditScreensUnpaired, fmt.Sprintf("%s: %d screens", r.PathValue("display"), n))
	writeJSON(w, http.StatusOK, map[string]int64{"removed": n})
}

// mayShowDisplay reports whether the request may stream displayID: a screen
// paired with that display, or a logged-in user previewing it.
func (s *Server) mayShowDisplay(r *http.Request, displayID string) (bool, error) {
	paired, err := s.displayFromCookie(r)
	if err == nil {
		return paired == displayID, nil
	}
	if !errors.Is(err, pairing.ErrInvalidToken) {
		return false, err
	}
	return s.isAdmin(r)
}

// isAdmin reports whether the request carries a valid admin session.
func (s *Server) isAdmin(r *http.Request) (bool, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false, nil
	}
	_, err = s.Auth.Authenticate(r.Context(), cookie.Value)
	if errors.Is(err, auth.ErrNoSession) {
		return false, nil
	}
	return err == nil, err
}

func (s *Server) displayFromCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(displayCookie)
	if err != nil {
		return "", pairing.ErrInvalidToken
	}
	return s.Pairing.Authenticate(r.Context(), cookie.Value)
}

// writePairingError translates pairing errors. It returns true when err is
// nil and the handler may continue.
func (s *Server) writePairingError(w http.ResponseWriter, op string, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, pairing.ErrUnknownDisplay):
		writeError(w, http.StatusNotFound, "not_found", "This display does not exist.")
	case errors.Is(err, pairing.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many pairing requests. Wait 15 minutes and try again.")
	case errors.Is(err, pairing.ErrInvalidCode):
		writeError(w, http.StatusUnprocessableEntity, "invalid_code", "This code is wrong or has expired. Check the code on the screen.")
	case errors.Is(err, pairing.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid_token", "This pairing has expired. The screen will show a new code.")
	default:
		s.writeStoreError(w, op, err)
	}
	return false
}
