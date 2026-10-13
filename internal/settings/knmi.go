package settings

import (
	"context"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// keyKNMI holds the KNMI Data Platform API key, sealed: it is the
// household's own and rate-limited per key.
const keyKNMI = "knmi"

// KNMI is the KNMI connection as the web interface sees it: never the key.
type KNMI struct {
	Connected bool `json:"connected"`
	// Unreadable is true when the stored key cannot be decrypted, for
	// example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

// CheckKNMIKey validates a pasted key before it is tried and returns it
// trimmed.
func CheckKNMIKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if !pastedToken(key) {
		var errs validation.Collector
		errs.Add("key", "paste the EDR API key from the KNMI Developer Portal")
		return "", errs.Err()
	}
	return key, nil
}

// SetKNMIKey stores a key that CheckKNMIKey returned and the caller tried.
func (s *Service) SetKNMIKey(ctx context.Context, key string) error {
	sealed, err := s.Box.Seal(key)
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyKNMI, sealed)
}

// ClearKNMIKey removes the key; measured weather then falls back to the
// weather model.
func (s *Service) ClearKNMIKey(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keyKNMI)
}

// KNMIKey returns the stored key, for the server only; ok is false when
// none is set.
func (s *Service) KNMIKey(ctx context.Context) (key string, ok bool, err error) {
	sealed, ok, err := s.Store.Setting(ctx, keyKNMI)
	if err != nil || !ok {
		return "", false, err
	}
	key, err = s.Box.Open(sealed)
	if err != nil {
		return "", false, fmt.Errorf("decrypt knmi key: %w", err)
	}
	return key, true, nil
}

// KNMIView returns the connection for the Settings page.
func (s *Service) KNMIView(ctx context.Context) (KNMI, error) {
	_, stored, err := s.Store.Setting(ctx, keyKNMI)
	if err != nil || !stored {
		return KNMI{}, err
	}
	if _, _, err := s.KNMIKey(ctx); err != nil {
		s.Logger.Error("knmi key cannot be decrypted; enter it again", "error", err)
		return KNMI{Connected: true, Unreadable: true}, nil
	}
	return KNMI{Connected: true}, nil
}
