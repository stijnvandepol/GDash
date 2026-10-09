package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/stijnvandepol/nookboard/internal/displays"
)

// splitDays reads the stored day list; empty means every day and is
// returned as an empty list, never nil, so the API always sends a list.
func splitDays(stored string) []string {
	if stored == "" {
		return []string{}
	}
	return strings.Split(stored, ",")
}

// ErrUnknownScheduleDashboard is returned when a slot of a display's
// schedule refers to a dashboard that does not exist.
var ErrUnknownScheduleDashboard = errors.New("schedule refers to an unknown dashboard")

// ErrUnknownRotationDashboard is returned when a display's rotation refers
// to a dashboard that does not exist.
var ErrUnknownRotationDashboard = errors.New("rotation refers to an unknown dashboard")

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// schedules returns the slots of every display, or of one display when
// displayID is set, in their stored order.
func schedules(ctx context.Context, q querier, displayID string) (map[string][]displays.Slot, error) {
	query := "SELECT display_id, starts, ends, days, dashboard_id FROM display_schedule"
	args := []any{}
	if displayID != "" {
		query += " WHERE display_id = ?"
		args = append(args, displayID)
	}
	query += " ORDER BY display_id, position LIMIT ?"
	args = append(args, displays.MaxDisplays*displays.MaxSlots)
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()
	result := map[string][]displays.Slot{}
	for rows.Next() {
		var id, days string
		var slot displays.Slot
		if err := rows.Scan(&id, &slot.From, &slot.Until, &days, &slot.DashboardID); err != nil {
			return nil, err
		}
		slot.Days = splitDays(days)
		result[id] = append(result[id], slot)
	}
	return result, rows.Err()
}

// replaceSchedule stores slots as the whole schedule of a display.
func replaceSchedule(ctx context.Context, tx *sql.Tx, displayID string, slots []displays.Slot) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM display_schedule WHERE display_id = ?", displayID); err != nil {
		return fmt.Errorf("clear schedule: %w", err)
	}
	for i, slot := range slots {
		if err := requireDashboard(ctx, tx, slot.DashboardID); err != nil {
			if errors.Is(err, ErrUnknownDashboard) {
				return ErrUnknownScheduleDashboard
			}
			return err
		}
		_, err := tx.ExecContext(ctx,
			"INSERT INTO display_schedule (display_id, position, starts, ends, days, dashboard_id) VALUES (?, ?, ?, ?, ?, ?)",
			displayID, i, slot.From, slot.Until, strings.Join(slot.Days, ","), slot.DashboardID)
		if err != nil {
			return fmt.Errorf("store schedule: %w", err)
		}
	}
	return nil
}

// rotations returns the rotation of every display, or of one display when
// displayID is set, in order.
func rotations(ctx context.Context, q querier, displayID string) (map[string][]string, error) {
	query := "SELECT display_id, dashboard_id FROM display_rotation"
	args := []any{}
	if displayID != "" {
		query += " WHERE display_id = ?"
		args = append(args, displayID)
	}
	query += " ORDER BY display_id, position LIMIT ?"
	args = append(args, displays.MaxDisplays*displays.MaxRotation)
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rotations: %w", err)
	}
	defer rows.Close()
	result := map[string][]string{}
	for rows.Next() {
		var id, dashboard string
		if err := rows.Scan(&id, &dashboard); err != nil {
			return nil, err
		}
		result[id] = append(result[id], dashboard)
	}
	return result, rows.Err()
}

// replaceRotation stores ids as the whole rotation of a display.
func replaceRotation(ctx context.Context, tx *sql.Tx, displayID string, ids []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM display_rotation WHERE display_id = ?", displayID); err != nil {
		return fmt.Errorf("clear rotation: %w", err)
	}
	for i, id := range ids {
		if err := requireDashboard(ctx, tx, id); err != nil {
			if errors.Is(err, ErrUnknownDashboard) {
				return ErrUnknownRotationDashboard
			}
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO display_rotation (display_id, position, dashboard_id) VALUES (?, ?, ?)", displayID, i, id); err != nil {
			return fmt.Errorf("store rotation: %w", err)
		}
	}
	return nil
}
