package backup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newDaily(t *testing.T, now *time.Time) (*Daily, *int) {
	t.Helper()
	writes := 0
	return &Daily{
		Dir:  filepath.Join(t.TempDir(), "backups"),
		Keep: 3,
		Write: func(_ context.Context, path string) error {
			writes++
			return os.WriteFile(path, []byte("copy"), 0o600)
		},
		Now:    func() time.Time { return *now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, &writes
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestOnceWritesOneCopyPerDayAndKeepsTheNewest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	d, writes := newDaily(t, &now)

	if made, err := d.Once(ctx); err != nil || made != "nookboard-2026-10-09.db" {
		t.Fatalf("first Once = %q, %v", made, err)
	}
	if made, err := d.Once(ctx); err != nil || made != "" || *writes != 1 {
		t.Fatalf("second Once on the same day = %q, %v (%d writes); want nothing written", made, err, *writes)
	}
	for range 4 {
		now = now.AddDate(0, 0, 1)
		if _, err := d.Once(ctx); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"nookboard-2026-10-11.db", "nookboard-2026-10-12.db", "nookboard-2026-10-13.db"}
	if got := names(t, d.Dir); !slices.Equal(got, want) {
		t.Fatalf("backups = %v, want the newest %d: %v", got, d.Keep, want)
	}
}

func TestFailedWriteLeavesNoBackup(t *testing.T) {
	now := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	d, _ := newDaily(t, &now)
	d.Write = func(_ context.Context, path string) error {
		_ = os.WriteFile(path, []byte("half"), 0o600)
		return errors.New("disk full")
	}
	if _, err := d.Once(context.Background()); err == nil {
		t.Fatal("Once hid the write error")
	}
	for _, n := range names(t, d.Dir) {
		if n == "nookboard-2026-10-09.db" {
			t.Fatal("a failed copy was left under the name of a good backup")
		}
	}
	// The next attempt replaces the leftover temporary file.
	d.Write = func(_ context.Context, path string) error { return os.WriteFile(path, []byte("copy"), 0o600) }
	if made, err := d.Once(context.Background()); err != nil || made == "" {
		t.Fatalf("retry = %q, %v", made, err)
	}
}

func TestPruneLeavesOtherFilesAlone(t *testing.T) {
	now := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	d, _ := newDaily(t, &now)
	d.Keep = 1
	if err := os.MkdirAll(d.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"nookboard-2026-01-01.db", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(d.Dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := names(t, d.Dir); !slices.Equal(got, []string{"nookboard-2026-10-09.db", "notes.txt"}) {
		t.Fatalf("files = %v", got)
	}
}
