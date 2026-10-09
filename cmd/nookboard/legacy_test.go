package main

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAdoptOldDatabaseRenamesItWithItsJournal(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "daycast.db"), "db")
	write(t, filepath.Join(dir, "daycast.db-wal"), "wal")

	adopted, err := adoptOldDatabase(dir)
	if err != nil || !adopted {
		t.Fatalf("adoptOldDatabase = %v, %v; want true, nil", adopted, err)
	}
	if got := read(t, filepath.Join(dir, "nookboard.db")); got != "db" {
		t.Errorf("nookboard.db = %q, want the old database", got)
	}
	if got := read(t, filepath.Join(dir, "nookboard.db-wal")); got != "wal" {
		t.Errorf("nookboard.db-wal = %q, want the old journal", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "daycast.db")); !os.IsNotExist(err) {
		t.Errorf("daycast.db still exists: %v", err)
	}
}

func TestAdoptOldDatabaseKeepsTheCurrentOne(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "daycast.db"), "old")
	write(t, filepath.Join(dir, "nookboard.db"), "current")

	adopted, err := adoptOldDatabase(dir)
	if err != nil || adopted {
		t.Fatalf("adoptOldDatabase = %v, %v; want false, nil", adopted, err)
	}
	if got := read(t, filepath.Join(dir, "nookboard.db")); got != "current" {
		t.Errorf("nookboard.db = %q, want it untouched", got)
	}
}

func TestAdoptOldDatabaseWithoutOne(t *testing.T) {
	adopted, err := adoptOldDatabase(t.TempDir())
	if err != nil || adopted {
		t.Fatalf("adoptOldDatabase = %v, %v; want false, nil", adopted, err)
	}
}
