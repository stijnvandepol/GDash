package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stijnvandepol/gdash/internal/schedule"
)

func TestActivePlanMigrationKeepsTheDailyWindow(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gdash.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// A database from before the active plan: one display with hours, one
	// that is always on.
	old := &Store{db: db}
	if err := old.migrate(ctx, path, scripts[:28]); err != nil {
		t.Fatal(err)
	}
	for _, d := range [][3]string{{"kitchen", "06:30", "23:00"}, {"hall", "", ""}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO displays (id, name, created_at, active_from, active_until) VALUES (?, ?, '2026-01-01T00:00:00Z', ?, ?)`,
			d[0], d[0], d[1], d[2]); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	kitchen, err := s.Display(ctx, "kitchen")
	if err != nil || !reflect.DeepEqual(kitchen.Active, schedule.Plan{{Days: []string{}, From: "06:30", Until: "23:00"}}) {
		t.Fatalf("kitchen = %+v, %v; want one block on every day", kitchen.Active, err)
	}
	hall, err := s.Display(ctx, "hall")
	if err != nil || len(hall.Active) != 0 {
		t.Fatalf("hall = %+v, %v; want always on", hall.Active, err)
	}
}
