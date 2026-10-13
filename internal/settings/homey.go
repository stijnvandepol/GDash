package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// keyHomey holds how Homey is set up, sealed: the address and API key of a
// local connection, as the key reads every device and who is home, or that
// the Homey sends its data itself.
const keyHomey = "homey"

// How GDash gets Homey's data.
const (
	// HomeyLocal asks a Homey Pro (2023) or Homey Self-Hosted Server with
	// an API key.
	HomeyLocal = "local"
	// HomeyPush waits for a snapshot that a HomeyScript sends, for a Homey
	// without API keys, such as a Homey Pro (Early 2019).
	HomeyPush = "push"
)

// Homey is the setup as the web interface sees it: the mode and the
// address, never the API key.
type Homey struct {
	Mode string `json:"mode"`
	// URL is empty when the Homey sends its data.
	URL string `json:"url"`
	// Unreadable is true when the stored connection cannot be decrypted,
	// for example after the database was moved without its secret.key.
	Unreadable bool `json:"unreadable"`
}

type sealedHomey struct {
	// Mode is empty for a local connection, as before push existed.
	Mode string `json:"mode,omitempty"`
	URL  string `json:"url,omitempty"`
	Key  string `json:"key,omitempty"`
}

// HomeySetup is how Homey is set up: Pushed, or else Connection.
type HomeySetup struct {
	Pushed     bool
	Connection homey.Connection
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
	return s.storeHomey(ctx, sealedHomey{URL: conn.URL, Key: conn.Key})
}

// SetHomeyPush makes GDash wait for the snapshots a HomeyScript sends; a
// local connection and its key are dropped.
func (s *Service) SetHomeyPush(ctx context.Context) error {
	return s.storeHomey(ctx, sealedHomey{Mode: HomeyPush})
}

func (s *Service) storeHomey(ctx context.Context, setup sealedHomey) error {
	value, err := json.Marshal(setup)
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

// HomeySetup returns how Homey is set up, with the API key of a local
// connection, for the server only; ok is false when it is not set up.
func (s *Service) HomeySetup(ctx context.Context) (setup HomeySetup, ok bool, err error) {
	sealed, ok, err := s.Store.Setting(ctx, keyHomey)
	if err != nil || !ok {
		return HomeySetup{}, false, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return HomeySetup{}, false, fmt.Errorf("decrypt homey connection: %w", err)
	}
	var c sealedHomey
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return HomeySetup{}, false, fmt.Errorf("stored homey connection is invalid: %w", err)
	}
	if c.Mode == HomeyPush {
		return HomeySetup{Pushed: true}, true, nil
	}
	return HomeySetup{Connection: homey.Connection{URL: c.URL, Key: c.Key}}, true, nil
}

// homeyView returns the connection for the Settings page, or nil.
func (s *Service) homeyView(ctx context.Context) (*Homey, error) {
	_, stored, err := s.Store.Setting(ctx, keyHomey)
	if err != nil || !stored {
		return nil, err
	}
	setup, _, err := s.HomeySetup(ctx)
	if err != nil {
		s.Logger.Error("homey connection cannot be decrypted; connect again", "error", err)
		return &Homey{Mode: HomeyLocal, Unreadable: true}, nil
	}
	if setup.Pushed {
		return &Homey{Mode: HomeyPush}, nil
	}
	return &Homey{Mode: HomeyLocal, URL: setup.Connection.URL}, nil
}
