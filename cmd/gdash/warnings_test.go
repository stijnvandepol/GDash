package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/displays"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/warnings"
)

// warningData is a warnings entry with one orange warning for Zeeland from
// onset, as the hub holds it.
func warningData(t *testing.T, onset time.Time) map[string]datahub.Entry {
	t.Helper()
	value, err := json.Marshal(map[string]warnings.Area{"Zeeland": {Region: "Zeeland", Warnings: []warnings.Warning{{
		ID: "w1", Level: warnings.LevelOrange, Kind: "wind", Onset: onset, Expires: onset.Add(6 * time.Hour),
		Headline: map[string]string{"nl": "Code oranje: zware windstoten", "en": "Code orange: severe gusts"},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]datahub.Entry{display.WarningsSource: {Value: value}}
}

func TestSevereWarningsAreAnnouncedOnEveryScreen(t *testing.T) {
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	now := noon.Add(-time.Hour)
	dashboard := display.Dashboard{ID: "hall", Widgets: []display.Widget{warningsOn(map[string]any{"region": "Zeeland", "announce": true})}}
	var events []store.AuditEvent
	var expiries []time.Time
	board := announcement.NewBoard(func() time.Time { return now })
	rules := &warningRules{
		dashboards: func(context.Context) ([]display.Dashboard, error) { return []display.Dashboard{dashboard}, nil },
		displays: func(context.Context) ([]displays.Display, error) {
			return []displays.Display{{ID: "hall"}, {ID: "kitchen"}}, nil
		},
		data:      func() map[string]datahub.Entry { return warningData(t, noon) },
		language:  func(context.Context) (string, error) { return "nl", nil },
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
	if board.For("hall", now) != nil {
		t.Fatal("a warning that has not started was announced")
	}
	now = noon
	rules.check(ctx)
	for _, id := range []string{"hall", "kitchen"} {
		shown := board.For(id, now)
		if shown == nil || shown.Title != "Code oranje · Zeeland" || shown.Message != "Code oranje: zware windstoten" {
			t.Fatalf("%s shows %+v, want the warning in Dutch", id, shown)
		}
	}
	if len(expiries) != 1 || !expiries[0].Equal(now.Add(5*time.Minute)) {
		t.Fatalf("expiries = %v, want one five minutes from now", expiries)
	}
	if len(events) != 1 || events[0].Action != warningAuditAnnounced || events[0].Actor != warningActor {
		t.Fatalf("audit = %+v", events)
	}
	now = noon.Add(10 * time.Minute)
	rules.check(ctx)
	if len(events) != 1 {
		t.Fatal("the same warning was announced twice")
	}
}

func TestWarningsAreNotAnnouncedWithoutTheSetting(t *testing.T) {
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	now := noon.Add(-time.Hour)
	dashboard := display.Dashboard{ID: "hall", Widgets: []display.Widget{warningsOn(map[string]any{"region": "Zeeland"})}}
	posted := 0
	rules := &warningRules{
		dashboards: func(context.Context) ([]display.Dashboard, error) { return []display.Dashboard{dashboard}, nil },
		displays:   func(context.Context) ([]displays.Display, error) { return []displays.Display{{ID: "hall"}}, nil },
		data:       func() map[string]datahub.Entry { return warningData(t, noon) },
		language:   func(context.Context) (string, error) { return "en", nil },
		board:      announcement.NewBoard(func() time.Time { return now }),
		announced:  func(time.Time) { posted++ },
		audit:      func(context.Context, store.AuditEvent) error { return nil },
		now:        func() time.Time { return now },
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rules.check(context.Background())
	now = noon
	rules.check(context.Background())
	if posted != 0 {
		t.Fatalf("%d announcements, want none", posted)
	}
}
