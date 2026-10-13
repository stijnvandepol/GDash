package speedcameras

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/traffic"
)

const (
	// MaxAge is how long a fetched list is used before asking again.
	MaxAge = 7 * 24 * time.Hour
	// retryAfterFailure keeps a failing Overpass from being asked more than
	// a few times a day.
	retryAfterFailure = 6 * time.Hour
)

// ErrWaiting is returned while the store waits before asking again after a
// failure.
var ErrWaiting = errors.New("waiting before asking overpass again")

// Summary is what the Settings page shows about the list.
type Summary struct {
	Cameras   int       `json:"cameras"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// cacheFile is the list as it is kept on disk.
type cacheFile struct {
	FetchedAt time.Time        `json:"fetchedAt"`
	Cameras   []traffic.Camera `json:"cameras"`
}

// Store keeps the camera list in memory and in a file, and asks the
// Overpass API only when the list is older than MaxAge, so a restart does
// not cost a query.
type Store struct {
	Client interface {
		Fetch(ctx context.Context) ([]traffic.Camera, error)
	}
	// Path is the file the list is kept in.
	Path   string
	Now    func() time.Time
	Logger *slog.Logger

	mu          sync.Mutex
	loaded      bool
	list        cacheFile
	lastFailure time.Time
}

// Cameras returns the list, and false when there is none yet. It never asks
// the Overpass API.
func (s *Store) Cameras() ([]traffic.Camera, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.list.Cameras, !s.list.FetchedAt.IsZero()
}

// Refresh asks the Overpass API for a new list when the one it has is older
// than MaxAge, and keeps it. It returns the summary of the list it has.
func (s *Store) Refresh(ctx context.Context) (Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	now := s.Now()
	if !s.list.FetchedAt.IsZero() && now.Sub(s.list.FetchedAt) < MaxAge {
		return s.summary(), nil
	}
	if !s.lastFailure.IsZero() && now.Sub(s.lastFailure) < retryAfterFailure {
		return Summary{}, ErrWaiting
	}
	cameras, err := s.Client.Fetch(ctx)
	if err != nil {
		s.lastFailure = now
		return Summary{}, err
	}
	s.lastFailure = time.Time{}
	s.list = cacheFile{FetchedAt: now, Cameras: cameras}
	if err := s.save(); err != nil {
		// The list is in memory; only the restart fallback is lost.
		return s.summary(), fmt.Errorf("keep the camera list on disk: %w", err)
	}
	return s.summary(), nil
}

func (s *Store) summary() Summary {
	return Summary{Cameras: len(s.list.Cameras), FetchedAt: s.list.FetchedAt}
}

// load reads the file once. A missing or unreadable file is no list: the
// next Refresh fetches one.
func (s *Store) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	body, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	var file cacheFile
	if err == nil {
		err = json.Unmarshal(body, &file)
	}
	if err != nil {
		s.Logger.Warn("could not read the kept speed camera list; fetching a new one", "error", err)
		return
	}
	s.list = file
}

// save writes the file through a temporary one, so a crash never leaves
// half a list.
func (s *Store) save() error {
	body, err := json.Marshal(s.list)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".cameras-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path)
}
