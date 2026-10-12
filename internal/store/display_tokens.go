package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MaxScreensPerDisplay bounds how many screens can be paired with one
// display. A display is one TV; a handful covers reloads and replacements.
const MaxScreensPerDisplay = 20

// Kinds of display token.
const (
	// TokenScreen is a screen paired by entering its code.
	TokenScreen = "screen"
	// TokenCast is the page a cast keeper opened; there is at most one per
	// display, replaced on every launch.
	TokenCast = "cast"
)

// NewDisplayToken is a token to store for a screen.
type NewDisplayToken struct {
	Hash      string
	DisplayID string
	// Kind is TokenScreen or TokenCast.
	Kind    string
	Created time.Time
}

// DisplayToken is a paired screen's token, by hash.
type DisplayToken struct {
	DisplayID  string
	LastSeenAt time.Time
}

// CreateDisplayToken pairs a screen with a display. A TokenCast token
// replaces the display's previous one. It returns ErrNotFound when the
// display does not exist and ErrConflict when the display already has
// MaxScreensPerDisplay screens.
func (s *Store) CreateDisplayToken(ctx context.Context, t NewDisplayToken) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM displays WHERE id = ?)", t.DisplayID).Scan(&exists); err != nil {
		return fmt.Errorf("check display: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	if t.Kind == TokenCast {
		if _, err := tx.ExecContext(ctx, "DELETE FROM display_tokens WHERE display_id = ? AND kind = ?", t.DisplayID, TokenCast); err != nil {
			return fmt.Errorf("replace cast token: %w", err)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM display_tokens WHERE display_id = ?", t.DisplayID).Scan(&count); err != nil {
		return fmt.Errorf("count paired screens: %w", err)
	}
	if count >= MaxScreensPerDisplay {
		return fmt.Errorf("%w: display %q already has %d paired screens", ErrConflict, t.DisplayID, count)
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO display_tokens (token_hash, display_id, kind, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)",
		t.Hash, t.DisplayID, t.Kind, formatTime(t.Created), formatTime(t.Created))
	if err != nil {
		return fmt.Errorf("create display token: %w", err)
	}
	return tx.Commit()
}

// DisplayToken returns the display a token belongs to, or ErrNotFound.
func (s *Store) DisplayToken(ctx context.Context, tokenHash string) (DisplayToken, error) {
	var t DisplayToken
	var lastSeen string
	err := s.db.QueryRowContext(ctx, "SELECT display_id, last_seen_at FROM display_tokens WHERE token_hash = ?", tokenHash).
		Scan(&t.DisplayID, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return DisplayToken{}, ErrNotFound
	}
	if err != nil {
		return DisplayToken{}, fmt.Errorf("find display token: %w", err)
	}
	if t.LastSeenAt, err = parseTime(lastSeen); err != nil {
		return DisplayToken{}, fmt.Errorf("display token has invalid last_seen_at: %w", err)
	}
	return t, nil
}

// TouchDisplayToken records that a paired screen connected.
func (s *Store) TouchDisplayToken(ctx context.Context, tokenHash string, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE display_tokens SET last_seen_at = ? WHERE token_hash = ?", formatTime(now), tokenHash); err != nil {
		return fmt.Errorf("touch display token: %w", err)
	}
	return nil
}

// PairedScreenCounts returns, per display, how many screens were paired with
// a code. Displays without paired screens are absent.
func (s *Store) PairedScreenCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT display_id, count(*) FROM display_tokens WHERE kind = ? GROUP BY display_id", TokenScreen)
	if err != nil {
		return nil, fmt.Errorf("count paired screens: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		counts[id] = n
	}
	return counts, rows.Err()
}

// DeleteDisplayToken unpairs one screen, if its token still exists.
func (s *Store) DeleteDisplayToken(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM display_tokens WHERE token_hash = ?", tokenHash); err != nil {
		return fmt.Errorf("delete display token: %w", err)
	}
	return nil
}

// DeleteScreenTokens unpairs the screens of a display that were paired with
// a code and returns how many there were. The page a cast keeper opened
// keeps its token: the keeper put it there, and it is replaced on the next
// launch anyway.
func (s *Store) DeleteScreenTokens(ctx context.Context, displayID string) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM display_tokens WHERE display_id = ? AND kind = ?", displayID, TokenScreen)
	if err != nil {
		return 0, fmt.Errorf("delete display tokens: %w", err)
	}
	return res.RowsAffected()
}
