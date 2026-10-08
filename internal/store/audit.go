package store

import (
	"context"
	"fmt"
	"time"
)

const (
	auditRetention = 90 * 24 * time.Hour
	maxAuditEvents = 5000
	// maxAnonymousEvents caps events without a logged-in user, such as
	// failed logins, so requests from anyone on the network can never push
	// the events of admins out of the log.
	maxAnonymousEvents = 1000
	// MaxAuditPage bounds one read of the audit log.
	MaxAuditPage = 200
)

// AuditEvent is one entry of the audit log.
type AuditEvent struct {
	At time.Time `json:"at"`
	// Actor is the username, or empty for an anonymous request such as a
	// failed login.
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
	IP     string `json:"ip"`
}

// AddAuditEvent records e and drops events older than 90 days, anonymous
// events beyond the newest 1000, and any beyond the newest 5000, so the log
// stays bounded without a separate job.
func (s *Store) AddAuditEvent(ctx context.Context, e AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO audit_log (at, actor, action, detail, ip) VALUES (?, ?, ?, ?, ?)",
		formatTime(e.At), e.Actor, e.Action, e.Detail, e.IP); err != nil {
		return fmt.Errorf("add audit event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM audit_log WHERE at < ?", formatTime(e.At.Add(-auditRetention))); err != nil {
		return fmt.Errorf("expire audit events: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM audit_log WHERE actor = '' AND id <= (SELECT id FROM audit_log WHERE actor = '' ORDER BY id DESC LIMIT 1 OFFSET ?)",
		maxAnonymousEvents); err != nil {
		return fmt.Errorf("trim anonymous audit events: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM audit_log WHERE id <= (SELECT id FROM audit_log ORDER BY id DESC LIMIT 1 OFFSET ?)", maxAuditEvents); err != nil {
		return fmt.Errorf("trim audit log: %w", err)
	}
	return tx.Commit()
}

// AuditEvents returns the newest events first, at most limit and at most
// MaxAuditPage.
func (s *Store) AuditEvents(ctx context.Context, limit int) ([]AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT at, actor, action, detail, ip FROM audit_log ORDER BY id DESC LIMIT ?", min(limit, MaxAuditPage))
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		var at string
		if err := rows.Scan(&at, &e.Actor, &e.Action, &e.Detail, &e.IP); err != nil {
			return nil, err
		}
		if e.At, err = parseTime(at); err != nil {
			return nil, fmt.Errorf("audit event has invalid time: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
