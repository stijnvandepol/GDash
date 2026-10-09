package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/stijnvandepol/nookboard/internal/store"
	"github.com/stijnvandepol/nookboard/internal/validation"
)

// listAPITokens handles GET /api/api-tokens: {"tokens": [{id, name,
// createdAt, lastUsedAt}]}, oldest first. Never the tokens themselves.
func (s *Server) listAPITokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.Store.APITokens(r.Context())
	if err != nil {
		s.internalError(w, "list api tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// postAPIToken handles POST /api/api-tokens {name}: 201 with the token and,
// once, its secret in "token". 409 at 10 tokens, 422 for an invalid name.
func (s *Server) postAPIToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	made, err := s.APITokens.Create(r.Context(), body.Name)
	switch {
	case validation.Fields(err) != nil:
		writeValidationError(w, validation.Fields(err))
		return
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "The limit of 10 API tokens is reached. Revoke one you no longer use.")
		return
	case err != nil:
		s.internalError(w, "create api token", err)
		return
	}
	// The name only: the token is a secret.
	s.auditUser(r, auditAPITokenCreated, made.Name)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, made)
}

// deleteAPIToken handles DELETE /api/api-tokens/{id}: revokes the token (204)
// or answers 404. Automations that use it get 401 from then on.
func (s *Server) deleteAPIToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "This API token does not exist.")
		return
	}
	name, err := s.Store.DeleteAPIToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This API token does not exist.")
		return
	}
	if err != nil {
		s.internalError(w, "delete api token", err)
		return
	}
	s.auditUser(r, auditAPITokenRevoked, name)
	w.WriteHeader(http.StatusNoContent)
}
