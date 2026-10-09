// Package apitoken manages the tokens that automations, such as a Home
// Assistant rest_command, use to call GDash without a login. Like display
// tokens, only a hash is stored: the token is shown once, when it is made.
package apitoken

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// Prefix makes a token recognisable in a configuration file, and to
	// secret scanners, without telling anything about its value.
	Prefix        = "gdk_"
	maxNameLength = 40
	// A used token is touched at most once per interval, so a busy
	// automation does not write to the database on every request.
	touchInterval = time.Minute
	// Wrong tokens per address before it is refused for a while. Tokens
	// are 256 bits, so this protects the server's time, not the tokens.
	maxFailures   = 20
	failureWindow = 15 * time.Minute
)

var (
	// ErrInvalidToken is returned for a token that is malformed, unknown or revoked.
	ErrInvalidToken = errors.New("invalid API token")
	// ErrRateLimited is returned when an address sent too many wrong tokens.
	ErrRateLimited = errors.New("too many invalid API tokens, try again later")
)

// Created is a new token with its secret, which is never available again.
type Created struct {
	store.APIToken
	Token string `json:"token"`
}

// Service creates and checks API tokens.
type Service struct {
	store    *store.Store
	now      func() time.Time
	failures *ratelimit.Limiter
}

// New creates the service.
func New(st *store.Store) *Service {
	return &Service{store: st, now: time.Now, failures: ratelimit.New(time.Now, maxFailures, failureWindow)}
}

// Create makes a token. It returns a *validation.Error for an unusable name
// and store.ErrConflict at store.MaxAPITokens tokens.
func (s *Service) Create(ctx context.Context, name string) (Created, error) {
	name = strings.TrimSpace(name)
	var errs validation.Collector
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameLength {
		errs.Add("name", "use 1 to 40 characters")
	} else if validation.HasControlCharacter(name) {
		errs.Add("name", "use letters, digits and punctuation only")
	}
	if err := errs.Err(); err != nil {
		return Created{}, err
	}
	secret, err := token.New()
	if err != nil {
		return Created{}, err
	}
	secret = Prefix + secret
	stored, err := s.store.CreateAPIToken(ctx, store.NewAPIToken{Name: name, Hash: token.Hash(secret), Created: s.now().UTC()})
	if err != nil {
		return Created{}, err
	}
	return Created{APIToken: stored, Token: secret}, nil
}

// Authenticate returns the token that secret belongs to. Wrong secrets
// count against clientIP. It returns ErrInvalidToken and ErrRateLimited.
func (s *Service) Authenticate(ctx context.Context, secret, clientIP string) (store.APIToken, error) {
	key := "apitoken|" + clientIP
	if !s.failures.Allowed(key) {
		return store.APIToken{}, ErrRateLimited
	}
	stored, err := s.lookup(ctx, secret)
	if errors.Is(err, store.ErrNotFound) {
		s.failures.Fail(key)
		return store.APIToken{}, ErrInvalidToken
	}
	if err != nil {
		return store.APIToken{}, err
	}
	if now := s.now(); stored.LastUsedAt == nil || now.Sub(*stored.LastUsedAt) >= touchInterval {
		if err := s.store.TouchAPIToken(ctx, stored.ID, now.UTC()); err != nil {
			return store.APIToken{}, err
		}
	}
	return stored, nil
}

// lookup skips the database for text that cannot be one of our tokens.
func (s *Service) lookup(ctx context.Context, secret string) (store.APIToken, error) {
	if !strings.HasPrefix(secret, Prefix) {
		return store.APIToken{}, store.ErrNotFound
	}
	return s.store.APITokenByHash(ctx, token.Hash(secret))
}
