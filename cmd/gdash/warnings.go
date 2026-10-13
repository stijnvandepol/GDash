package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/warnings"
)

const (
	// The paths on GDASH_TEST_UPSTREAM that stand in for the real
	// services; web/e2e/upstream.ts serves them.
	testBuienradarPath = "/buienradar/data/raintext"
	testOpenMeteoPath  = "/open-meteo/v1/forecast"
	testMeteoAlarmPath = "/meteoalarm/api/v1/warnings/feeds-netherlands"
	testSolarPath      = "/open-meteo-solar/v1/forecast"

	// warningSeconds is how long a code orange or red warning stays over
	// the dashboards: long enough to be seen when walking past.
	warningSeconds = 300
	// warningActor is who the audit log names for such an announcement.
	warningActor = "weather warning"
	// warningAuditAnnounced is the audit action; web/src/admin/audit.ts
	// describes it.
	warningAuditAnnounced = "weather_warning_announced"
)

// warningRules puts code orange and red warnings over every screen when
// they come into effect, for Weather warnings widgets that announce. It
// runs after every data change and every minute, since a warning that was
// issued in advance comes into effect without new data.
type warningRules struct {
	announcer  warnings.Announcer
	dashboards func(ctx context.Context) ([]display.Dashboard, error)
	displays   func(ctx context.Context) ([]displays.Display, error)
	data       func() map[string]datahub.Entry
	language   func(ctx context.Context) (string, error)
	board      *announcement.Board
	// announced is called with the expiry of a posted announcement, so
	// screens show it and drop it on time.
	announced func(expiresAt time.Time)
	audit     func(ctx context.Context, e store.AuditEvent) error
	now       func() time.Time
	logger    *slog.Logger
}

func (r *warningRules) check(ctx context.Context) {
	dashboards, err := r.dashboards(ctx)
	if err != nil {
		r.logger.Error("could not read dashboards for the weather warnings", "error", err)
		return
	}
	keys := display.AnnouncedWarningKeys(dashboards)
	areas, ok := r.areas(keys)
	if !ok {
		// Not fetched yet: nothing is known, so nothing is learned either.
		return
	}
	now := r.now()
	notices := r.announcer.Check(areas, now)
	if len(notices) == 0 {
		return
	}
	list, err := r.displays(ctx)
	if err != nil {
		r.logger.Error("could not list displays for a weather warning", "error", err)
		return
	}
	language, err := r.language(ctx)
	if err != nil {
		r.logger.Error("could not read the display language for a weather warning", "error", err)
		return
	}
	known := make([]string, len(list))
	for i, d := range list {
		known[i] = d.ID
	}
	for _, n := range notices {
		r.post(ctx, n, language, known, now)
	}
}

// areas reads the regions under keys from the warnings entry. ok is false
// while the entry has no value.
func (r *warningRules) areas(keys []string) ([]warnings.Area, bool) {
	if len(keys) == 0 {
		return nil, true
	}
	entry, found := r.data()[display.WarningsSource]
	if !found || len(entry.Value) == 0 {
		return nil, false
	}
	var all map[string]warnings.Area
	if err := json.Unmarshal(entry.Value, &all); err != nil {
		r.logger.Error("could not read the weather warnings for announcing", "error", err)
		return nil, false
	}
	var areas []warnings.Area
	for _, key := range keys {
		if area, ok := all[key]; ok {
			areas = append(areas, area)
		}
	}
	return areas, true
}

func (r *warningRules) post(ctx context.Context, n warnings.Notice, language string, known []string, now time.Time) {
	title, message := n.Text(language)
	seconds := warningSeconds
	posted, err := r.board.Post(announcement.Request{Title: title, Message: message, Seconds: &seconds}, known, now)
	if err != nil {
		// No displays yet, or a text the board refuses.
		r.logger.Warn("a weather warning was not announced", "region", n.Region, "warning", n.Warning.ID, "error", err)
		return
	}
	detail := title + ": " + message
	if runes := []rune(detail); len(runes) > maxAuditDetail {
		detail = string(runes[:maxAuditDetail])
	}
	if err := r.audit(ctx, store.AuditEvent{At: now.UTC(), Actor: warningActor, Action: warningAuditAnnounced, Detail: detail}); err != nil {
		r.logger.Error("could not write the audit log", "action", warningAuditAnnounced, "error", err)
	}
	r.logger.Info("a weather warning was announced", "region", n.Region, "level", n.Warning.Level, "displays", len(posted.Displays))
	r.announced(posted.ExpiresAt)
}
