package settings

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/synology"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// keySynology holds the share link, its password and the accepted
// certificate, sealed together: the link alone opens the album.
const keySynology = "synology"

// maxSharePassword is far above what Synology Photos lets a share have.
const maxSharePassword = 256

// Synology is the shared album as the web interface sees it: the NAS's
// address, never the link or the password.
type Synology struct {
	Host        string `json:"host"`
	PasswordSet bool   `json:"passwordSet"`
	// Certificate is the SHA-256 fingerprint of the accepted certificate
	// as hex pairs, or empty when the certificate is checked as browsers
	// do.
	Certificate string `json:"certificate"`
	// Unreadable is true when the stored album cannot be decrypted, for
	// example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

type sealedSynology struct {
	Link     string `json:"link"`
	Password string `json:"password,omitempty"`
	Pin      string `json:"pin,omitempty"`
}

// CheckSynology validates a share link and password before they are
// tried. An empty password keeps the stored one when the link stays the
// same, so the form never needs to show it.
func (s *Service) CheckSynology(ctx context.Context, link, password string) (synology.Album, error) {
	var errs validation.Collector
	normalized, msg := synology.ParseLink(link)
	if msg != "" {
		errs.Add("link", msg)
	}
	if len(password) > maxSharePassword || validation.HasControlCharacter(password) {
		errs.Add("password", "enter the password of the share link, or leave it empty")
	}
	if err := errs.Err(); err != nil {
		return synology.Album{}, err
	}
	album := synology.Album{Link: normalized, Password: password}
	if password == "" {
		if old, ok, err := s.SynologyAlbum(ctx); err == nil && ok && old.Link == normalized {
			album.Password = old.Password
		}
	}
	return album, nil
}

// SetSynology stores an album that CheckSynology returned and the caller
// tried.
func (s *Service) SetSynology(ctx context.Context, album synology.Album) error {
	stored := sealedSynology{Link: album.Link, Password: album.Password}
	if album.Pin != (outbound.Fingerprint{}) {
		stored.Pin = hex.EncodeToString(album.Pin[:])
	}
	value, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	sealed, err := s.Box.Seal(string(value))
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keySynology, sealed)
}

// ClearSynology removes the album; Photos widgets that show it then say
// that it is not set up.
func (s *Service) ClearSynology(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keySynology)
}

// SynologyAlbum returns the stored album with its secrets, for the server
// only; ok is false when none is set up.
func (s *Service) SynologyAlbum(ctx context.Context) (album synology.Album, ok bool, err error) {
	sealed, ok, err := s.Store.Setting(ctx, keySynology)
	if err != nil || !ok {
		return synology.Album{}, false, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return synology.Album{}, false, fmt.Errorf("decrypt synology album: %w", err)
	}
	var stored sealedSynology
	if err := json.Unmarshal([]byte(plain), &stored); err != nil {
		return synology.Album{}, false, fmt.Errorf("stored synology album is invalid: %w", err)
	}
	album = synology.Album{Link: stored.Link, Password: stored.Password}
	if stored.Pin != "" {
		pin, err := hex.DecodeString(stored.Pin)
		if err != nil || len(pin) != len(album.Pin) {
			return synology.Album{}, false, fmt.Errorf("stored synology certificate is invalid")
		}
		copy(album.Pin[:], pin)
	}
	return album, true, nil
}

// synologyView returns the album for the Settings page, or nil.
func (s *Service) synologyView(ctx context.Context) (*Synology, error) {
	_, stored, err := s.Store.Setting(ctx, keySynology)
	if err != nil || !stored {
		return nil, err
	}
	album, _, err := s.SynologyAlbum(ctx)
	if err != nil {
		s.Logger.Error("synology album cannot be decrypted; connect again", "error", err)
		return &Synology{Unreadable: true}, nil
	}
	return SynologyView(album), nil
}

// SynologyView is what the web interface may see of an album.
func SynologyView(album synology.Album) *Synology {
	v := &Synology{Host: synology.Host(album.Link), PasswordSet: album.Password != ""}
	if album.Pin != (outbound.Fingerprint{}) {
		pairs := make([]string, len(album.Pin))
		for i, b := range album.Pin {
			pairs[i] = fmt.Sprintf("%02X", b)
		}
		v.Certificate = strings.Join(pairs, ":")
	}
	return v
}
