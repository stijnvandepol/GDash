package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/immich"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// keyImmich holds the address, API key and chosen album, sealed together:
// the key reads the household's whole photo library.
const keyImmich = "immich"

// Immich is the connection as the web interface sees it: the address and
// the album, never the API key.
type Immich struct {
	URL string `json:"url"`
	// AlbumID and AlbumName are empty until an album is chosen.
	AlbumID   string `json:"albumId"`
	AlbumName string `json:"albumName"`
	// Unreadable is true when the stored connection cannot be decrypted,
	// for example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

// ImmichAlbum is the connection with the album that Photos widgets show,
// for the server only.
type ImmichAlbum struct {
	Connection immich.Connection
	AlbumID    string
}

type sealedImmich struct {
	URL       string `json:"url"`
	Key       string `json:"key"`
	AlbumID   string `json:"albumId,omitempty"`
	AlbumName string `json:"albumName,omitempty"`
}

// CheckImmich validates an address and API key before they are tried, and
// returns the connection with the address normalized.
func CheckImmich(address, key string) (immich.Connection, error) {
	var errs validation.Collector
	base, msg := immich.NormalizeURL(address)
	if msg != "" {
		errs.Add("url", msg)
	}
	key = strings.TrimSpace(key)
	if !pastedToken(key) {
		errs.Add("key", "paste the API key that Immich showed when you made it")
	}
	if err := errs.Err(); err != nil {
		return immich.Connection{}, err
	}
	return immich.Connection{URL: base, Key: key}, nil
}

// SetImmich stores a connection that CheckImmich returned and the caller
// tried. The album stays chosen when the address stays the same.
func (s *Service) SetImmich(ctx context.Context, conn immich.Connection) error {
	next := sealedImmich{URL: conn.URL, Key: conn.Key}
	if old, ok, err := s.immich(ctx); err == nil && ok && old.URL == conn.URL {
		next.AlbumID, next.AlbumName = old.AlbumID, old.AlbumName
	}
	return s.saveImmich(ctx, next)
}

// SetImmichAlbum chooses the album that Photos widgets show; the caller
// checked that the connection reads it. It returns store.ErrNotFound when
// no connection is stored.
func (s *Service) SetImmichAlbum(ctx context.Context, album immich.Album) error {
	current, ok, err := s.immich(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return store.ErrNotFound
	}
	current.AlbumID, current.AlbumName = album.ID, album.Name
	return s.saveImmich(ctx, current)
}

// ClearImmich removes the connection; Photos widgets that show the album
// then say that it is not set up.
func (s *Service) ClearImmich(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keyImmich)
}

// ImmichConnection returns the stored connection with its API key, for the
// server only; ok is false when none is set up.
func (s *Service) ImmichConnection(ctx context.Context) (conn immich.Connection, ok bool, err error) {
	c, ok, err := s.immich(ctx)
	if err != nil || !ok {
		return immich.Connection{}, false, err
	}
	return immich.Connection{URL: c.URL, Key: c.Key}, true, nil
}

// immichAlbum returns the connection with its chosen album, or nil when
// none is set up or no album is chosen.
func (s *Service) immichAlbum(ctx context.Context) (*ImmichAlbum, error) {
	c, ok, err := s.immich(ctx)
	if err != nil || !ok || c.AlbumID == "" {
		return nil, err
	}
	return &ImmichAlbum{Connection: immich.Connection{URL: c.URL, Key: c.Key}, AlbumID: c.AlbumID}, nil
}

// immichView returns the connection for the Settings page, or nil.
func (s *Service) immichView(ctx context.Context) (*Immich, error) {
	_, stored, err := s.Store.Setting(ctx, keyImmich)
	if err != nil || !stored {
		return nil, err
	}
	c, _, err := s.immich(ctx)
	if err != nil {
		s.Logger.Error("immich connection cannot be decrypted; connect again", "error", err)
		return &Immich{Unreadable: true}, nil
	}
	return &Immich{URL: c.URL, AlbumID: c.AlbumID, AlbumName: c.AlbumName}, nil
}

func (s *Service) immich(ctx context.Context) (sealedImmich, bool, error) {
	sealed, ok, err := s.Store.Setting(ctx, keyImmich)
	if err != nil || !ok {
		return sealedImmich{}, false, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return sealedImmich{}, false, fmt.Errorf("decrypt immich connection: %w", err)
	}
	var c sealedImmich
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return sealedImmich{}, false, fmt.Errorf("stored immich connection is invalid: %w", err)
	}
	return c, true, nil
}

func (s *Service) saveImmich(ctx context.Context, c sealedImmich) error {
	value, err := json.Marshal(c)
	if err != nil {
		return err
	}
	sealed, err := s.Box.Seal(string(value))
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyImmich, sealed)
}
