package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestArchiveHoldsDatabaseKeyAndPhotos(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), []byte("key-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "photos"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photos", "a.jpg"), []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := Archive{
		DataDir: dir, DatabaseName: "gdash.db",
		Write: func(_ context.Context, path string) error { return os.WriteFile(path, []byte("database"), 0o600) },
	}
	var buf bytes.Buffer
	if err := archive.WriteZip(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	want := map[string]string{"gdash.db": "database", "secret.key": "key-bytes", "photos/a.jpg": "jpeg"}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s = %q, want %q", name, got[name], content)
		}
	}
	entries, _ := os.ReadDir(dir)
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"photos", "secret.key"}) {
		t.Errorf("data directory after the backup = %v, want the temporary copy gone", names)
	}
}

func TestArchiveWithoutPhotos(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := Archive{DataDir: dir, DatabaseName: "gdash.db", Write: func(_ context.Context, path string) error {
		return os.WriteFile(path, nil, 0o600)
	}}
	if err := archive.WriteZip(context.Background(), io.Discard); err != nil {
		t.Fatalf("a data directory without photos: %v", err)
	}
}
