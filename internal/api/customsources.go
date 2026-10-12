package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// CustomFetcher fetches a custom source once, for the Test button.
type CustomFetcher interface {
	Fetch(ctx context.Context, src custom.Source) (json.RawMessage, error)
}

// customTestTimeout bounds a Test; the fetcher's client has its own,
// shorter timeout per request.
const customTestTimeout = 20 * time.Second

// handleCustomSources adds the routes of custom API sources, all for a
// logged-in admin only.
func (s *Server) handleCustomSources(mux *http.ServeMux) {
	mux.Handle("GET /api/custom-sources", s.requireUser(s.listCustomSources))
	mux.Handle("POST /api/custom-sources", s.requireUser(s.postCustomSource))
	mux.Handle("PUT /api/custom-sources/{id}", s.requireUser(s.putCustomSource))
	mux.Handle("DELETE /api/custom-sources/{id}", s.requireUser(s.deleteCustomSource))
	mux.Handle("POST /api/custom-sources/{id}/tests", s.requireUser(s.postCustomSourceTest))
	mux.Handle("POST /api/custom-rule-checks", s.requireUser(s.postCustomRuleCheck))
}

// listCustomSources handles GET /api/custom-sources: {"sources": [...]},
// without secrets; each says whether its authentication secret is set.
func (s *Server) listCustomSources(w http.ResponseWriter, r *http.Request) {
	list, err := s.Settings.CustomSources(r.Context())
	if err != nil {
		s.internalError(w, "list custom sources", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": list})
}

// postCustomSource handles POST /api/custom-sources with a source: 201
// with the source, 422 naming every invalid field, 409 at the limit.
func (s *Server) postCustomSource(w http.ResponseWriter, r *http.Request) {
	var in custom.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	src, err := s.Settings.AddCustomSource(r.Context(), in)
	if err != nil {
		s.writeSettingsError(w, "add custom source", err)
		return
	}
	s.auditUser(r, auditCustomSourceAdded, describeCustomSource(src))
	s.SettingsChanged()
	writeJSON(w, http.StatusCreated, src)
}

// putCustomSource handles PUT /api/custom-sources/{id}: a secret left empty
// keeps the stored one. 200 with the source, 404, or 422.
func (s *Server) putCustomSource(w http.ResponseWriter, r *http.Request) {
	id, ok := customSourceID(w, r)
	if !ok {
		return
	}
	var in custom.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	src, err := s.Settings.UpdateCustomSource(r.Context(), id, in)
	if err != nil {
		s.writeSettingsError(w, "update custom source", err)
		return
	}
	s.auditUser(r, auditCustomSourceChanged, describeCustomSource(src))
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, src)
}

// deleteCustomSource handles DELETE /api/custom-sources/{id}: 204 or 404.
// Widgets that show it then say that their source is not available.
func (s *Server) deleteCustomSource(w http.ResponseWriter, r *http.Request) {
	id, ok := customSourceID(w, r)
	if !ok {
		return
	}
	if err := s.Settings.DeleteCustomSource(r.Context(), id); err != nil {
		s.writeSettingsError(w, "delete custom source", err)
		return
	}
	s.auditUser(r, auditCustomSourceRemoved, "id "+strconv.FormatInt(id, 10))
	s.SettingsChanged()
	w.WriteHeader(http.StatusNoContent)
}

// postCustomSourceTest handles POST /api/custom-sources/{id}/tests: fetches
// the source once and returns {data, truncated}, the answer cut down for
// the editor's tree. 502 with what went wrong, such as the status the API
// answered with; the message names the host but never a secret.
func (s *Server) postCustomSourceTest(w http.ResponseWriter, r *http.Request) {
	id, ok := customSourceID(w, r)
	if !ok {
		return
	}
	src, err := s.Settings.CustomSourceWithSecrets(r.Context(), id)
	if err != nil {
		s.writeSettingsError(w, "read custom source", err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), customTestTimeout)
	defer cancel()
	raw, err := s.CustomFetcher.Fetch(ctx, src)
	if err != nil {
		s.Logger.Warn("custom source test failed", "source", src.Name, "error", err)
		message := "The API could not be fetched. Check the GDash log."
		var fetchErr *custom.FetchError
		if errors.As(err, &fetchErr) {
			message = "The API could not be fetched: " + fetchErr.Message + "."
		}
		writeError(w, http.StatusBadGateway, "fetch_failed", message)
		return
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		s.internalError(w, "decode custom source answer", err)
		return
	}
	data, cut := custom.Preview(doc)
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "truncated": cut, "bytes": len(raw)})
}

// postCustomRuleCheck handles POST /api/custom-rule-checks {when, data,
// value?, row?}: evaluates a rule's condition against data, such as a
// pasted answer, so the editor can show what it gives. 200 with {result}
// or {error} when it does not apply to the data; 422 on when for a
// condition that cannot be parsed.
func (s *Server) postCustomRuleCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		When  string `json:"when"`
		Data  any    `json:"data"`
		Value any    `json:"value"`
		Row   any    `json:"row"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	expr, err := custom.ParseExpr(body.When)
	if err != nil {
		writeValidationError(w, []validation.FieldError{{Field: "when", Message: err.Error()}})
		return
	}
	result, err := expr.Eval(custom.Env{Doc: body.Data, Vars: map[string]any{"value": body.Value, "row": body.Row}, Now: s.Now()})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func customSourceID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "This source does not exist.")
		return 0, false
	}
	return id, true
}

// describeCustomSource is the audit detail: the name and host, never the
// path or query, which could hold a key.
func describeCustomSource(src settings.CustomSource) string {
	host := ""
	if u, err := url.Parse(src.URL); err == nil {
		host = u.Host
	}
	return src.Name + " (" + host + ")"
}
