package datahub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// validKey keeps source keys usable as file names on every platform.
var validKey = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)

// FileStore keeps the last good entry of each source as a JSON file, so data
// survives a restart. SQLite replaces this once Daycast stores configuration.
type FileStore struct {
	dir string
}

// NewFileStore creates dir if needed.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fmt.Errorf("invalid source key %q", key)
	}
	return filepath.Join(s.dir, key+".json"), nil
}

// cached is the file format: an entry and the configuration it was
// fetched with.
type cached struct {
	Version string `json:"version"`
	Entry   Entry  `json:"entry"`
}

// Load returns the stored entry for key if it was fetched with the given
// source version. ok is false when nothing is stored, or when it belongs to
// another configuration, such as a calendar that has been removed since.
func (s *FileStore) Load(key, version string) (entry Entry, ok bool, err error) {
	path, err := s.path(key)
	if err != nil {
		return Entry{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("read cached %s: %w", key, err)
	}
	var c cached
	if err := json.Unmarshal(data, &c); err != nil {
		return Entry{}, false, fmt.Errorf("decode cached %s: %w", key, err)
	}
	if c.Version != version {
		return Entry{}, false, nil
	}
	return c.Entry, true, nil
}

// Save stores entry for key, fetched with the given source version. It writes a temporary file and renames it, so a
// crash halfway never leaves a corrupt cache behind.
func (s *FileStore) Save(key, version string, entry Entry) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	data, err := json.Marshal(cached{Version: version, Entry: entry})
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	tmp, err := os.CreateTemp(s.dir, key+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", key, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", key, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}
	return nil
}

// Delete removes the stored entry for key; a missing entry is fine.
func (s *FileStore) Delete(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}
