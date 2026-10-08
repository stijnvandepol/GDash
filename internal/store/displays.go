package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/stijnvandepol/daycast/internal/displays"
)

// Displays returns all displays ordered by name.
func (s *Store) Displays(ctx context.Context) ([]displays.Display, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, cast_device_id, created_at FROM displays ORDER BY name LIMIT ?", displays.MaxDisplays)
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
	return list, rows.Err()
}

// Display returns one display, or ErrNotFound.
func (s *Store) Display(ctx context.Context, id string) (displays.Display, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, name, cast_device_id, created_at FROM displays WHERE id = ?", id)
	d, err := scanDisplay(row)
	if errors.Is(err, sql.ErrNoRows) {
		return displays.Display{}, ErrNotFound
	}
	return d, err
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
	_, err = tx.ExecContext(ctx,
		"INSERT INTO displays (id, name, cast_device_id, created_at) VALUES (?, ?, ?, ?)",
		d.ID, d.Name, d.CastDeviceID, formatTime(d.CreatedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: display %q", ErrConflict, d.ID)
		}
		return fmt.Errorf("create display: %w", err)
	}
	return tx.Commit()
}

// UpdateDisplay changes the name and Cast device of an existing display.
func (s *Store) UpdateDisplay(ctx context.Context, d displays.Display) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE displays SET name = ?, cast_device_id = ? WHERE id = ?", d.Name, d.CastDeviceID, d.ID)
	if err != nil {
		return fmt.Errorf("update display: %w", err)
	}
	return requireOneRow(res)
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
	if err := row.Scan(&d.ID, &d.Name, &d.CastDeviceID, &created); err != nil {
		return displays.Display{}, err
	}
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
