package display

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/datahub"
	"github.com/stijnvandepol/daycast/internal/schedule"
)

var now = time.Date(2026, 10, 8, 7, 30, 0, 0, time.UTC)

func decode(t *testing.T, msg []byte) Snapshot {
	t.Helper()
	var s Snapshot
	if err := json.Unmarshal(msg, &s); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return s
}

func kitchenOnly() map[string]View {
	return map[string]View{"kitchen": {Dashboard: Normalize(DefaultDashboard(false))}}
}

func TestSubscriberGetsCurrentSnapshotFirst(t *testing.T) {
	b := NewBroadcaster()
	if err := b.Publish(now, kitchenOnly(), nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := decode(t, <-b.Subscribe(ctx, "kitchen"))
	if first.Version != 1 || first.Type != "snapshot" || first.Dashboard.ID != DefaultID {
		t.Fatalf("first message = %+v", first)
	}
}

func TestSlowSubscriberOnlyGetsNewestSnapshot(t *testing.T) {
	b := NewBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx, "kitchen")

	for range 5 {
		if err := b.Publish(now, kitchenOnly(), nil); err != nil {
			t.Fatal(err)
		}
	}
	if got := decode(t, <-ch); got.Version != 5 {
		t.Fatalf("version = %d, want only the newest (5)", got.Version)
	}
	select {
	case msg := <-ch:
		t.Fatalf("unexpected backlog message: %s", msg)
	default:
	}
}

func TestEachDisplayGetsItsDashboardAndOnlyItsData(t *testing.T) {
	b := NewBroadcaster()
	withCalendar := Normalize(DefaultDashboard(true))
	withoutCalendar := Normalize(DefaultDashboard(false))
	data := map[string]datahub.Entry{
		"weather.current":  {Value: []byte(`14`)},
		"calendar.default": {Value: []byte(`["Dentist"]`)},
		"news.default":     {Value: []byte(`[]`)},
	}
	if err := b.Publish(now, map[string]View{"kitchen": {Dashboard: withCalendar}, "hall": {Dashboard: withoutCalendar}}, data); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kitchen := decode(t, <-b.Subscribe(ctx, "kitchen"))
	hall := decode(t, <-b.Subscribe(ctx, "hall"))
	if _, ok := kitchen.Data["calendar.default"]; !ok {
		t.Fatal("kitchen, which has a calendar widget, did not get calendar data")
	}
	if _, ok := hall.Data["calendar.default"]; ok {
		t.Fatal("hall has no calendar widget but received calendar data")
	}
	if _, ok := hall.Data["weather.current"]; !ok {
		t.Fatal("hall did not get the weather it shows")
	}

	unknown := b.Subscribe(ctx, "garage")
	select {
	case msg := <-unknown:
		t.Fatalf("a display without a dashboard got %s", msg)
	default:
	}
}

func TestDisplayOutsideActiveHoursGetsNoData(t *testing.T) {
	b := NewBroadcaster()
	// now is 07:30 UTC; the window opens at 08:00 in the same zone.
	night := schedule.Hours{From: "08:00", Until: "23:00"}
	views := map[string]View{"kitchen": {Dashboard: Normalize(DefaultDashboard(false)), Hours: night}}
	data := map[string]datahub.Entry{"weather.current": {Value: []byte(`14`)}}
	if err := b.Publish(now, views, data); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := decode(t, <-b.Subscribe(ctx, "kitchen"))
	if got.Active || len(got.Data) != 0 {
		t.Fatalf("snapshot outside active hours = active %v, data %v", got.Active, got.Data)
	}
	if err := b.Publish(now.Add(time.Hour), views, data); err != nil {
		t.Fatal(err)
	}
	got = decode(t, <-b.Subscribe(ctx, "kitchen"))
	if !got.Active || len(got.Data) != 1 {
		t.Fatalf("snapshot inside active hours = active %v, data %v", got.Active, got.Data)
	}
}

func TestSubscriptionClosesWithContext(t *testing.T) {
	b := NewBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx, "kitchen")
	cancel()

	select {
	case _, open := <-ch:
		if open {
			t.Fatal("received a message after cancel instead of a close")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after context cancel")
	}
}

func TestDefaultDashboardsFitGridWithoutOverlap(t *testing.T) {
	for _, withCalendar := range []bool{false, true} {
		d := DefaultDashboard(withCalendar)
		taken := map[[2]int]string{}
		for _, w := range d.Widgets {
			if w.X < 0 || w.Y < 0 || w.W < 1 || w.H < 1 || w.X+w.W > d.Columns || w.Y+w.H > d.Rows {
				t.Errorf("calendar=%v: widget %s at %d,%d size %dx%d does not fit a %dx%d grid", withCalendar, w.ID, w.X, w.Y, w.W, w.H, d.Columns, d.Rows)
			}
			if cell, other := claimCells(taken, w); other != "" {
				t.Fatalf("calendar=%v: widgets %s and %s overlap at %v", withCalendar, other, w.ID, cell)
			}
		}
	}
}

// claimCells marks the widget's cells as taken and returns the first cell
// that another widget already held, with that widget's id.
func claimCells(taken map[[2]int]string, w Widget) ([2]int, string) {
	for x := w.X; x < w.X+w.W; x++ {
		for y := w.Y; y < w.Y+w.H; y++ {
			if other, ok := taken[[2]int{x, y}]; ok {
				return [2]int{x, y}, other
			}
			taken[[2]int{x, y}] = w.ID
		}
	}
	return [2]int{}, ""
}
