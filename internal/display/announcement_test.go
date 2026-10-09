package display

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/schedule"
)

func TestSnapshotCarriesTheAnnouncement(t *testing.T) {
	b := NewBroadcaster()
	shown := &announcement.Announcement{Title: "Laundry", Message: "Done", ExpiresAt: now.Add(time.Minute)}
	views := map[string]View{
		"kitchen": {Dashboard: Normalize(DefaultDashboard(false)), Announcement: shown},
		"hall":    {Dashboard: Normalize(DefaultDashboard(false))},
	}
	if err := b.Publish(now, views, nil, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kitchen := decode(t, <-b.Subscribe(ctx, "kitchen"))
	if kitchen.Announcement == nil || *kitchen.Announcement != *shown {
		t.Fatalf("kitchen announcement = %+v, want %+v", kitchen.Announcement, shown)
	}
	if hall := decode(t, <-b.Subscribe(ctx, "hall")); hall.Announcement != nil {
		t.Fatalf("hall announcement = %+v, want none", hall.Announcement)
	}
}

func TestDisplayOutsideActiveHoursGetsNoAnnouncement(t *testing.T) {
	b := NewBroadcaster()
	shown := &announcement.Announcement{Message: "Done", ExpiresAt: now.Add(time.Hour)}
	// now is 07:30 UTC; the window opens at 08:00 in the same zone.
	views := map[string]View{"kitchen": {
		Dashboard: Normalize(DefaultDashboard(false)), Hours: schedule.Hours{From: "08:00", Until: "23:00"}, Announcement: shown,
	}}
	if err := b.Publish(now, views, nil, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := decode(t, <-b.Subscribe(ctx, "kitchen")); got.Active || got.Announcement != nil {
		t.Fatalf("snapshot outside active hours = active %v, announcement %+v", got.Active, got.Announcement)
	}
}

func TestPublisherAddsAndDropsAnnouncements(t *testing.T) {
	clock := now
	board := announcement.NewBoard(func() time.Time { return clock })
	b := NewBroadcaster()
	p := &Publisher{
		Broadcaster: b,
		Views: func(context.Context) (map[string]View, error) {
			return map[string]View{
				"kitchen": {Dashboard: Normalize(DefaultDashboard(false))},
				"hall":    {Dashboard: Normalize(DefaultDashboard(false))},
			}, nil
		},
		Data:          func() map[string]datahub.Entry { return nil },
		Language:      func(context.Context) (string, error) { return "en", nil },
		Announcements: board,
		Now:           func() time.Time { return clock },
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	latest := func(id string) Snapshot { return decode(t, <-b.Subscribe(ctx, id)) }

	if _, err := board.Post(announcement.Request{Message: "Done", Displays: []string{"kitchen"}}, []string{"kitchen", "hall"}, clock); err != nil {
		t.Fatal(err)
	}
	p.Refresh(ctx)
	if got := latest("kitchen"); got.Announcement == nil || got.Announcement.Message != "Done" {
		t.Fatalf("kitchen announcement = %+v", got.Announcement)
	}
	if got := latest("hall"); got.Announcement != nil {
		t.Fatalf("hall announcement = %+v, want none", got.Announcement)
	}

	clock = clock.Add(announcement.DefaultSeconds * time.Second)
	p.Refresh(ctx)
	if got := latest("kitchen"); got.Announcement != nil {
		t.Fatalf("expired announcement still in the snapshot: %+v", got.Announcement)
	}
}
