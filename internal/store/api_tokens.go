package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MaxAPITokens bounds how many API tokens exist at once. A home has a few
// automations; the limit keeps the list on the Settings page short.
const MaxAPITokens = 10

// APIToken is an API token without its secret.
type APIToken struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	// LastUsedAt is null until the token is used.
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

// NewAPIToken is an API token to store, by hash.
type NewAPIToken struct {
	Name    string
	Hash    string
	Created time.Time
}

// CreateAPIToken stores a token. It returns ErrConflict when MaxAPITokens
// tokens exist already.
func (s *Store) CreateAPIToken(ctx context.Context, t NewAPIToken) (APIToken, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return APIToken{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM api_tokens").Scan(&count); err != nil {
		return APIToken{}, fmt.Errorf("count api tokens: %w", err)
	}
	if count >= MaxAPITokens {
		return APIToken{}, fmt.Errorf("%w: the limit of %d API tokens is reached", ErrConflict, MaxAPITokens)
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO api_tokens (name, token_hash, created_at) VALUES (?, ?, ?)",
		t.Name, t.Hash, formatTime(t.Created))
	if err != nil {
		return APIToken{}, fmt.Errorf("create api token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return APIToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIToken{}, err
	}
	return APIToken{ID: id, Name: t.Name, CreatedAt: t.Created}, nil
}

// APITokens returns every token, oldest first.
func (s *Store) APITokens(ctx context.Context) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, created_at, last_used_at FROM api_tokens ORDER BY id LIMIT ?", MaxAPITokens)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()
	tokens := []APIToken{}
	for rows.Next() {
		t, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// APITokenByHash returns the token with this hash, or ErrNotFound.
func (s *Store) APITokenByHash(ctx context.Context, hash string) (APIToken, error) {
	t, err := scanAPIToken(s.db.QueryRowContext(ctx,
		"SELECT id, name, created_at, last_used_at FROM api_tokens WHERE token_hash = ?", hash))
	if errors.Is(err, sql.ErrNoRows) {
		return APIToken{}, ErrNotFound
	}
	return t, err
}

// TouchAPIToken records that a token was used.
func (s *Store) TouchAPIToken(ctx context.Context, id int64, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = ? WHERE id = ?", formatTime(now), id); err != nil {
		return fmt.Errorf("touch api token: %w", err)
	}
	return nil
}

// DeleteAPIToken revokes a token and returns its name, or ErrNotFound.
func (s *Store) DeleteAPIToken(ctx context.Context, id int64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, "DELETE FROM api_tokens WHERE id = ? RETURNING name", id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("delete api token: %w", err)
	}
	return name, nil
}

func scanAPIToken(row scanner) (APIToken, error) {
	var t APIToken
	var created, lastUsed string
	if err := row.Scan(&t.ID, &t.Name, &created, &lastUsed); err != nil {
		return APIToken{}, err
	}
	var err error
	if t.CreatedAt, err = parseTime(created); err != nil {
		return APIToken{}, fmt.Errorf("api token has invalid created_at: %w", err)
	}
	if lastUsed != "" {
		used, err := parseTime(lastUsed)
		if err != nil {
			return APIToken{}, fmt.Errorf("api token has invalid last_used_at: %w", err)
		}
		t.LastUsedAt = &used
	}
	return t, nil
}
