package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// maxCustomSources bounds the list, so every read is bounded.
// internal/custom enforces its own, lower limit.
const maxCustomSources = 50

// CustomSource is a JSON HTTP API as stored. Headers is JSON; the secret
// values are sealed together in SecretsSealed by internal/custom.
type CustomSource struct {
	ID              int64
	Name            string
	URL             string
	Method          string
	Body            string
	Auth            string
	AuthName        string
	Headers         string
	SecretsSealed   string
	IntervalSeconds int
	Sample          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const customSourceColumns = "id, name, url, method, body, auth, auth_name, headers, secrets_sealed, interval_seconds, sample, created_at, updated_at"

// CustomSources returns every custom source in the order they were added.
func (s *Store) CustomSources(ctx context.Context) ([]CustomSource, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+customSourceColumns+" FROM custom_sources ORDER BY id LIMIT ?", maxCustomSources)
	if err != nil {
		return nil, fmt.Errorf("list custom sources: %w", err)
	}
	defer rows.Close()
	var list []CustomSource
	for rows.Next() {
		src, err := scanCustomSource(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, src)
	}
	return list, rows.Err()
}

// CustomSource returns one custom source, or ErrNotFound.
func (s *Store) CustomSource(ctx context.Context, id int64) (CustomSource, error) {
	src, err := scanCustomSource(s.db.QueryRowContext(ctx, "SELECT "+customSourceColumns+" FROM custom_sources WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return CustomSource{}, ErrNotFound
	}
	return src, err
}

// AddCustomSource stores a source and returns its ID. It returns
// ErrConflict when limit sources exist already.
func (s *Store) AddCustomSource(ctx context.Context, src CustomSource, limit int) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM custom_sources").Scan(&count); err != nil {
		return 0, fmt.Errorf("count custom sources: %w", err)
	}
	if count >= min(limit, maxCustomSources) {
		return 0, fmt.Errorf("%w: limit of %d custom sources reached", ErrConflict, limit)
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO custom_sources (name, url, method, body, auth, auth_name, headers, secrets_sealed, interval_seconds, sample, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		src.Name, src.URL, src.Method, src.Body, src.Auth, src.AuthName, src.Headers, src.SecretsSealed,
		src.IntervalSeconds, src.Sample, formatTime(src.CreatedAt), formatTime(src.UpdatedAt))
	if err != nil {
		return 0, fmt.Errorf("add custom source: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateCustomSource replaces everything of a source except its ID and
// creation time, or returns ErrNotFound.
func (s *Store) UpdateCustomSource(ctx context.Context, src CustomSource) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE custom_sources SET name = ?, url = ?, method = ?, body = ?, auth = ?, auth_name = ?, headers = ?,
		 secrets_sealed = ?, interval_seconds = ?, sample = ?, updated_at = ? WHERE id = ?`,
		src.Name, src.URL, src.Method, src.Body, src.Auth, src.AuthName, src.Headers, src.SecretsSealed,
		src.IntervalSeconds, src.Sample, formatTime(src.UpdatedAt), src.ID)
	if err != nil {
		return fmt.Errorf("update custom source: %w", err)
	}
	return requireOneRow(res)
}

// DeleteCustomSource removes a source, or returns ErrNotFound.
func (s *Store) DeleteCustomSource(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM custom_sources WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete custom source: %w", err)
	}
	return requireOneRow(res)
}

func scanCustomSource(row interface{ Scan(...any) error }) (CustomSource, error) {
	var src CustomSource
	var created, updated string
	err := row.Scan(&src.ID, &src.Name, &src.URL, &src.Method, &src.Body, &src.Auth, &src.AuthName, &src.Headers,
		&src.SecretsSealed, &src.IntervalSeconds, &src.Sample, &created, &updated)
	if err != nil {
		return CustomSource{}, err
	}
	if src.CreatedAt, err = parseTime(created); err != nil {
		return CustomSource{}, fmt.Errorf("custom source %d has invalid created_at: %w", src.ID, err)
	}
	if src.UpdatedAt, err = parseTime(updated); err != nil {
		return CustomSource{}, fmt.Errorf("custom source %d has invalid updated_at: %w", src.ID, err)
	}
	return src, nil
}
