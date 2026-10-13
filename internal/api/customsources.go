package api

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// CustomFetcher fetches a custom source once, for the Test button.
type CustomFetcher interface {
	Fetch(ctx context.Context, src custom.Source) (json.RawMessage, error)
}

const (
	// customTestTimeout bounds a Test; the fetcher's client has its own,
	// shorter timeout per request.
	customTestTimeout = 20 * time.Second
	// pushGap is the least time between two accepted answers of one pushed
	// source, a custom one or Homey, so a script stuck in a loop cannot keep
	// the server busy: every answer is decoded and published to every
	// screen.
	pushGap = 5 * time.Second
)

// handleCustomSources adds the routes of custom API sources, all for a
// logged-in admin only, except the one that a household's system sends a
// pushed source's answer to, which also takes an API token.
func (s *Server) handleCustomSources(mux *http.ServeMux) {
	mux.Handle("PUT /api/custom-sources/{id}/data", s.requireUserOrToken(s.putCustomSourceData))
	mux.Handle("GET /api/custom-sources", s.requireUser(s.listCustomSources))
	mux.Handle("POST /api/custom-sources", s.requireUser(s.postCustomSource))
	mux.Handle("PUT /api/custom-sources/{id}", s.requireUser(s.putCustomSource))
	mux.Handle("PUT /api/custom-sources/{id}/values", s.requireUser(s.putCustomSourceValues))
	mux.Handle("DELETE /api/custom-sources/{id}", s.requireUser(s.deleteCustomSource))
	mux.Handle("POST /api/custom-sources/{id}/tests", s.requireUser(s.postCustomSourceTest))
	mux.Handle("POST /api/custom-source-tests", s.requireUser(s.postCustomSourceTrial))
	mux.Handle("POST /api/custom-rule-checks", s.requireUser(s.postCustomRuleCheck))
}

// listedCustomSource is a source in the list, with when a pushed one last
// sent its answer.
type listedCustomSource struct {
	settings.CustomSource
	LastPushAt *time.Time `json:"lastPushAt,omitempty"`
}

// listCustomSources handles GET /api/custom-sources: {"sources": [...]},
// without secrets; each says whether its authentication secret is set, and
// a pushed one when its last answer arrived (lastPushAt, absent before the
// first).
func (s *Server) listCustomSources(w http.ResponseWriter, r *http.Request) {
	list, err := s.Settings.CustomSources(r.Context())
	if err != nil {
		s.internalError(w, "list custom sources", err)
		return
	}
	status := s.SourceStatus()
	out := make([]listedCustomSource, len(list))
	for i, src := range list {
		out[i].CustomSource = src
		if entry := status[custom.SourceKey(src.ID)]; src.Mode == custom.ModePush && !entry.FetchedAt.IsZero() {
			out[i].LastPushAt = &entry.FetchedAt
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

// putCustomSourceData handles PUT /api/custom-sources/{id}/data with an API
// token or an admin session. The body, JSON of at most 1 MB sent as
// application/json, becomes the answer of a pushed source, exactly as if
// GDash had fetched it. 204; 404 for an unknown source, 409 for one that
// GDash fetches itself, 413 over 1 MB, 415 for another content type, 422
// for a body that is not JSON, 429 within 5 seconds of its last accepted
// answer. Not audited: a script sends one every minute.
func (s *Server) putCustomSourceData(w http.ResponseWriter, r *http.Request) {
	id, ok := customSourceID(w, r)
	if !ok {
		return
	}
	src, err := s.Store.CustomSource(r.Context(), id)
	if err != nil {
		s.writeSettingsError(w, "read custom source", err)
		return
	}
	if src.Mode != custom.ModePush {
		writeError(w, http.StatusConflict, "not_pushed", "GDash fetches this source from its address. Edit it under Integrations, Custom APIs, and choose that your system sends the data.")
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Send the data as application/json.")
		return
	}
	key := "custom/" + strconv.FormatInt(id, 10)
	if !s.pushes.Attempt(key) {
		writeRateLimited(w)
		return
	}
	answer, err := custom.ReadAnswer(http.MaxBytesReader(w, r.Body, custom.MaxResponseBytes+1))
	if err != nil {
		// Only an accepted answer counts against the limit.
		s.pushes.Release(key)
		writeAnswerError(w, err)
		return
	}
	if err := s.ReceiveCustom(id, answer); err != nil {
		s.pushes.Release(key)
		if errors.Is(err, datahub.ErrNotPushed) {
			writeError(w, http.StatusConflict, "not_pushed", "GDash is still switching this source over. Try again in a moment.")
			return
		}
		s.internalError(w, "receive custom source data", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeRateLimited refuses data that arrived within pushGap of the last.
func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(int(pushGap.Seconds())))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "Data arrived less than 5 seconds ago. Send it at most about once a minute.")
}

// writeAnswerError refuses data that custom.ReadAnswer did not accept.
func writeAnswerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, custom.ErrAnswerTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "The data is larger than 1 MB. Send only the values the dashboards show.")
	case errors.Is(err, custom.ErrAnswerNotJSON):
		writeError(w, http.StatusUnprocessableEntity, "invalid_json", "The data is not valid JSON.")
	default:
		// The client stopped sending halfway; there is no one to tell.
		writeError(w, http.StatusBadRequest, "incomplete", "The data did not arrive completely.")
	}
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

// putCustomSourceValues handles PUT /api/custom-sources/{id}/values with
// {values, lists}: the named values the dashboard editor offers. 200 with
// the source, 404, or 422 naming each field, such as values[1].path. The
// fetching does not change, so the data hub is not told.
func (s *Server) putCustomSourceValues(w http.ResponseWriter, r *http.Request) {
	id, ok := customSourceID(w, r)
	if !ok {
		return
	}
	var in custom.Values
	if !decodeJSON(w, r, &in) {
		return
	}
	src, err := s.Settings.SetCustomSourceValues(r.Context(), id, in)
	if err != nil {
		s.writeSettingsError(w, "set named values of custom source", err)
		return
	}
	s.auditUser(r, auditCustomSourceChanged, describeCustomSource(src))
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
// answered with; the message names the host but never a secret. A pushed
// source gives the answer it last sent instead, see writeLastPushed.
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
	if src.Mode == custom.ModePush {
		s.writeLastPushed(w, id)
		return
	}
	s.writeFetched(w, r, src)
}

// postCustomSourceTrial handles POST /api/custom-source-tests with a source
// as a form has it, and the id of the stored source it changes or none:
// fetches it once without storing it, so a setup can be tried before it is
// saved. A secret left empty is the stored source's. 200 as a Test, 404 for
// an unknown id, 422 naming each invalid field, 502 when the fetch fails.
func (s *Server) postCustomSourceTrial(w http.ResponseWriter, r *http.Request) {
	var body struct {
		custom.Input
		ID int64 `json:"id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	src, err := s.Settings.TrialCustomSource(r.Context(), body.ID, body.Input)
	if err != nil {
		s.writeSettingsError(w, "try custom source", err)
		return
	}
	if src.Mode == custom.ModePush {
		writeValidationError(w, []validation.FieldError{{Field: "mode", Message: "GDash cannot ask a source that your system sends; save it and send data to it"}})
		return
	}
	s.writeFetched(w, r, src)
}

// writeFetched fetches src once and answers like a Test. 502 with what went
// wrong, such as the status the API answered with; the message names the
// host but never a secret.
func (s *Server) writeFetched(w http.ResponseWriter, r *http.Request, src custom.Source) {
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
	s.writeAnswer(w, raw, nil)
}

// writeLastPushed answers a Test of a pushed source with the answer it
// last sent, and when it arrived as receivedAt: GDash cannot ask for one.
// 409 when nothing has arrived yet.
func (s *Server) writeLastPushed(w http.ResponseWriter, id int64) {
	entry := s.SourceStatus()[custom.SourceKey(id)]
	if len(entry.Value) == 0 {
		writeError(w, http.StatusConflict, "nothing_received", "Nothing has been received yet. Send data to this source first; Integrations, Custom APIs shows how.")
		return
	}
	s.writeAnswer(w, entry.Value, &entry.FetchedAt)
}

// writeAnswer sends an answer cut down for the editor's tree.
func (s *Server) writeAnswer(w http.ResponseWriter, raw json.RawMessage, receivedAt *time.Time) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		s.internalError(w, "decode custom source answer", err)
		return
	}
	data, cut := custom.Preview(doc)
	body := map[string]any{"data": data, "truncated": cut, "bytes": len(raw)}
	if receivedAt != nil {
		body["receivedAt"] = receivedAt
	}
	writeJSON(w, http.StatusOK, body)
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
	if src.Mode == custom.ModePush {
		return src.Name + " (sent to GDash)"
	}
	host := ""
	if u, err := url.Parse(src.URL); err == nil {
		host = u.Host
	}
	return src.Name + " (" + host + ")"
}
