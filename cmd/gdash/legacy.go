package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// databaseName is the configuration database in the data directory.
const databaseName = "gdash.db"

// oldDatabaseNames are the databases of earlier names of the project:
// Daycast (the 0.5.0 release) and Nookboard, newest first.
var oldDatabaseNames = []string{"nookboard.db", "daycast.db"}

// adoptOldDatabase renames the database of an earlier name in dataDir to
// the current name, with SQLite's journal files, so an update keeps the
// configuration. It does nothing when the current database exists or there
// is no old one, and returns the name it adopted.
func adoptOldDatabase(dataDir string) (adopted string, err error) {
	current := filepath.Join(dataDir, databaseName)
	if _, err := os.Stat(current); !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	for _, name := range oldDatabaseNames {
		old := filepath.Join(dataDir, name)
		if _, err := os.Stat(old); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		// The journal files go first: an interrupted rename then leaves
		// the old database in place to retry, never a database without its
		// journal.
		for _, suffix := range []string{"-wal", "-shm", ""} {
			err := os.Rename(old+suffix, current+suffix)
			if err != nil && !(suffix != "" && errors.Is(err, fs.ErrNotExist)) {
				return "", fmt.Errorf("rename %s to %s: %w", name+suffix, databaseName+suffix, err)
			}
		}
		return name, nil
	}
	return "", nil
}
