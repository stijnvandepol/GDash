package store

import (
	"context"
	"errors"
	"testing"

	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/displays"
	"github.com/stijnvandepol/daycast/internal/schedule"
)

func morning() display.Dashboard {
	return display.Dashboard{ID: "morning", Name: "Morning", Theme: "minimal-light", Widgets: []display.Widget{
		{ID: "clock", Type: "clock", X: 0, Y: 0, W: 8, H: 4},
		{ID: "news", Type: "news", X: 8, Y: 0, W: 10, H: 6, Settings: map[string]any{"maxItems": float64(4)}},
	}}
}

func TestDashboardLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDashboard(ctx, morning(), created); err != nil {
		t.Fatalf("CreateDashboard: %v", err)
	}
	got, err := s.Dashboard(ctx, "morning")
	if err != nil {
		t.Fatal(err)
	}
	// Reading normalizes: grid, theme and sources come from the server.
	if got.Columns != display.GridColumns || got.Theme != "minimal-light" || got.Widgets[1].Source != "news.default" || got.Widgets[1].Settings["maxItems"] != float64(4) {
		t.Fatalf("Dashboard = %+v", got)
	}

	changed := morning()
	changed.Name = "Mornings"
	changed.Widgets = changed.Widgets[:1]
	if err := s.UpdateDashboard(ctx, changed, created); err != nil {
		t.Fatal(err)
	}
	list, err := s.Dashboards(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Mornings" || len(list[0].Widgets) != 1 {
		t.Fatalf("Dashboards = %+v, %v", list, err)
	}
	if err := s.CreateDashboard(ctx, morning(), created); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate create: err = %v, want ErrConflict", err)
	}
	if err := s.DeleteDashboard(ctx, "morning"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Dashboard(ctx, "morning"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDefaultDashboardCannotBeDeletedAndEnsureKeepsEdits(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	def := display.DefaultDashboard(false)
	if err := s.EnsureDashboard(ctx, def, created); err != nil {
		t.Fatal(err)
	}
	edited := def
	edited.Name = "Kitchen layout"
	if err := s.UpdateDashboard(ctx, edited, created); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureDashboard(ctx, def, created); err != nil {
		t.Fatalf("second EnsureDashboard: %v", err)
	}
	if got, _ := s.Dashboard(ctx, display.DefaultID); got.Name != "Kitchen layout" {
		t.Fatalf("EnsureDashboard overwrote an edit: name = %q", got.Name)
	}
	if err := s.DeleteDashboard(ctx, display.DefaultID); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete default: err = %v, want ErrConflict", err)
	}
}

func TestDisplayDashboards(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.EnsureDashboard(ctx, display.DefaultDashboard(false), created); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDashboard(ctx, morning(), created); err != nil {
		t.Fatal(err)
	}
	kitchen := displays.Display{ID: "kitchen", Name: "Kitchen", DashboardID: "morning", CreatedAt: created,
		Hours: schedule.Hours{From: "06:30", Until: "23:00"}}
	hall := displays.Display{ID: "hall", Name: "Hall", CreatedAt: created}
	for _, d := range []displays.Display{kitchen, hall} {
		if err := s.CreateDisplay(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.DisplayViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got["kitchen"].Hours.From != "06:30" || !got["hall"].Hours.Always() {
		t.Fatalf("hours = kitchen:%+v hall:%+v", got["kitchen"].Hours, got["hall"].Hours)
	}
	if got["kitchen"].Dashboard.ID != "morning" || got["hall"].Dashboard.ID != display.DefaultID {
		t.Fatalf("DisplayViews = kitchen:%s hall:%s", got["kitchen"].Dashboard.ID, got["hall"].Dashboard.ID)
	}

	// Deleting a dashboard sends its displays back to the default.
	if err := s.DeleteDashboard(ctx, "morning"); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.Display(ctx, "kitchen"); d.DashboardID != "" {
		t.Fatalf("display still points at a deleted dashboard: %q", d.DashboardID)
	}
	if got, _ := s.DisplayViews(ctx); got["kitchen"].Dashboard.ID != display.DefaultID {
		t.Fatalf("kitchen shows %q after its dashboard was deleted", got["kitchen"].Dashboard.ID)
	}
}

func TestDisplaySchedule(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.EnsureDashboard(ctx, display.DefaultDashboard(false), created); err != nil {
		t.Fatal(err)
	}
	evening := morning()
	evening.ID, evening.Name = "evening", "Evening"
	for _, b := range []display.Dashboard{morning(), evening} {
		if err := s.CreateDashboard(ctx, b, created); err != nil {
			t.Fatal(err)
		}
	}
	d := displays.Display{ID: "kitchen", Name: "Kitchen", CreatedAt: created, Schedule: []displays.Slot{
		{From: "18:00", Until: "23:00", DashboardID: "evening"},
		{From: "06:30", Until: "09:00", DashboardID: "morning"},
	}}
	if err := s.CreateDisplay(ctx, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Display(ctx, "kitchen")
	if err != nil || len(got.Schedule) != 2 || got.Schedule[0] != d.Schedule[0] || got.Schedule[1] != d.Schedule[1] {
		t.Fatalf("Display = %+v, %v; want the schedule in its stored order", got.Schedule, err)
	}
	views, err := s.DisplayViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v := views["kitchen"]
	if v.Dashboard.ID != display.DefaultID || len(v.Schedule) != 2 || v.Schedule[1].Dashboard.ID != "morning" || v.Schedule[1].Hours.From != "06:30" {
		t.Fatalf("view = %+v", v)
	}

	// A slot with an unknown dashboard is refused and changes nothing.
	d.Schedule = []displays.Slot{{From: "06:30", Until: "09:00", DashboardID: "missing"}}
	if err := s.UpdateDisplay(ctx, d); !errors.Is(err, ErrUnknownScheduleDashboard) {
		t.Fatalf("update: err = %v, want ErrUnknownScheduleDashboard", err)
	}
	if got, _ := s.Display(ctx, "kitchen"); len(got.Schedule) != 2 {
		t.Fatalf("failed update changed the schedule to %+v", got.Schedule)
	}

	// Deleting a dashboard removes its slot; the others stay.
	if err := s.DeleteDashboard(ctx, "evening"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Display(ctx, "kitchen"); len(got.Schedule) != 1 || got.Schedule[0].DashboardID != "morning" {
		t.Fatalf("after deleting evening: schedule = %+v", got.Schedule)
	}

	// An update with an empty schedule clears it; deleting the display
	// removes its slots with it.
	d.Schedule = nil
	if err := s.UpdateDisplay(ctx, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Display(ctx, "kitchen"); got.Schedule == nil || len(got.Schedule) != 0 {
		t.Fatalf("cleared schedule = %#v, want an empty list", got.Schedule)
	}
	d.Schedule = []displays.Slot{{From: "06:30", Until: "09:00", DashboardID: "morning"}}
	if err := s.UpdateDisplay(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDisplay(ctx, "kitchen"); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM display_schedule").Scan(&left); err != nil || left != 0 {
		t.Fatalf("slots left after deleting the display = %d, %v", left, err)
	}
}

func TestDisplayWithUnknownDashboard(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	d := displays.Display{ID: "kitchen", Name: "Kitchen", DashboardID: "missing", CreatedAt: created}
	if err := s.CreateDisplay(ctx, d); !errors.Is(err, ErrUnknownDashboard) {
		t.Fatalf("create: err = %v, want ErrUnknownDashboard", err)
	}
	d.DashboardID = ""
	if err := s.CreateDisplay(ctx, d); err != nil {
		t.Fatal(err)
	}
	d.DashboardID = "missing"
	if err := s.UpdateDisplay(ctx, d); !errors.Is(err, ErrUnknownDashboard) {
		t.Fatalf("update: err = %v, want ErrUnknownDashboard", err)
	}
}
