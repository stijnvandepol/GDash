package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is an account of the web interface.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Session is a logged-in browser.
type Session struct {
	User       User
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// timeLayout has a fixed width, so timestamps stored as text sort and
// compare correctly in SQL. RFC3339Nano drops trailing zeros and would not.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// HasUsers reports whether any account exists.
func (s *Store) HasUsers(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users)").Scan(&exists); err != nil {
		return false, fmt.Errorf("check for users: %w", err)
	}
	return exists, nil
}

// CreateFirstUser creates the initial admin account. It returns ErrConflict
// when an account already exists, so setup can only ever run once, even if
// two requests race.
func (s *Store) CreateFirstUser(ctx context.Context, username, passwordHash string, now time.Time) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users)").Scan(&exists); err != nil {
		return User{}, fmt.Errorf("check for users: %w", err)
	}
	if exists {
		return User{}, fmt.Errorf("%w: setup already done", ErrConflict)
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)", username, passwordHash, formatTime(now))
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: now.UTC()}, nil
}

// UserByName returns the account with this username (case-insensitive).
func (s *Store) UserByName(ctx context.Context, username string) (User, error) {
	var u User
	var created string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, username, password_hash, created_at FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}
	if u.CreatedAt, err = parseTime(created); err != nil {
		return User{}, fmt.Errorf("user %d has invalid created_at: %w", u.ID, err)
	}
	return u, nil
}

// CreateSession stores a new session for userID.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at) VALUES (?, ?, ?, ?)",
		tokenHash, userID, formatTime(now), formatTime(now))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByHash returns the session and its user, or ErrNotFound.
func (s *Store) SessionByHash(ctx context.Context, tokenHash string) (Session, error) {
	var sess Session
	var userCreated, created, lastSeen string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.created_at, s.created_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, tokenHash).
		Scan(&sess.User.ID, &sess.User.Username, &sess.User.PasswordHash, &userCreated, &created, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}
	for _, f := range []struct {
		dst *time.Time
		src string
	}{{&sess.User.CreatedAt, userCreated}, {&sess.CreatedAt, created}, {&sess.LastSeenAt, lastSeen}} {
		if *f.dst, err = parseTime(f.src); err != nil {
			return Session{}, fmt.Errorf("session has invalid timestamp: %w", err)
		}
	}
	return sess, nil
}

// TouchSession records that a session was used.
func (s *Store) TouchSession(ctx context.Context, tokenHash string, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?", formatTime(now), tokenHash); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// DeleteSession removes one session. Deleting a session that does not exist
// is not an error: the result, being logged out, is the same.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes sessions idle since before idleCutoff or
// created before absoluteCutoff.
func (s *Store) DeleteExpiredSessions(ctx context.Context, idleCutoff, absoluteCutoff time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE last_seen_at < ? OR created_at < ?", formatTime(idleCutoff), formatTime(absoluteCutoff))
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// ChangePassword stores a new password hash and ends every session of the
// user except the one with keepTokenHash, in one transaction.
func (s *Store) ChangePassword(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := setPassword(ctx, tx, userID, passwordHash, keepTokenHash); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPassword stores a new password hash, ends every session of the user
// and turns two-step login off, in one transaction, for an admin who lost
// the password or the phone. It reports whether two-step login was on.
func (s *Store) ResetPassword(ctx context.Context, userID int64, passwordHash string) (twoStepWasOn bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := setPassword(ctx, tx, userID, passwordHash, ""); err != nil {
		return false, err
	}
	if twoStepWasOn, err = deleteTwoStep(ctx, tx, userID); err != nil {
		return false, err
	}
	return twoStepWasOn, tx.Commit()
}

func setPassword(ctx context.Context, tx *sql.Tx, userID int64, passwordHash, keepTokenHash string) error {
	res, err := tx.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	if err := requireOneRow(res); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND token_hash != ?", userID, keepTokenHash); err != nil {
		return fmt.Errorf("end other sessions: %w", err)
	}
	return nil
}
