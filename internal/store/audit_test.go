package store

import (
	"context"
	"testing"
	"time"
)

func TestAuditLog(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	old := AuditEvent{At: created.Add(-100 * 24 * time.Hour), Action: "login", IP: "192.168.1.20"}
	if err := s.AddAuditEvent(ctx, old); err != nil {
		t.Fatal(err)
	}
	for i, action := range []string{"login", "display_updated"} {
		e := AuditEvent{At: created.Add(time.Duration(i) * time.Minute), Actor: "stijn", Action: action, Detail: "keuken", IP: "192.168.1.20"}
		if err := s.AddAuditEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	events, err := s.AuditEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v; want the two recent ones, the 100-day-old one expired", events)
	}
	if events[0].Action != "display_updated" || events[0].Actor != "stijn" || !events[0].At.Equal(created.Add(time.Minute)) {
		t.Fatalf("newest event = %+v", events[0])
	}
	if events, _ := s.AuditEvents(ctx, 1); len(events) != 1 {
		t.Fatalf("limit 1 returned %d events", len(events))
	}
}

func TestAuditLogKeepsAtMostMaxEvents(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	for i := range maxAuditEvents + 3 {
		if err := s.AddAuditEvent(ctx, AuditEvent{At: created.Add(time.Duration(i) * time.Second), Actor: "stijn", Action: "login"}); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM audit_log").Scan(&n); err != nil || n != maxAuditEvents {
		t.Fatalf("events kept = %d, %v; want %d", n, err, maxAuditEvents)
	}
}

func TestAnonymousEventsCannotPushOutAdminEvents(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	admin := AuditEvent{At: created, Actor: "stijn", Action: "display_updated", Detail: "keuken"}
	if err := s.AddAuditEvent(ctx, admin); err != nil {
		t.Fatal(err)
	}
	for i := range maxAuditEvents + 10 {
		e := AuditEvent{At: created.Add(time.Duration(i+1) * time.Second), Action: "login_failed"}
		if err := s.AddAuditEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	var admins, anonymous int
	s.db.QueryRowContext(ctx, "SELECT count(*) FROM audit_log WHERE actor = 'stijn'").Scan(&admins)
	s.db.QueryRowContext(ctx, "SELECT count(*) FROM audit_log WHERE actor = ''").Scan(&anonymous)
	if admins != 1 || anonymous != maxAnonymousEvents {
		t.Fatalf("admin events = %d, anonymous = %d; want the admin event kept and %d anonymous", admins, anonymous, maxAnonymousEvents)
	}
}
