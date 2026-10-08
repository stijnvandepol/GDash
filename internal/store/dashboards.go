package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stijnvandepol/daycast/internal/display"
	"github.com/stijnvandepol/daycast/internal/displays"
)

// MaxDashboards bounds how many dashboards one server keeps.
const MaxDashboards = 50

// Dashboards returns all dashboards ordered by name.
func (s *Store) Dashboards(ctx context.Context) ([]display.Dashboard, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, widgets FROM dashboards ORDER BY name LIMIT ?", MaxDashboards)
	if err != nil {
		return nil, fmt.Errorf("list dashboards: %w", err)
	}
	defer rows.Close()
	var list []display.Dashboard
	for rows.Next() {
		d, err := scanDashboard(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// Dashboard returns one dashboard, or ErrNotFound.
func (s *Store) Dashboard(ctx context.Context, id string) (display.Dashboard, error) {
	d, err := scanDashboard(s.db.QueryRowContext(ctx, "SELECT id, name, widgets FROM dashboards WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return display.Dashboard{}, ErrNotFound
	}
	return d, err
}

// CreateDashboard stores a new dashboard. It returns ErrConflict when the id
// is taken or the limit is reached.
func (s *Store) CreateDashboard(ctx context.Context, d display.Dashboard, now time.Time) error {
	widgets, err := json.Marshal(d.Widgets)
	if err != nil {
		return fmt.Errorf("encode widgets: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM dashboards").Scan(&count); err != nil {
		return fmt.Errorf("count dashboards: %w", err)
	}
	if count >= MaxDashboards {
		return fmt.Errorf("%w: limit of %d dashboards reached", ErrConflict, MaxDashboards)
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO dashboards (id, name, widgets, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		d.ID, d.Name, string(widgets), formatTime(now), formatTime(now))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: dashboard %q", ErrConflict, d.ID)
		}
		return fmt.Errorf("create dashboard: %w", err)
	}
	return tx.Commit()
}

// UpdateDashboard replaces the name and widgets of a dashboard.
func (s *Store) UpdateDashboard(ctx context.Context, d display.Dashboard, now time.Time) error {
	widgets, err := json.Marshal(d.Widgets)
	if err != nil {
		return fmt.Errorf("encode widgets: %w", err)
	}
	res, err := s.db.ExecContext(ctx, "UPDATE dashboards SET name = ?, widgets = ?, updated_at = ? WHERE id = ?",
		d.Name, string(widgets), formatTime(now), d.ID)
	if err != nil {
		return fmt.Errorf("update dashboard: %w", err)
	}
	return requireOneRow(res)
}

// DeleteDashboard removes a dashboard; its displays fall back to the default
// dashboard. The default dashboard itself cannot be deleted (ErrConflict).
func (s *Store) DeleteDashboard(ctx context.Context, id string) error {
	if id == display.DefaultID {
		return fmt.Errorf("%w: the default dashboard cannot be deleted", ErrConflict)
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM dashboards WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete dashboard: %w", err)
	}
	return requireOneRow(res)
}

// EnsureDashboard creates d unless a dashboard with its id exists, so a
// fresh install starts with a default dashboard and later edits survive
// restarts.
func (s *Store) EnsureDashboard(ctx context.Context, d display.Dashboard, now time.Time) error {
	err := s.CreateDashboard(ctx, d, now)
	if errors.Is(err, ErrConflict) {
		if _, getErr := s.Dashboard(ctx, d.ID); getErr == nil {
			return nil
		}
	}
	return err
}

// DisplayDashboards returns, for every display, the dashboard it shows.
func (s *Store) DisplayDashboards(ctx context.Context) (map[string]display.Dashboard, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, b.id, b.name, b.widgets
		FROM displays d
		JOIN dashboards b ON b.id = COALESCE(d.dashboard_id, ?)
		LIMIT ?`, display.DefaultID, displays.MaxDisplays)
	if err != nil {
		return nil, fmt.Errorf("list display dashboards: %w", err)
	}
	defer rows.Close()
	result := map[string]display.Dashboard{}
	for rows.Next() {
		var displayID, widgets string
		var d display.Dashboard
		if err := rows.Scan(&displayID, &d.ID, &d.Name, &widgets); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(widgets), &d.Widgets); err != nil {
			return nil, fmt.Errorf("dashboard %q has invalid widgets: %w", d.ID, err)
		}
		result[displayID] = display.Normalize(d)
	}
	return result, rows.Err()
}

func scanDashboard(row scanner) (display.Dashboard, error) {
	var d display.Dashboard
	var widgets string
	if err := row.Scan(&d.ID, &d.Name, &widgets); err != nil {
		return display.Dashboard{}, err
	}
	if err := json.Unmarshal([]byte(widgets), &d.Widgets); err != nil {
		return display.Dashboard{}, fmt.Errorf("dashboard %q has invalid widgets: %w", d.ID, err)
	}
	return display.Normalize(d), nil
}
