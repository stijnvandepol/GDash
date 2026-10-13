package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestCastSessionsSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := loadCastSessions(dir, logger)
	s.remember("kitchen", "abc")
	s.remember("hall", "def")
	s.remember("hall", "")

	again := loadCastSessions(dir, logger)
	if got := again.recall("kitchen"); got != "abc" {
		t.Errorf("kitchen = %q, want abc", got)
	}
	if got := again.recall("hall"); got != "" {
		t.Errorf("hall = %q, want it forgotten", got)
	}
}

func TestCastSessionsStartEmptyFromABrokenFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, castSessionsFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := loadCastSessions(dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := s.recall("kitchen"); got != "" {
		t.Errorf("recall = %q, want empty", got)
	}
	s.remember("kitchen", "abc")
	if got := loadCastSessions(dir, slog.New(slog.NewTextHandler(io.Discard, nil))).recall("kitchen"); got != "abc" {
		t.Errorf("after a write recall = %q, want abc", got)
	}
}
