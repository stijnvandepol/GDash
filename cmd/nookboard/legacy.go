package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// databaseName is the configuration database in the data directory.
const databaseName = "nookboard.db"

// oldDatabaseName is the database of the releases called Daycast, before
// the project was renamed to Nookboard.
const oldDatabaseName = "daycast.db"

// adoptOldDatabase renames a Daycast database in dataDir to the current
// name, with SQLite's journal files, so an update keeps the configuration.
// It does nothing when the current database exists or there is no old one.
func adoptOldDatabase(dataDir string) (adopted bool, err error) {
	current := filepath.Join(dataDir, databaseName)
	if _, err := os.Stat(current); !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	old := filepath.Join(dataDir, oldDatabaseName)
	if _, err := os.Stat(old); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// The journal files go first: an interrupted rename then leaves the
	// old database in place to retry, never a database without its journal.
	for _, suffix := range []string{"-wal", "-shm", ""} {
		err := os.Rename(old+suffix, current+suffix)
		if err != nil && !(suffix != "" && errors.Is(err, fs.ErrNotExist)) {
			return false, fmt.Errorf("rename %s to %s: %w", oldDatabaseName+suffix, databaseName+suffix, err)
		}
	}
	return true, nil
}
