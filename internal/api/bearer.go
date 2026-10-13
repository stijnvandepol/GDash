package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/stijnvandepol/gdash/internal/apitoken"
)

// sender is who makes a request that an admin session or an API token may
// make. key identifies it for rate limiting; name is for the log.
type sender struct{ key, name string }

type senderKey struct{}

func senderFrom(ctx context.Context) sender {
	s, _ := ctx.Value(senderKey{}).(sender)
	return s
}

// requireUserOrToken lets a request through with a valid API token or, when
// it carries no Authorization header, a valid session. Only routes meant for
// automations use it; every other route takes sessions only.
//
// A request with an Authorization header is judged on that header alone: a
// browser cookie sent along is ignored, so the cookie can never turn a
// failed token into an admin request. Cross-site browser requests never get
// here: http.CrossOriginProtection in Handler refuses them first, with or
// without a token. An automation is no browser and sends neither
// Sec-Fetch-Site nor Origin, which that check lets through.
func (s *Server) requireUserOrToken(next http.HandlerFunc) http.Handler {
	byUser := s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r.Context())
		next(w, r.WithContext(context.WithValue(r.Context(), senderKey{}, sender{key: "user|" + u.Username, name: u.Username})))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			byUser.ServeHTTP(w, r)
			return
		}
		secret, ok := bearerToken(header)
		if !ok {
			writeBearerError(w, "invalid_token", "Send the API token as 'Authorization: Bearer <token>'.")
			return
		}
		stored, err := s.APITokens.Authenticate(r.Context(), secret, clientIP(r))
		switch {
		case errors.Is(err, apitoken.ErrRateLimited):
			w.Header().Set("Retry-After", strconv.Itoa(15*60))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many invalid API tokens. Wait 15 minutes and try again.")
			return
		case errors.Is(err, apitoken.ErrInvalidToken):
			// Anonymous: the token names no one, and it is never logged.
			s.audit(r, "", auditAPITokenRejected, "")
			writeBearerError(w, "invalid_token", "This API token is not valid, or it was revoked. Make a new one under Integrations, API tokens.")
			return
		case err != nil:
			s.internalError(w, "authenticate api token", err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), senderKey{}, sender{key: "token|" + strconv.FormatInt(stored.ID, 10), name: "API token " + stored.Name})))
	})
}

// bearerToken returns the token of an "Authorization: Bearer <token>" header.
func bearerToken(header string) (string, bool) {
	scheme, secret, found := strings.Cut(header, " ")
	secret = strings.TrimSpace(secret)
	if !found || !strings.EqualFold(scheme, "Bearer") || secret == "" {
		return "", false
	}
	return secret, true
}

func writeBearerError(w http.ResponseWriter, code, message string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, code, message)
}
