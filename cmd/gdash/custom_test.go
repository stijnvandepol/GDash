package main

import (
	"context"
	"encoding/json"
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

func TestPushedCustomSourcesAreAlwaysInTheHub(t *testing.T) {
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := &dataSources{settings: &settings.Service{Box: box}}
	all := []custom.Source{{ID: 4, Name: "Homey", Mode: custom.ModePush, Interval: time.Minute}}
	got := d.customSources(all, nil)
	if len(got) != 1 || got[0].Key != custom.SourceKey(4) || got[0].Fetch != nil || got[0].FreshFor != 150*time.Second {
		t.Fatalf("customSources = %+v, want source 4 as pushed, fresh for 2m30s", got)
	}
	before := got[0].Version
	all[0].Name, all[0].Interval = "Homey Pro", 5*time.Minute
	if after := d.customSources(all, nil)[0].Version; after != before {
		t.Fatal("a new name or interval would drop the last pushed answer")
	}
	all[0].Mode, all[0].URL = custom.ModeFetch, "https://x"
	if after := d.customSources(all, []int64{4})[0].Version; after == before {
		t.Fatal("switching to fetching keeps the pushed answer")
	}
}

func TestPushedAnswerRunsTheRules(t *testing.T) {
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cache, err := datahub.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dashboard := waterDashboard()
	board := announcement.NewBoard(func() time.Time { return now })
	var hub *datahub.Hub
	rules := &customRules{
		announcer:  custom.Announcer{Gap: custom.AnnounceGap},
		dashboards: func(context.Context) ([]display.Dashboard, error) { return []display.Dashboard{dashboard}, nil },
		displays:   func(context.Context) ([]displays.Display, error) { return []displays.Display{{ID: "hall"}}, nil },
		data:       func() map[string]datahub.Entry { return hub.Entries() },
		board:      board,
		announced:  func(time.Time) {},
		audit:      func(context.Context, store.AuditEvent) error { return nil },
		now:        func() time.Time { return now },
		logger:     logger,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub = datahub.New(cache, logger, func() { rules.check(ctx) })
	d := &dataSources{hub: hub, settings: &settings.Service{Box: box}}
	hub.Configure(ctx, d.customSources([]custom.Source{{ID: 1, Mode: custom.ModePush, Interval: time.Minute}}, nil))

	for _, answer := range []string{`{"level": 5}`, `{"level": 12}`} {
		if err := d.receiveCustom(1, json.RawMessage(answer)); err != nil {
			t.Fatal(err)
		}
	}
	if shown := board.For("hall", now); shown == nil || shown.Message != "The water is high" {
		t.Fatalf("announcement after a pushed answer = %+v", shown)
	}
	if err := d.receiveCustom(2, json.RawMessage(`1`)); err == nil {
		t.Fatal("an answer for a source that is not pushed was stored")
	}
}

// waterDashboard shows source 1 with a rule that announces high water.
func waterDashboard() display.Dashboard {
	return display.Dashboard{ID: "hall", Widgets: []display.Widget{{
		ID: "water", Type: display.CustomWidget, W: 4, H: 3, Settings: map[string]any{
			"source": 1.0, "mode": "value", "path": "level", "title": "Water",
			"rules": []any{map[string]any{"when": "value > 10", "message": "The water is high", "announce": true}},
		},
	}}}
}

func TestCustomRulesAnnounceAndAudit(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	dashboard := waterDashboard()
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
