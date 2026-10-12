package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TwoStep is the two-step login of an account.
type TwoStep struct {
	// SecretSealed is the authenticator secret, sealed by internal/secret.
	SecretSealed string
	// Enabled is false while the setup waits for its first code.
	Enabled bool
	// LastStep is the newest time step whose code was used.
	LastStep  int64
	CreatedAt time.Time
	// FailedCodes counts the wrong codes in a row, the last one at
	// LastFailedAt (zero while FailedCodes is 0). A right code, a new setup
	// and turning two-step login off end the run.
	FailedCodes  int
	LastFailedAt time.Time
}

// TwoStep returns the two-step login of userID, or ErrNotFound when it was
// never set up.
func (s *Store) TwoStep(ctx context.Context, userID int64) (TwoStep, error) {
	var t TwoStep
	var created, lastFailed string
	err := s.db.QueryRowContext(ctx, `
		SELECT secret_sealed, enabled, last_step, created_at, failed_codes, last_failed_at
		FROM two_step WHERE user_id = ?`, userID).
		Scan(&t.SecretSealed, &t.Enabled, &t.LastStep, &created, &t.FailedCodes, &lastFailed)
	if errors.Is(err, sql.ErrNoRows) {
		return TwoStep{}, ErrNotFound
	}
	if err != nil {
		return TwoStep{}, fmt.Errorf("find two-step login: %w", err)
	}
	if t.CreatedAt, err = parseTime(created); err != nil {
		return TwoStep{}, fmt.Errorf("two-step login of user %d has invalid created_at: %w", userID, err)
	}
	if lastFailed != "" {
		if t.LastFailedAt, err = parseTime(lastFailed); err != nil {
			return TwoStep{}, fmt.Errorf("two-step login of user %d has invalid last_failed_at: %w", userID, err)
		}
	}
	return t, nil
}

// StartTwoStep stores a new secret that waits for its first code, replacing
// an earlier setup that never got one. It returns ErrConflict when two-step
// login is already on: that secret is only replaced by turning it off.
func (s *Store) StartTwoStep(ctx context.Context, userID int64, secretSealed string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO two_step (user_id, secret_sealed, enabled, last_step, created_at) VALUES (?, ?, 0, 0, ?)
		ON CONFLICT (user_id) DO UPDATE SET secret_sealed = excluded.secret_sealed, last_step = 0,
			created_at = excluded.created_at, failed_codes = 0, last_failed_at = ''
		WHERE two_step.enabled = 0`, userID, secretSealed, formatTime(now))
	if err != nil {
		return fmt.Errorf("start two-step login: %w", err)
	}
	if err := requireOneRow(res); errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%w: two-step login is already on", ErrConflict)
	} else if err != nil {
		return err
	}
	return nil
}

// EnableTwoStep turns on the setup of userID with secretSealed after its
// first code, of time step step, stores the hashes of its recovery codes,
// and ends every other session of the user except keepTokenHash, in one
// transaction. It returns ErrNotFound when no setup waits, and ErrConflict
// when the setup that waits has another secret: a newer setup replaced the
// one whose code was checked.
func (s *Store) EnableTwoStep(ctx context.Context, userID int64, secretSealed string, step int64, recoveryHashes []string, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE two_step SET enabled = 1, last_step = ?, failed_codes = 0, last_failed_at = ''
		WHERE user_id = ? AND enabled = 0 AND secret_sealed = ?`, step, userID, secretSealed)
	if err != nil {
		return fmt.Errorf("enable two-step login: %w", err)
	}
	if err := requireOneRow(res); errors.Is(err, ErrNotFound) {
		return setupGone(ctx, tx, userID)
	} else if err != nil {
		return err
	}
	for _, hash := range recoveryHashes {
		if _, err := tx.ExecContext(ctx, "INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)", userID, hash); err != nil {
			return fmt.Errorf("store recovery code: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND token_hash != ?", userID, keepTokenHash); err != nil {
		return fmt.Errorf("end other sessions: %w", err)
	}
	return tx.Commit()
}

// setupGone says why no setup of userID was turned on: none waits
// (ErrNotFound), or one with another secret does (ErrConflict).
func setupGone(ctx context.Context, tx *sql.Tx, userID int64) error {
	var enabled bool
	err := tx.QueryRowContext(ctx, "SELECT enabled FROM two_step WHERE user_id = ?", userID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) || err == nil && enabled {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find two-step setup: %w", err)
	}
	return fmt.Errorf("%w: two-step login was set up again with another secret", ErrConflict)
}

// CountCodeAttempt counts a code for the two-step login of userID as wrong
// before it is checked, and reports false when the login changed since it
// was read with secretSealed and failedCodes wrong codes in a row: another
// attempt came first, or the setup was replaced. Counting first means that
// parallel attempts cannot all pass a limit before one of them counts; a
// right code ends the run.
func (s *Store) CountCodeAttempt(ctx context.Context, userID int64, secretSealed string, failedCodes int, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE two_step SET failed_codes = failed_codes + 1, last_failed_at = ?
		WHERE user_id = ? AND secret_sealed = ? AND failed_codes = ?`,
		formatTime(now), userID, secretSealed, failedCodes)
	if err != nil {
		return false, fmt.Errorf("count two-step code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// UseTwoStepStep records that the code of step was used, which ends the run
// of wrong codes, and reports false when that step or a later one was used
// before: a replayed code. Two requests with the same code race on one row,
// and only one wins.
func (s *Store) UseTwoStepStep(ctx context.Context, userID, step int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE two_step SET last_step = ?, failed_codes = 0, last_failed_at = ''
		WHERE user_id = ? AND last_step < ?`, step, userID, step)
	if err != nil {
		return false, fmt.Errorf("record two-step code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// UseRecoveryCode deletes the recovery code with codeHash, which ends the
// run of wrong codes, and returns how many are left. ok is false when the
// user has no such code.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, codeHash string) (left int, ok bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "DELETE FROM recovery_codes WHERE user_id = ? AND code_hash = ?", userID, codeHash)
	if err != nil {
		return 0, false, fmt.Errorf("use recovery code: %w", err)
	}
	if err := requireOneRow(res); errors.Is(err, ErrNotFound) {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE two_step SET failed_codes = 0, last_failed_at = '' WHERE user_id = ?", userID); err != nil {
		return 0, false, fmt.Errorf("reset wrong two-step codes: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_codes WHERE user_id = ?", userID).Scan(&left); err != nil {
		return 0, false, fmt.Errorf("count recovery codes: %w", err)
	}
	return left, true, tx.Commit()
}

// RecoveryCodesLeft returns how many unused recovery codes userID has.
func (s *Store) RecoveryCodesLeft(ctx context.Context, userID int64) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_codes WHERE user_id = ?", userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count recovery codes: %w", err)
	}
	return n, nil
}

// DeleteTwoStep turns two-step login of userID off, with its recovery
// codes and its count of wrong codes, and reports whether it was on. A
// setup that never got its first code is removed too.
func (s *Store) DeleteTwoStep(ctx context.Context, userID int64) (wasOn bool, err error) {
	return deleteTwoStep(ctx, s.db, userID)
}

// rowQuerier is a database or a transaction.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func deleteTwoStep(ctx context.Context, db rowQuerier, userID int64) (bool, error) {
	var wasOn bool
	err := db.QueryRowContext(ctx, "DELETE FROM two_step WHERE user_id = ? RETURNING enabled", userID).Scan(&wasOn)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("turn off two-step login: %w", err)
	}
	return wasOn, nil
}
