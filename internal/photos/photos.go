// Package photos keeps the household's photos for the photo widget, in a
// folder in the data directory. Photos are private: internal/api only
// serves them to admins and paired screens.
package photos

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// MaxBytes keeps one photo well below what a phone camera makes at
	// most, and a Chromecast can decode it without running out of memory.
	MaxBytes = 8 << 20
	// MaxPhotos bounds the folder; a slideshow on a kitchen TV needs no
	// more.
	MaxPhotos = 50
	// sniffBytes is what http.DetectContentType reads.
	sniffBytes = 512
)

var (
	// ErrNotAPhoto is returned for a file that is not JPEG, PNG or WebP.
	ErrNotAPhoto = errors.New("the file is not a JPEG, PNG or WebP photo")
	// ErrTooLarge is returned for a photo over MaxBytes.
	ErrTooLarge = fmt.Errorf("the photo is larger than %d MB", MaxBytes>>20)
	// ErrFull is returned when MaxPhotos are stored.
	ErrFull = fmt.Errorf("there are already %d photos; remove one first", MaxPhotos)
	// ErrNotFound is returned for a name that is not a stored photo.
	ErrNotFound = errors.New("no such photo")
)

// extensions maps the detected type to the file extension Daycast uses.
var extensions = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

// validName matches the names Add gives photos, so a name from a request
// can never point outside the folder.
var validName = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp)$`)

// Photo is a stored photo.
type Photo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	AddedAt time.Time `json:"addedAt"`
}

// Store is the photo folder.
type Store struct {
	dir string
	// mu makes the count check and the write in Add one step.
	mu sync.Mutex
}

// NewStore uses dir, creating it if needed, readable by Daycast only.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create photo folder: %w", err)
	}
	return &Store{dir: dir}, nil
}

// List returns the photos, oldest first, so a slideshow keeps its order
// when photos are added.
func (s *Store) List() ([]Photo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read photo folder: %w", err)
	}
	photos := []Photo{}
	for _, e := range entries {
		if e.IsDir() || !validName.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		photos = append(photos, Photo{Name: e.Name(), Size: info.Size(), AddedAt: info.ModTime().UTC()})
	}
	slices.SortStableFunc(photos, func(a, b Photo) int { return a.AddedAt.Compare(b.AddedAt) })
	return photos, nil
}

// Add stores the photo read from r under a new random name and returns it.
// The type is decided by the content, not by a file name or header.
func (s *Store) Add(r io.Reader) (Photo, error) {
	head := make([]byte, sniffBytes)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		if errors.Is(err, io.EOF) {
			return Photo{}, ErrNotAPhoto
		}
		return Photo{}, fmt.Errorf("read photo: %w", err)
	}
	head = head[:n]
	ext, ok := extensions[http.DetectContentType(head)]
	if !ok {
		return Photo{}, ErrNotAPhoto
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.List()
	if err != nil {
		return Photo{}, err
	}
	if len(current) >= MaxPhotos {
		return Photo{}, ErrFull
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return Photo{}, err
	}
	name := hex.EncodeToString(id) + ext
	return s.write(name, io.MultiReader(bytes.NewReader(head), r))
}

// write copies at most MaxBytes to the photo, through a temporary file so
// a failed or oversized upload never leaves a photo behind.
func (s *Store) write(name string, r io.Reader) (Photo, error) {
	tmp, err := os.CreateTemp(s.dir, "upload-*.tmp")
	if err != nil {
		return Photo{}, fmt.Errorf("store photo: %w", err)
	}
	defer os.Remove(tmp.Name())
	written, err := io.Copy(tmp, io.LimitReader(r, MaxBytes+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Photo{}, fmt.Errorf("store photo: %w", err)
	}
	if written > MaxBytes {
		return Photo{}, ErrTooLarge
	}
	path := filepath.Join(s.dir, name)
	if err := os.Rename(tmp.Name(), path); err != nil {
		return Photo{}, fmt.Errorf("store photo: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Photo{}, err
	}
	return Photo{Name: name, Size: written, AddedAt: info.ModTime().UTC()}, nil
}

// Open returns the photo's file and content type, or ErrNotFound.
func (s *Store) Open(name string) (*os.File, string, error) {
	if !validName.MatchString(name) {
		return nil, "", ErrNotFound
	}
	f, err := os.Open(filepath.Join(s.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return f, contentType(name), nil
}

// Delete removes a photo, or returns ErrNotFound.
func (s *Store) Delete(name string) error {
	if !validName.MatchString(name) {
		return ErrNotFound
	}
	err := os.Remove(filepath.Join(s.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func contentType(name string) string {
	for typ, ext := range extensions {
		if strings.HasSuffix(name, ext) {
			return typ
		}
	}
	return "application/octet-stream"
}
