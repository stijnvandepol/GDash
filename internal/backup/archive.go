package backup

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Archive is what a backup to download holds: everything needed to run
// GDash elsewhere with the same configuration.
type Archive struct {
	// DataDir is the data directory; the secret key and the photos are read
	// from it.
	DataDir string
	// DatabaseName is the database's file name inside the archive.
	DatabaseName string
	// Write writes a consistent copy of the database to a path that does
	// not exist yet, as for Daily.
	Write func(ctx context.Context, path string) error
}

// secretKeyName and photosDir are the names GDash uses in its data
// directory (cmd/gdash).
const (
	secretKeyName = "secret.key"
	photosDir     = "photos"
)

// WriteZip writes the archive to w as a zip file: a fresh copy of the
// database, the secret key that unlocks the calendar addresses and the Home
// Assistant token in it, and the photos. Unzipped into an empty data
// directory, it is a working GDash again.
func (a Archive) WriteZip(ctx context.Context, w io.Writer) error {
	tmp, err := os.MkdirTemp(a.DataDir, ".backup-*")
	if err != nil {
		return fmt.Errorf("make room for the database copy: %w", err)
	}
	defer os.RemoveAll(tmp)
	copyPath := filepath.Join(tmp, a.DatabaseName)
	if err := a.Write(ctx, copyPath); err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	if err := addFile(zw, a.DatabaseName, copyPath); err != nil {
		return err
	}
	if err := addFile(zw, secretKeyName, filepath.Join(a.DataDir, secretKeyName)); err != nil {
		return err
	}
	photos, err := os.ReadDir(filepath.Join(a.DataDir, photosDir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("list photos: %w", err)
	}
	for _, p := range photos {
		if p.Type().IsRegular() {
			if err := addFile(zw, photosDir+"/"+p.Name(), filepath.Join(a.DataDir, photosDir, p.Name())); err != nil {
				return err
			}
		}
	}
	return zw.Close()
}

// addFile copies the file at path into the archive under name.
func addFile(zw *zip.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read %s for the backup: %w", name, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	dst, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, f); err != nil {
		return fmt.Errorf("write %s to the backup: %w", name, err)
	}
	return nil
}
