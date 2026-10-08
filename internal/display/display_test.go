package display

import (
	"context"
	"encoding/json"
	"testing"
	"time"
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

func TestSubscriberGetsCurrentSnapshotFirst(t *testing.T) {
	b := NewBroadcaster()
	if err := b.Publish(now, DefaultDashboard(), nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := decode(t, <-b.Subscribe(ctx))
	if first.Version != 1 || first.Type != "snapshot" || first.Dashboard.ID != "default" {
		t.Fatalf("first message = %+v", first)
	}
}

func TestSlowSubscriberOnlyGetsNewestSnapshot(t *testing.T) {
	b := NewBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx)

	for range 5 {
		if err := b.Publish(now, DefaultDashboard(), nil); err != nil {
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

func TestSubscriptionClosesWithContext(t *testing.T) {
	b := NewBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)
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

func TestDefaultDashboardFitsGrid(t *testing.T) {
	d := DefaultDashboard()
	for _, w := range d.Widgets {
		if w.X < 0 || w.Y < 0 || w.W < 1 || w.H < 1 || w.X+w.W > d.Columns || w.Y+w.H > d.Rows {
			t.Errorf("widget %s at %d,%d size %dx%d does not fit a %dx%d grid", w.ID, w.X, w.Y, w.W, w.H, d.Columns, d.Rows)
		}
	}
}
