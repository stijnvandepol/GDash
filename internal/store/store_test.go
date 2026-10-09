package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/displays"
)

var created = time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "daycast.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func kitchen() displays.Display {
	return displays.Display{ID: "keuken", Name: "Keuken TV", CastDeviceID: "e542c41e4d94baa86e46f36dfdebf26e",
		Schedule: []displays.Slot{}, Rotation: []string{}, CreatedAt: created}
}

func TestDisplayLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)

	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}
	got, err := s.Display(ctx, "keuken")
	if err != nil {
		t.Fatalf("Display: %v", err)
	}
	if !reflect.DeepEqual(got, kitchen()) {
		t.Fatalf("Display = %+v, want %+v", got, kitchen())
	}

	renamed := kitchen()
	renamed.Name = "Keuken"
	renamed.CastDeviceID = ""
	if err := s.UpdateDisplay(ctx, renamed); err != nil {
		t.Fatalf("UpdateDisplay: %v", err)
	}
	list, err := s.Displays(ctx)
	if err != nil {
		t.Fatalf("Displays: %v", err)
	}
	if len(list) != 1 || !reflect.DeepEqual(list[0], renamed) {
		t.Fatalf("Displays = %+v, want [%+v]", list, renamed)
	}

	if err := s.DeleteDisplay(ctx, "keuken"); err != nil {
		t.Fatalf("DeleteDisplay: %v", err)
	}
	if _, err := s.Display(ctx, "keuken"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Display after delete: err = %v, want ErrNotFound", err)
	}
}

func TestCreateDisplayRejectsDuplicateID(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDisplay(ctx, kitchen()); !errors.Is(err, ErrConflict) {
		t.Fatalf("second CreateDisplay: err = %v, want ErrConflict", err)
	}
}

func TestCreateDisplayEnforcesLimit(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	for i := range displays.MaxDisplays {
		d := displays.Display{ID: "d" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Name: "x", CreatedAt: created}
		if err := s.CreateDisplay(ctx, d); err != nil {
			t.Fatalf("CreateDisplay %d: %v", i, err)
		}
	}
	over := displays.Display{ID: "one-too-many", Name: "x", CreatedAt: created}
	if err := s.CreateDisplay(ctx, over); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateDisplay above limit: err = %v, want ErrConflict", err)
	}
}

func TestUpdateAndDeleteMissingDisplay(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	if err := s.UpdateDisplay(ctx, kitchen()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateDisplay: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteDisplay(ctx, "keuken"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteDisplay: err = %v, want ErrNotFound", err)
	}
}

func TestReopenKeepsDataAndSkipsMigrations(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	s.Close()

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	if _, err := reopened.Display(ctx, "keuken"); err != nil {
		t.Fatalf("display lost after reopen: %v", err)
	}
	backups, _ := filepath.Glob(path + ".bak-*")
	if len(backups) != 0 {
		t.Fatalf("backup written although no migration ran: %v", backups)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database missing after reopen: %v", err)
	}
}

func TestOpenRefusesNewerSchema(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if _, err := s.db.ExecContext(ctx, "PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(ctx, path); err == nil {
		t.Fatal("Open accepted a database from a newer Daycast")
	}
}

func TestMigrationBacksUpExistingDatabase(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	scripts, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// Two new migrations in one upgrade: each gets its own backup.
	next := append(scripts, "CREATE TABLE extra (x INTEGER) STRICT;", "CREATE TABLE more (y INTEGER) STRICT;")
	if err := s.migrate(ctx, path, next); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	backup, err := Open(ctx, fmt.Sprintf("%s.bak-v%d", path, len(scripts)))
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	if _, err := backup.Display(ctx, "keuken"); err != nil {
		t.Fatalf("backup does not contain the data from before the migration: %v", err)
	}
	// This build cannot open the second copy, which is one version newer
	// than its own migrations; it only has to exist.
	if _, err := os.Stat(fmt.Sprintf("%s.bak-v%d", path, len(scripts)+1)); err != nil {
		t.Fatalf("no backup before the second migration: %v", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != len(next) {
		t.Fatalf("schema version = %d (err %v), want %d", version, err, len(next))
	}
}

func TestFailedMigrationLeavesVersionUnchanged(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	scripts, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	broken := append(scripts, "CREATE TABLE broken (;")
	if err := s.migrate(ctx, path, broken); err == nil {
		t.Fatal("migrate accepted invalid SQL")
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != len(scripts) {
		t.Fatalf("schema version = %d (err %v), want %d", version, err, len(scripts))
	}
}

func TestWriteCopyMakesAUsableDatabase(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if err := s.CreateDisplay(ctx, kitchen()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(path), "copy.db")
	if err := s.WriteCopy(ctx, target); err != nil {
		t.Fatalf("WriteCopy: %v", err)
	}
	if err := s.WriteCopy(ctx, target); err == nil {
		t.Fatal("WriteCopy overwrote an existing file")
	}
	copied, err := Open(ctx, target)
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	defer copied.Close()
	if d, err := copied.Display(ctx, "keuken"); err != nil || d.Name != "Keuken TV" {
		t.Fatalf("display in the copy = %+v, %v", d, err)
	}
}

func TestOpenCreatesTheDataDirectory(t *testing.T) {
	// A first start of the binary uses a data directory that does not
	// exist yet.
	path := filepath.Join(t.TempDir(), "data", "daycast.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open in a missing directory: %v", err)
	}
	s.Close()
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || !info.IsDir() {
		t.Fatalf("data directory = %v, %v", info, err)
	}
}
