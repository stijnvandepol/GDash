package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stijnvandepol/nookboard/internal/homeassistant"
	"github.com/stijnvandepol/nookboard/internal/validation"
)

const (
	// keyHomeAssistant holds the address and token, sealed together: the
	// token gives read access to the whole home.
	keyHomeAssistant = "homeassistant"
	maxTokenLength   = 1024
)

// HomeAssistant is the connection as the web interface sees it: the
// address, never the token.
type HomeAssistant struct {
	URL string `json:"url"`
	// Unreadable is true when the stored connection cannot be decrypted,
	// for example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

type sealedConnection struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// CheckHomeAssistant validates an address and token before they are tried,
// and returns the connection with the address normalized.
func CheckHomeAssistant(address, token string) (homeassistant.Connection, error) {
	var errs validation.Collector
	base, msg := homeassistant.NormalizeURL(address)
	if msg != "" {
		errs.Add("url", msg)
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxTokenLength || strings.ContainsAny(token, " \t\r\n") {
		errs.Add("token", "paste a long-lived access token from your Home Assistant profile")
	}
	if err := errs.Err(); err != nil {
		return homeassistant.Connection{}, err
	}
	return homeassistant.Connection{URL: base, Token: token}, nil
}

// SetHomeAssistant stores a connection that CheckHomeAssistant returned
// and the caller tried.
func (s *Service) SetHomeAssistant(ctx context.Context, conn homeassistant.Connection) error {
	value, err := json.Marshal(sealedConnection{URL: conn.URL, Token: conn.Token})
	if err != nil {
		return err
	}
	sealed, err := s.Box.Seal(string(value))
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyHomeAssistant, sealed)
}

// ClearHomeAssistant removes the connection; Home Assistant widgets then
// show that none is set up.
func (s *Service) ClearHomeAssistant(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keyHomeAssistant)
}

// HomeAssistantConnection returns the stored connection with its token,
// for the server only; ok is false when none is set up.
func (s *Service) HomeAssistantConnection(ctx context.Context) (conn homeassistant.Connection, ok bool, err error) {
	sealed, ok, err := s.Store.Setting(ctx, keyHomeAssistant)
	if err != nil || !ok {
		return homeassistant.Connection{}, false, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return homeassistant.Connection{}, false, fmt.Errorf("decrypt home assistant connection: %w", err)
	}
	var c sealedConnection
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return homeassistant.Connection{}, false, fmt.Errorf("stored home assistant connection is invalid: %w", err)
	}
	return homeassistant.Connection{URL: c.URL, Token: c.Token}, true, nil
}

// homeAssistantView returns the connection for the Settings page, or nil.
func (s *Service) homeAssistantView(ctx context.Context) (*HomeAssistant, error) {
	_, stored, err := s.Store.Setting(ctx, keyHomeAssistant)
	if err != nil || !stored {
		return nil, err
	}
	conn, _, err := s.HomeAssistantConnection(ctx)
	if err != nil {
		s.Logger.Error("home assistant connection cannot be decrypted; connect again", "error", err)
		return &HomeAssistant{Unreadable: true}, nil
	}
	return &HomeAssistant{URL: conn.URL}, nil
}
