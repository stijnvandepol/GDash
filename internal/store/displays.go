package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/displays"
)

// Displays returns all displays ordered by name.
func (s *Store) Displays(ctx context.Context) ([]displays.Display, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, cast_device_id, dashboard_id, active_from, active_until, rotate_seconds, text_scale, screensaver_minutes, created_at FROM displays ORDER BY name LIMIT ?", displays.MaxDisplays)
	if err != nil {
		return nil, fmt.Errorf("list displays: %w", err)
	}
	defer rows.Close()
	var list []displays.Display
	for rows.Next() {
		d, err := scanDisplay(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slots, err := schedules(ctx, s.db, "")
	if err != nil {
		return nil, err
	}
	turns, err := rotations(ctx, s.db, "")
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Schedule = withSlots(slots[list[i].ID])
		list[i].Rotation = displays.NormalizeRotation(turns[list[i].ID])
	}
	return list, nil
}

// withSlots returns an empty schedule instead of nil, so the API always
// sends a list.
func withSlots(slots []displays.Slot) []displays.Slot {
	if slots == nil {
		return []displays.Slot{}
	}
	return slots
}

// Display returns one display, or ErrNotFound.
func (s *Store) Display(ctx context.Context, id string) (displays.Display, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, name, cast_device_id, dashboard_id, active_from, active_until, rotate_seconds, text_scale, screensaver_minutes, created_at FROM displays WHERE id = ?", id)
	d, err := scanDisplay(row)
	if errors.Is(err, sql.ErrNoRows) {
		return displays.Display{}, ErrNotFound
	}
	if err != nil {
		return displays.Display{}, err
	}
	slots, err := schedules(ctx, s.db, id)
	if err != nil {
		return displays.Display{}, err
	}
	d.Schedule = withSlots(slots[id])
	turns, err := rotations(ctx, s.db, id)
	if err != nil {
		return displays.Display{}, err
	}
	d.Rotation = displays.NormalizeRotation(turns[id])
	return d, nil
}

// CreateDisplay stores a new display. It returns ErrConflict when the id is
// taken and when the display limit is reached.
func (s *Store) CreateDisplay(ctx context.Context, d displays.Display) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM displays").Scan(&count); err != nil {
		return fmt.Errorf("count displays: %w", err)
	}
	if count >= displays.MaxDisplays {
		return fmt.Errorf("%w: limit of %d displays reached", ErrConflict, displays.MaxDisplays)
	}
	if err := requireDashboard(ctx, tx, d.DashboardID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO displays (id, name, cast_device_id, dashboard_id, active_from, active_until, rotate_seconds, text_scale, screensaver_minutes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		d.ID, d.Name, d.CastDeviceID, nullable(d.DashboardID), d.From, d.Until, d.RotateSeconds, d.TextScale, d.ScreensaverMinutes, formatTime(d.CreatedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: display %q", ErrConflict, d.ID)
		}
		return fmt.Errorf("create display: %w", err)
	}
	if err := replaceSchedule(ctx, tx, d.ID, d.Schedule); err != nil {
		return err
	}
	if err := replaceRotation(ctx, tx, d.ID, d.Rotation); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateDisplay changes the name, Cast device, dashboard, active hours and
// schedule of an existing display.
func (s *Store) UpdateDisplay(ctx context.Context, d displays.Display) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireDashboard(ctx, tx, d.DashboardID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE displays SET name = ?, cast_device_id = ?, dashboard_id = ?, active_from = ?, active_until = ?, rotate_seconds = ?, text_scale = ?, screensaver_minutes = ? WHERE id = ?",
		d.Name, d.CastDeviceID, nullable(d.DashboardID), d.From, d.Until, d.RotateSeconds, d.TextScale, d.ScreensaverMinutes, d.ID)
	if err != nil {
		return fmt.Errorf("update display: %w", err)
	}
	if err := requireOneRow(res); err != nil {
		return err
	}
	if err := replaceSchedule(ctx, tx, d.ID, d.Schedule); err != nil {
		return err
	}
	if err := replaceRotation(ctx, tx, d.ID, d.Rotation); err != nil {
		return err
	}
	return tx.Commit()
}

// requireDashboard returns ErrUnknownDashboard when id is set and no such
// dashboard exists, so the API can name the field instead of failing on
// the foreign key.
func requireDashboard(ctx context.Context, tx *sql.Tx, id string) error {
	if id == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM dashboards WHERE id = ?)", id).Scan(&exists); err != nil {
		return fmt.Errorf("check dashboard: %w", err)
	}
	if !exists {
		return ErrUnknownDashboard
	}
	return nil
}

// nullable stores an empty string as NULL, for optional references.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// DeleteDisplay removes a display, or returns ErrNotFound.
func (s *Store) DeleteDisplay(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM displays WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete display: %w", err)
	}
	return requireOneRow(res)
}

type scanner interface{ Scan(dest ...any) error }

func scanDisplay(row scanner) (displays.Display, error) {
	var d displays.Display
	var created string
	var dashboard sql.NullString
	if err := row.Scan(&d.ID, &d.Name, &d.CastDeviceID, &dashboard, &d.From, &d.Until, &d.RotateSeconds, &d.TextScale, &d.ScreensaverMinutes, &created); err != nil {
		return displays.Display{}, err
	}
	d.DashboardID = dashboard.String
	t, err := parseTime(created)
	if err != nil {
		return displays.Display{}, fmt.Errorf("display %q has invalid created_at: %w", d.ID, err)
	}
	d.CreatedAt = t
	return d, nil
}

func requireOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation recognises SQLite's constraint error by its message,
// which is stable across driver versions, instead of importing the
// driver's internal error codes.
func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "PRIMARY KEY")
}
