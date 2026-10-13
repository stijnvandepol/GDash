package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// castSessionsFile keeps, per display, the Cast session GDash launched, so a
// keeper that starts after an update recognises its own dashboard on the TV
// instead of waiting for someone it thinks is using it (cast.Keeper.Recall).
const castSessionsFile = "cast-sessions.json"

// castSessions is the file in the data directory, read once at start.
type castSessions struct {
	path   string
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]string
}

// loadCastSessions reads the file; a missing or unreadable one starts empty,
// which only means a keeper waits once more as it did before.
func loadCastSessions(dataDir string, logger *slog.Logger) *castSessions {
	s := &castSessions{path: filepath.Join(dataDir, castSessionsFile), logger: logger, sessions: map[string]string{}}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s
	}
	if err == nil {
		err = json.Unmarshal(data, &s.sessions)
	}
	if err != nil {
		logger.Warn("could not read the Cast sessions; TVs found casting after this start wait until switched off", "error", err)
		s.sessions = map[string]string{}
	}
	return s
}

func (s *castSessions) recall(displayID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[displayID]
}

// remember stores a change for displayID and writes the file through a
// temporary file, so a crash never leaves half of it behind.
func (s *castSessions) remember(displayID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID == "" {
		delete(s.sessions, displayID)
	} else {
		s.sessions[displayID] = sessionID
	}
	if err := s.write(); err != nil {
		s.logger.Warn("could not store the Cast session", "display", displayID, "error", err)
	}
}

func (s *castSessions) write() error {
	data, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), castSessionsFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
