package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/store"
)

// CustomSource is a custom API source as the web interface sees it:
// everything but the secrets, which only say whether they are set. The
// sources themselves, their checks and fetching are in internal/custom;
// this file stores them, with the secrets sealed.
type CustomSource struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`
	Auth   struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		SecretSet bool   `json:"secretSet"`
	} `json:"auth"`
	// Headers carry an empty value when they are secret.
	Headers         []custom.Header `json:"headers"`
	IntervalSeconds int             `json:"intervalSeconds"`
	Sample          json.RawMessage `json:"sample"`
	// Unreadable is true when the secrets cannot be decrypted, for example
	// after the database was moved without its secret.key.
	Unreadable bool      `json:"unreadable"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// sealedCustomSecrets is what secrets_sealed holds.
type sealedCustomSecrets struct {
	Auth    string            `json:"auth,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// CustomSources returns every custom API source for the web interface.
func (s *Service) CustomSources(ctx context.Context) ([]CustomSource, error) {
	rows, err := s.Store.CustomSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CustomSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.customView(row))
	}
	return out, nil
}

// CustomSourceWithSecrets returns one source with its secrets, for
// fetching it; it returns store.ErrNotFound for an unknown ID.
func (s *Service) CustomSourceWithSecrets(ctx context.Context, id int64) (custom.Source, error) {
	row, err := s.Store.CustomSource(ctx, id)
	if err != nil {
		return custom.Source{}, err
	}
	return s.openCustom(row)
}

// FetchableCustomSources returns every source with its secrets. A source
// whose secrets cannot be decrypted is left out and logged, so the others
// keep working.
func (s *Service) FetchableCustomSources(ctx context.Context) ([]custom.Source, error) {
	rows, err := s.Store.CustomSources(ctx)
	if err != nil {
		return nil, err
	}
	var out []custom.Source
	for _, row := range rows {
		src, err := s.openCustom(row)
		if err != nil {
			s.Logger.Error("custom API secrets cannot be decrypted; enter them again", "source", row.Name, "error", err)
			continue
		}
		out = append(out, src)
	}
	return out, nil
}

// AddCustomSource stores a new source. It returns a *validation.Error for
// invalid input and store.ErrConflict when custom.MaxSources exist.
func (s *Service) AddCustomSource(ctx context.Context, in custom.Input) (CustomSource, error) {
	src, err := custom.Check(in, nil)
	if err != nil {
		return CustomSource{}, err
	}
	row, err := s.customRow(src)
	if err != nil {
		return CustomSource{}, err
	}
	row.CreatedAt = row.UpdatedAt
	if row.ID, err = s.Store.AddCustomSource(ctx, row, custom.MaxSources); err != nil {
		return CustomSource{}, err
	}
	return s.customView(row), nil
}

// UpdateCustomSource changes a source; empty secrets keep the stored ones.
// It returns store.ErrNotFound for an unknown ID.
func (s *Service) UpdateCustomSource(ctx context.Context, id int64, in custom.Input) (CustomSource, error) {
	stored, err := s.Store.CustomSource(ctx, id)
	if err != nil {
		return CustomSource{}, err
	}
	old, err := s.openCustom(stored)
	if err != nil {
		// The secrets are lost with the key; the admin enters them again.
		old = custom.Source{ID: id}
	}
	src, err := custom.Check(in, &old)
	if err != nil {
		return CustomSource{}, err
	}
	row, err := s.customRow(src)
	if err != nil {
		return CustomSource{}, err
	}
	row.CreatedAt = stored.CreatedAt
	if err := s.Store.UpdateCustomSource(ctx, row); err != nil {
		return CustomSource{}, err
	}
	return s.customView(row), nil
}

// DeleteCustomSource removes a source, or returns store.ErrNotFound.
// Widgets that show it then say that their source is gone.
func (s *Service) DeleteCustomSource(ctx context.Context, id int64) error {
	return s.Store.DeleteCustomSource(ctx, id)
}

// customRow seals the secrets of src for storing.
func (s *Service) customRow(src custom.Source) (store.CustomSource, error) {
	secrets := sealedCustomSecrets{Auth: src.AuthSecret, Headers: map[string]string{}}
	shown := make([]custom.Header, len(src.Headers))
	for i, h := range src.Headers {
		shown[i] = h
		if h.Secret {
			secrets.Headers[strings.ToLower(h.Name)] = h.Value
			shown[i].Value = ""
		}
	}
	headers, err := json.Marshal(shown)
	if err != nil {
		return store.CustomSource{}, err
	}
	sealed := ""
	if secrets.Auth != "" || len(secrets.Headers) > 0 {
		plain, err := json.Marshal(secrets)
		if err != nil {
			return store.CustomSource{}, err
		}
		if sealed, err = s.Box.Seal(string(plain)); err != nil {
			return store.CustomSource{}, err
		}
	}
	return store.CustomSource{
		ID: src.ID, Name: src.Name, URL: src.URL, Method: src.Method, Body: src.Body, Auth: src.Auth,
		AuthName: src.AuthName, Headers: string(headers), SecretsSealed: sealed,
		IntervalSeconds: int(src.Interval / time.Second), Sample: src.Sample, UpdatedAt: s.Now().UTC(),
	}, nil
}

// openCustom decrypts a stored source.
func (s *Service) openCustom(row store.CustomSource) (custom.Source, error) {
	src := custom.Source{
		ID: row.ID, Name: row.Name, URL: row.URL, Method: row.Method, Body: row.Body, Auth: row.Auth,
		AuthName: row.AuthName, Interval: time.Duration(row.IntervalSeconds) * time.Second, Sample: row.Sample,
	}
	if err := json.Unmarshal([]byte(row.Headers), &src.Headers); err != nil {
		return custom.Source{}, fmt.Errorf("stored headers of custom source %d are invalid: %w", row.ID, err)
	}
	if row.SecretsSealed == "" {
		return src, nil
	}
	plain, err := s.Box.Open(row.SecretsSealed)
	if err != nil {
		return custom.Source{}, err
	}
	var secrets sealedCustomSecrets
	if err := json.Unmarshal([]byte(plain), &secrets); err != nil {
		return custom.Source{}, errors.New("stored secrets are invalid")
	}
	src.AuthSecret = secrets.Auth
	for i, h := range src.Headers {
		if h.Secret {
			src.Headers[i].Value = secrets.Headers[strings.ToLower(h.Name)]
		}
	}
	return src, nil
}

func (s *Service) customView(row store.CustomSource) CustomSource {
	v := CustomSource{
		ID: row.ID, Name: row.Name, URL: row.URL, Method: row.Method, Body: row.Body,
		IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Headers: []custom.Header{},
	}
	v.Auth.Kind, v.Auth.Name = row.Auth, row.AuthName
	if row.Sample != "" {
		v.Sample = json.RawMessage(row.Sample)
	}
	if err := json.Unmarshal([]byte(row.Headers), &v.Headers); err != nil {
		v.Unreadable = true
	}
	if row.SecretsSealed != "" {
		_, err := s.Box.Open(row.SecretsSealed)
		v.Unreadable = v.Unreadable || err != nil
		// Header secrets are listed by their header; only the auth one is
		// a separate flag.
		v.Auth.SecretSet = row.Auth != custom.AuthNone && err == nil
	}
	return v
}
