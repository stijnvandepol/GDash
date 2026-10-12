package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// keyHomey holds the address and API key, sealed together: the key reads
// every device and who is home.
const keyHomey = "homey"

// Homey is the connection as the web interface sees it: the address,
// never the API key.
type Homey struct {
	URL string `json:"url"`
	// Unreadable is true when the stored connection cannot be decrypted,
	// for example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

type sealedHomey struct {
	URL string `json:"url"`
	Key string `json:"key"`
}

// CheckHomey validates an address and API key before they are tried, and
// returns the connection with the address normalized.
func CheckHomey(address, key string) (homey.Connection, error) {
	var errs validation.Collector
	base, msg := homey.NormalizeURL(address)
	if msg != "" {
		errs.Add("url", msg)
	}
	key = strings.TrimSpace(key)
	if !pastedToken(key) {
		errs.Add("key", "paste the API key that the Homey web app showed when you made it")
	}
	if err := errs.Err(); err != nil {
		return homey.Connection{}, err
	}
	return homey.Connection{URL: base, Key: key}, nil
}

// SetHomey stores a connection that CheckHomey returned and the caller
// tried.
func (s *Service) SetHomey(ctx context.Context, conn homey.Connection) error {
	value, err := json.Marshal(sealedHomey{URL: conn.URL, Key: conn.Key})
	if err != nil {
		return err
	}
	sealed, err := s.Box.Seal(string(value))
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyHomey, sealed)
}

// ClearHomey removes the connection; Homey widgets then show that none is
// set up.
func (s *Service) ClearHomey(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keyHomey)
}

// HomeyConnection returns the stored connection with its API key, for the
// server only; ok is false when none is set up.
func (s *Service) HomeyConnection(ctx context.Context) (conn homey.Connection, ok bool, err error) {
	sealed, ok, err := s.Store.Setting(ctx, keyHomey)
	if err != nil || !ok {
		return homey.Connection{}, false, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return homey.Connection{}, false, fmt.Errorf("decrypt homey connection: %w", err)
	}
	var c sealedHomey
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return homey.Connection{}, false, fmt.Errorf("stored homey connection is invalid: %w", err)
	}
	return homey.Connection{URL: c.URL, Key: c.Key}, true, nil
}

// homeyView returns the connection for the Settings page, or nil.
func (s *Service) homeyView(ctx context.Context) (*Homey, error) {
	_, stored, err := s.Store.Setting(ctx, keyHomey)
	if err != nil || !stored {
		return nil, err
	}
	conn, _, err := s.HomeyConnection(ctx)
	if err != nil {
		s.Logger.Error("homey connection cannot be decrypted; connect again", "error", err)
		return &Homey{Unreadable: true}, nil
	}
	return &Homey{URL: conn.URL}, nil
}
