// Package store keeps GDash's configuration in SQLite. It is the only
// package that writes SQL.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a record with the same key already exists.
var ErrConflict = errors.New("already exists")

// ErrUnknownDashboard is returned when a display refers to a dashboard that
// does not exist.
var ErrUnknownDashboard = errors.New("unknown dashboard")

// Store is an open GDash database.
type Store struct {
	db *sql.DB
}

// Open opens or creates the database at path and brings its schema up to
// date. Before changing an existing schema it writes a backup next to it.
// It creates the database's directory, readable by GDash only, so a first
// start needs no folder made by hand.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := ownerOnly(path); err != nil {
		return nil, err
	}
	dsn := "file:" + url.PathEscape(path) +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows one writer at a time; one connection avoids "database is
	// locked" errors at the scale of a home dashboard.
	db.SetMaxOpenConns(1)

	scripts, err := loadMigrations()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx, path, scripts); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func loadMigrations() ([]string, error) {
	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	scripts := make([]string, 0, len(names))
	for _, name := range names {
		body, err := migrationFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, string(body))
	}
	return scripts, nil
}

func (s *Store) migrate(ctx context.Context, path string, scripts []string) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > len(scripts) {
		return fmt.Errorf("database schema version %d is newer than this GDash (%d); use a newer release", version, len(scripts))
	}
	if version == len(scripts) {
		return nil
	}
	// An existing database is copied before every step, so each migration's
	// rollback note ("restore gdash.db.bak-v<N>") holds, also when one
	// upgrade applies several. A new database has nothing to save.
	existing := version > 0
	for i := version; i < len(scripts); i++ {
		if existing {
			if err := s.backup(ctx, path, i); err != nil {
				return err
			}
		}
		if err := s.apply(ctx, i+1, scripts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backup(ctx context.Context, path string, version int) error {
	target := fmt.Sprintf("%s.bak-v%d", path, version)
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove old backup: %w", err)
	}
	if err := s.WriteCopy(ctx, target); err != nil {
		return fmt.Errorf("back up database before migration: %w", err)
	}
	return nil
}

// WriteCopy writes a consistent copy of the database to target, which must
// not exist yet. It is safe while GDash runs: SQLite copies from one
// read transaction.
func (s *Store) WriteCopy(ctx context.Context, target string) error {
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", target); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}
	if err := os.Chmod(target, databaseMode); err != nil {
		return fmt.Errorf("restrict database copy: %w", err)
	}
	return nil
}

// databaseMode keeps the database, its journal and its copies readable by
// GDash only: they hold sessions, token hashes and the sealed secrets.
const databaseMode = 0o600

// ownerOnly makes the database file at path readable by its owner only,
// creating it empty when it does not exist yet. SQLite gives its journal
// files the mode of the database file.
func ownerOnly(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, databaseMode)
	if err != nil {
		return fmt.Errorf("create database file: %w", err)
	}
	f.Close()
	if err := os.Chmod(path, databaseMode); err != nil {
		return fmt.Errorf("restrict database file: %w", err)
	}
	return nil
}

func (s *Store) apply(ctx context.Context, version int, script string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("apply migration %d: %w", version, err)
	}
	// PRAGMA does not accept parameters; version is an integer we control.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return fmt.Errorf("record schema version %d: %w", version, err)
	}
	return tx.Commit()
}
