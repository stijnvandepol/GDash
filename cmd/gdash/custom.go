package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/store"
)

const (
	// customTimeout bounds one refresh of a custom source; the outbound
	// client's own timeout bounds each request below it.
	customTimeout = 30 * time.Second
	// customRuleActor is who the audit log names for a rule's announcement.
	customRuleActor = "rule"
	// customAuditAnnounced is the audit action; web/src/admin/audit.ts
	// describes it.
	customAuditAnnounced = "custom_rule_announced"
	// maxAuditDetail matches internal/api's limit on audit details.
	maxAuditDetail = 200
)

// customSources returns a hub source for each custom source that a
// dashboard shows; the others are never fetched. Every pushed source is in
// the hub, shown or not: it costs no requests, and its last answer is
// there to name values in before a dashboard shows it.
func (d *dataSources) customSources(all []custom.Source, shown []int64) []datahub.Source {
	var sources []datahub.Source
	for _, src := range all {
		if src.Mode == custom.ModePush {
			sources = append(sources, datahub.Source{
				Key:      custom.SourceKey(src.ID),
				Interval: src.Interval,
				FreshFor: custom.PushFreshFor(src.Interval),
				// Renaming it or changing its interval keeps the last
				// answer; switching from fetching starts over.
				Version: d.fingerprint(struct{ Mode string }{custom.ModePush}),
			})
			continue
		}
		if !slices.Contains(shown, src.ID) {
			continue
		}
		sources = append(sources, datahub.Source{
			Key:      custom.SourceKey(src.ID),
			Interval: src.Interval,
			Timeout:  customTimeout,
			// The fingerprint covers the secrets, so a new token fetches
			// at once; it is keyed, so the cache file does not reveal it.
			Version: d.fingerprint(src),
			Fetch:   func(ctx context.Context) (any, error) { return d.custom.Fetch(ctx, src) },
		})
	}
	return sources
}

// receiveCustom stores an answer that a pushed source sent, as its latest;
// the hub then refreshes the screens and the rules.
func (d *dataSources) receiveCustom(sourceID int64, answer json.RawMessage) error {
	return d.hub.Put(custom.SourceKey(sourceID), answer)
}

// customRules posts the announcements of Custom widget rules that start to
// hold. It runs after every data change and every minute, since a rule on
// minutesSince can start to hold without new data.
type customRules struct {
	announcer  custom.Announcer
	dashboards func(ctx context.Context) ([]display.Dashboard, error)
	displays   func(ctx context.Context) ([]displays.Display, error)
	data       func() map[string]datahub.Entry
	board      *announcement.Board
	// announced is called with the expiry of a posted announcement, so
	// screens show it and drop it on time.
	announced func(expiresAt time.Time)
	audit     func(ctx context.Context, e store.AuditEvent) error
	now       func() time.Time
	logger    *slog.Logger
}

func (r *customRules) check(ctx context.Context) {
	dashboards, err := r.dashboards(ctx)
	if err != nil {
		r.logger.Error("could not read dashboards for the Custom widget rules", "error", err)
		return
	}
	now := r.now()
	fired := r.announcer.Check(display.CustomTargets(dashboards, r.data(), now), now)
	if len(fired) == 0 {
		return
	}
	list, err := r.displays(ctx)
	if err != nil {
		r.logger.Error("could not list displays for a rule's announcement", "error", err)
		return
	}
	known := make([]string, len(list))
	for i, d := range list {
		known[i] = d.ID
	}
	for _, a := range fired {
		r.post(ctx, a, known, now)
	}
}

func (r *customRules) post(ctx context.Context, a custom.Announcement, known []string, now time.Time) {
	posted, err := r.board.Post(announcement.Request{Title: a.Title, Message: a.Message}, known, now)
	if err != nil {
		// No displays yet, or a title or message the board refuses.
		r.logger.Warn("a rule's announcement was not posted", "widget", a.Key, "error", err)
		return
	}
	detail := a.Key + ": " + a.Message
	if runes := []rune(detail); len(runes) > maxAuditDetail {
		detail = string(runes[:maxAuditDetail])
	}
	if err := r.audit(ctx, store.AuditEvent{At: now.UTC(), Actor: customRuleActor, Action: customAuditAnnounced, Detail: detail}); err != nil {
		r.logger.Error("could not write the audit log", "action", customAuditAnnounced, "error", err)
	}
	r.logger.Info("a rule posted an announcement", "widget", a.Key, "displays", len(posted.Displays))
	r.announced(posted.ExpiresAt)
}
