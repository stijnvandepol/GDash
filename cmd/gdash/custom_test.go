package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
)

func TestCustomSourcesAreFetchedOnlyWhenShown(t *testing.T) {
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{settings: &settings.Service{Box: box}}
	all := []custom.Source{
		{ID: 1, Interval: time.Minute, AuthSecret: "a"},
		{ID: 2, Interval: time.Hour},
	}
	got := d.customSources(all, []int64{2, 3})
	if len(got) != 1 || got[0].Key != custom.SourceKey(2) || got[0].Interval != time.Hour {
		t.Fatalf("customSources = %+v, want only source 2", got)
	}
	before := d.customSources(all, []int64{1})[0].Version
	all[0].AuthSecret = "b"
	if after := d.customSources(all, []int64{1})[0].Version; after == before {
		t.Fatal("a new secret did not restart the source")
	}
}

func TestCustomRulesAnnounceAndAudit(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	dashboard := display.Dashboard{ID: "hall", Widgets: []display.Widget{{
		ID: "water", Type: display.CustomWidget, W: 4, H: 3, Settings: map[string]any{
			"source": 1.0, "mode": "value", "path": "level", "title": "Water",
			"rules": []any{map[string]any{"when": "value > 10", "message": "The water is high", "announce": true}},
		},
	}}}
	level := `{"level": 20}`
	var events []store.AuditEvent
	var expiries []time.Time
	board := announcement.NewBoard(func() time.Time { return now })
	rules := &customRules{
		announcer:  custom.Announcer{Gap: custom.AnnounceGap},
		dashboards: func(context.Context) ([]display.Dashboard, error) { return []display.Dashboard{dashboard}, nil },
		displays:   func(context.Context) ([]displays.Display, error) { return []displays.Display{{ID: "hall"}}, nil },
		data: func() map[string]datahub.Entry {
			return map[string]datahub.Entry{custom.SourceKey(1): {Value: []byte(level), FetchedAt: now}}
		},
		board:     board,
		announced: func(expiresAt time.Time) { expiries = append(expiries, expiresAt) },
		audit: func(_ context.Context, e store.AuditEvent) error {
			events = append(events, e)
			return nil
		},
		now:    func() time.Time { return now },
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx := context.Background()
	rules.check(ctx)
	if len(events) != 0 || board.For("hall", now) != nil {
		t.Fatal("the first look announced; it only records the state")
	}
	level = `{"level": 5}`
	now = now.Add(time.Minute)
	rules.check(ctx)
	level = `{"level": 12}`
	now = now.Add(time.Minute)
	rules.check(ctx)
	shown := board.For("hall", now)
	if shown == nil || shown.Message != "The water is high" || shown.Title != "Water" || len(expiries) != 1 {
		t.Fatalf("announcement = %+v, expiries %v", shown, expiries)
	}
	if len(events) != 1 || events[0].Actor != customRuleActor || events[0].Action != customAuditAnnounced ||
		events[0].Detail != "hall/water: The water is high" {
		t.Fatalf("audit = %+v", events)
	}
}
