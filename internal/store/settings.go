package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Kinds of data source.
const (
	SourceNews     = "news"
	SourceCalendar = "calendar"
)

// maxSourcesPerKind bounds the source lists, so every read is bounded.
// internal/settings enforces its own, lower limits.
const maxSourcesPerKind = 50

// Source is a news feed or calendar. For a calendar, Address is sealed by
// internal/secret; Host is stored apart so it can be shown.
type Source struct {
	ID      int64
	Kind    string
	Name    string
	Address string
	Host    string
	// Color is a palette name chosen in the web interface, or empty.
	Color     string
	CreatedAt time.Time
}

// Setting returns the value stored at key; ok is false when there is none.
func (s *Store) Setting(ctx context.Context, key string) (value string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %s: %w", key, err)
	}
	return value, true, nil
}

// SetSetting stores value at key, replacing what was there.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	if err != nil {
		return fmt.Errorf("store setting %s: %w", key, err)
	}
	return nil
}

// DeleteSetting removes the value at key, if any.
func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM settings WHERE key = ?", key); err != nil {
		return fmt.Errorf("delete setting %s: %w", key, err)
	}
	return nil
}

// Sources returns the sources of one kind in the order they were added.
func (s *Store) Sources(ctx context.Context, kind string) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, kind, name, address, host, color, created_at FROM sources WHERE kind = ? ORDER BY id LIMIT ?", kind, maxSourcesPerKind)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	var list []Source
	for rows.Next() {
		var src Source
		var created string
		if err := rows.Scan(&src.ID, &src.Kind, &src.Name, &src.Address, &src.Host, &src.Color, &created); err != nil {
			return nil, err
		}
		if src.CreatedAt, err = parseTime(created); err != nil {
			return nil, fmt.Errorf("source %d has invalid created_at: %w", src.ID, err)
		}
		list = append(list, src)
	}
	return list, rows.Err()
}

// AddSource stores a source and returns its ID. It returns ErrConflict when
// limit sources of that kind exist already.
func (s *Store) AddSource(ctx context.Context, src Source, limit int) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sources WHERE kind = ?", src.Kind).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sources: %w", err)
	}
	if count >= min(limit, maxSourcesPerKind) {
		return 0, fmt.Errorf("%w: limit of %d %s sources reached", ErrConflict, limit, src.Kind)
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO sources (kind, name, address, host, color, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		src.Kind, src.Name, src.Address, src.Host, src.Color, formatTime(src.CreatedAt))
	if err != nil {
		return 0, fmt.Errorf("add source: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// SetSourceColor changes the colour of a source of the given kind, or
// returns ErrNotFound.
func (s *Store) SetSourceColor(ctx context.Context, kind string, id int64, color string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE sources SET color = ? WHERE kind = ? AND id = ?", color, kind, id)
	if err != nil {
		return fmt.Errorf("set source color: %w", err)
	}
	return requireOneRow(res)
}

// DeleteSource removes a source of the given kind, or returns ErrNotFound.
func (s *Store) DeleteSource(ctx context.Context, kind string, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM sources WHERE kind = ? AND id = ?", kind, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return requireOneRow(res)
}
