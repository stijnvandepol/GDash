package custom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
)

// SourceView is a source as the web interface sees it: everything but the
// secrets, which only say whether they are set.
type SourceView struct {
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
	Headers         []Header        `json:"headers"`
	IntervalSeconds int             `json:"intervalSeconds"`
	Sample          json.RawMessage `json:"sample"`
	// Unreadable is true when the secrets cannot be decrypted, for example
	// after the database was moved without its secret.key.
	Unreadable bool      `json:"unreadable"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// sealedSecrets is what secrets_sealed holds.
type sealedSecrets struct {
	Auth    string            `json:"auth,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Service reads and changes the custom sources.
type Service struct {
	Store  *store.Store
	Box    *secret.Box
	Now    func() time.Time
	Logger *slog.Logger
}

// List returns every source for the web interface.
func (s *Service) List(ctx context.Context) ([]SourceView, error) {
	rows, err := s.Store.CustomSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SourceView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.view(row))
	}
	return out, nil
}

// Source returns one source with its secrets, for fetching it; it returns
// store.ErrNotFound for an unknown ID.
func (s *Service) Source(ctx context.Context, id int64) (Source, error) {
	row, err := s.Store.CustomSource(ctx, id)
	if err != nil {
		return Source{}, err
	}
	return s.open(row)
}

// Sources returns every source that can be fetched. A source whose secrets
// cannot be decrypted is left out and logged, so the others keep working.
func (s *Service) Sources(ctx context.Context) ([]Source, error) {
	rows, err := s.Store.CustomSources(ctx)
	if err != nil {
		return nil, err
	}
	var out []Source
	for _, row := range rows {
		src, err := s.open(row)
		if err != nil {
			s.Logger.Error("custom API secrets cannot be decrypted; enter them again", "source", row.Name, "error", err)
			continue
		}
		out = append(out, src)
	}
	return out, nil
}

// Add stores a new source. It returns a *validation.Error for invalid input
// and store.ErrConflict when MaxSources exist.
func (s *Service) Add(ctx context.Context, in Input) (SourceView, error) {
	src, err := Check(in, nil)
	if err != nil {
		return SourceView{}, err
	}
	row, err := s.row(src)
	if err != nil {
		return SourceView{}, err
	}
	row.CreatedAt = row.UpdatedAt
	if row.ID, err = s.Store.AddCustomSource(ctx, row, MaxSources); err != nil {
		return SourceView{}, err
	}
	return s.view(row), nil
}

// Update changes a source; empty secrets keep the stored ones. It returns
// store.ErrNotFound for an unknown ID.
func (s *Service) Update(ctx context.Context, id int64, in Input) (SourceView, error) {
	stored, err := s.Store.CustomSource(ctx, id)
	if err != nil {
		return SourceView{}, err
	}
	old, err := s.open(stored)
	if err != nil {
		// The secrets are lost with the key; the admin enters them again.
		old = Source{ID: id}
	}
	src, err := Check(in, &old)
	if err != nil {
		return SourceView{}, err
	}
	row, err := s.row(src)
	if err != nil {
		return SourceView{}, err
	}
	row.CreatedAt = stored.CreatedAt
	if err := s.Store.UpdateCustomSource(ctx, row); err != nil {
		return SourceView{}, err
	}
	return s.view(row), nil
}

// Delete removes a source, or returns store.ErrNotFound. Widgets that show
// it then say that their source is gone.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.Store.DeleteCustomSource(ctx, id)
}

// row seals the secrets of src for storing.
func (s *Service) row(src Source) (store.CustomSource, error) {
	secrets := sealedSecrets{Auth: src.AuthSecret, Headers: map[string]string{}}
	shown := make([]Header, len(src.Headers))
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

// open decrypts a stored source.
func (s *Service) open(row store.CustomSource) (Source, error) {
	src := Source{
		ID: row.ID, Name: row.Name, URL: row.URL, Method: row.Method, Body: row.Body, Auth: row.Auth,
		AuthName: row.AuthName, Interval: time.Duration(row.IntervalSeconds) * time.Second, Sample: row.Sample,
	}
	if err := json.Unmarshal([]byte(row.Headers), &src.Headers); err != nil {
		return Source{}, fmt.Errorf("stored headers of custom source %d are invalid: %w", row.ID, err)
	}
	if row.SecretsSealed == "" {
		return src, nil
	}
	plain, err := s.Box.Open(row.SecretsSealed)
	if err != nil {
		return Source{}, err
	}
	var secrets sealedSecrets
	if err := json.Unmarshal([]byte(plain), &secrets); err != nil {
		return Source{}, errors.New("stored secrets are invalid")
	}
	src.AuthSecret = secrets.Auth
	for i, h := range src.Headers {
		if h.Secret {
			src.Headers[i].Value = secrets.Headers[strings.ToLower(h.Name)]
		}
	}
	return src, nil
}

func (s *Service) view(row store.CustomSource) SourceView {
	v := SourceView{
		ID: row.ID, Name: row.Name, URL: row.URL, Method: row.Method, Body: row.Body,
		IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Headers: []Header{},
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
		v.Auth.SecretSet = row.Auth != AuthNone && err == nil
	}
	return v
}
