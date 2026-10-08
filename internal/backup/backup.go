// Package backup keeps daily copies of the database, so a bad change or a
// broken upgrade can be undone. The copies live next to the database; a
// copy elsewhere protects against a failing disk, see docs/install.md.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	prefix = "daycast-"
	suffix = ".db"
	// checkInterval is how often Run looks whether a new day began. A
	// server that sleeps through midnight catches up within the hour.
	checkInterval = time.Hour
)

// Daily writes one copy a day and keeps the newest Keep of them.
type Daily struct {
	Dir  string
	Keep int
	// Write writes a consistent copy of the database to a path that does
	// not exist yet.
	Write  func(ctx context.Context, path string) error
	Now    func() time.Time
	Logger *slog.Logger
}

// Run makes today's copy if there is none, and keeps doing so every day
// until ctx ends. Failures are logged; the next check tries again.
func (d *Daily) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		if made, err := d.Once(ctx); err != nil {
			d.Logger.Error("daily backup failed; retrying within the hour", "error", err)
		} else if made != "" {
			d.Logger.Info("database backed up", "file", made)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Once writes today's copy unless it exists, removes copies beyond Keep,
// and returns the file it wrote, or "" when today's copy was already there.
func (d *Daily) Once(ctx context.Context) (string, error) {
	if err := os.MkdirAll(d.Dir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	name := prefix + d.Now().Format(time.DateOnly) + suffix
	target := filepath.Join(d.Dir, name)
	if _, err := os.Stat(target); err == nil {
		return "", nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	// Write to a temporary name first, so a crash never leaves a
	// half-written file that looks like a good backup.
	tmp := target + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := d.Write(ctx, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", fmt.Errorf("finish backup: %w", err)
	}
	return name, d.prune()
}

// prune removes the oldest copies beyond Keep. Names sort by date.
func (d *Daily) prune() error {
	entries, err := os.ReadDir(d.Dir)
	if err != nil {
		return err
	}
	var copies []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) && !e.IsDir() {
			copies = append(copies, n)
		}
	}
	slices.Sort(copies)
	for len(copies) > d.Keep {
		if err := os.Remove(filepath.Join(d.Dir, copies[0])); err != nil {
			return fmt.Errorf("remove old backup: %w", err)
		}
		copies = copies[1:]
	}
	return nil
}
