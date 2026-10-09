// Package auth handles accounts and sessions of the web interface: the
// one-time setup, login with rate limiting, and session lifetimes.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// A dashboard admin logs in rarely; a week of inactivity or a month in
	// total ends a session.
	sessionIdleTimeout     = 7 * 24 * time.Hour
	sessionAbsoluteTimeout = 30 * 24 * time.Hour
	// Recording every request would mean a database write per page load.
	touchInterval = time.Hour

	maxFailures   = 5
	failureWindow = 15 * time.Minute
)

var (
	// ErrInvalidCredentials is returned for a wrong username or password,
	// without saying which, so usernames cannot be probed.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrRateLimited is returned after too many failed attempts.
	ErrRateLimited = errors.New("too many failed attempts, try again later")
	// ErrSetupDone is returned when setup runs while an account exists.
	ErrSetupDone = errors.New("setup has already been completed")
	// ErrInvalidSetupToken is returned for a wrong setup token.
	ErrInvalidSetupToken = errors.New("invalid setup token")
	// ErrNoSession is returned for a missing, unknown or expired session.
	ErrNoSession = errors.New("not logged in")
	// ErrWrongPassword is returned when the current password given to
	// change it is not correct.
	ErrWrongPassword = errors.New("current password is not correct")
)

var validUsername = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// Credentials is a username and password as a person typed them.
type Credentials struct {
	Username string
	Password string
}

// User is the logged-in account as the rest of GDash sees it.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Service runs authentication against the store.
type Service struct {
	store      *store.Store
	limiter    *ratelimit.Limiter
	now        func() time.Time
	setupToken string
	// dummyHash is verified when a username does not exist, so a login for
	// an unknown user takes as long as one with a wrong password.
	dummyHash string
}

// NewService creates the service. setupToken must be a random secret shown
// to the person installing GDash; it is only accepted while no account
// exists.
func NewService(st *store.Store, setupToken string) (*Service, error) {
	dummy, err := hashPassword("gdash-timing-equaliser")
	if err != nil {
		return nil, err
	}
	return &Service{store: st, limiter: ratelimit.New(time.Now, maxFailures, failureWindow), now: time.Now, setupToken: setupToken, dummyHash: dummy}, nil
}

// NewSetupToken returns a random token for the first-run setup.
func NewSetupToken() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate setup token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SetupNeeded reports whether no account exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	has, err := s.store.HasUsers(ctx)
	return !has, err
}

// Setup creates the first account and logs it in. It returns the session
// token to put in a cookie.
func (s *Service) Setup(ctx context.Context, setupToken string, c Credentials, clientIP string) (string, User, error) {
	key := "setup|" + clientIP
	if !s.limiter.Allowed(key) {
		return "", User{}, ErrRateLimited
	}
	if subtle.ConstantTimeCompare([]byte(setupToken), []byte(s.setupToken)) != 1 {
		s.limiter.Fail(key)
		return "", User{}, ErrInvalidSetupToken
	}
	if err := validateAccount(c); err != nil {
		return "", User{}, err
	}
	hash, err := hashPassword(c.Password)
	if err != nil {
		return "", User{}, err
	}
	u, err := s.store.CreateFirstUser(ctx, c.Username, hash, s.now())
	if errors.Is(err, store.ErrConflict) {
		return "", User{}, ErrSetupDone
	}
	if err != nil {
		return "", User{}, err
	}
	s.limiter.Reset(key)
	return s.startSession(ctx, u)
}

// Login checks credentials and starts a session.
func (s *Service) Login(ctx context.Context, c Credentials, clientIP string) (string, User, error) {
	keys := []string{"ip|" + clientIP, "user|" + strings.ToLower(c.Username)}
	// The attempt counts as a failure until the password is proven right.
	if !s.limiter.Attempt(keys...) {
		return "", User{}, ErrRateLimited
	}
	if len(c.Password) > maxPasswordBytes {
		return "", User{}, ErrInvalidCredentials
	}
	u, err := s.store.UserByName(ctx, c.Username)
	hash := u.PasswordHash
	if errors.Is(err, store.ErrNotFound) {
		hash = s.dummyHash
	} else if err != nil {
		return "", User{}, err
	}
	ok, err := verifyPassword(hash, c.Password)
	if err != nil {
		return "", User{}, fmt.Errorf("verify password of user %d: %w", u.ID, err)
	}
	if !ok || u.ID == 0 {
		return "", User{}, ErrInvalidCredentials
	}
	s.limiter.Reset(keys...)
	if err := s.store.DeleteExpiredSessions(ctx, s.now().Add(-sessionIdleTimeout), s.now().Add(-sessionAbsoluteTimeout)); err != nil {
		return "", User{}, err
	}
	return s.startSession(ctx, u)
}

// Authenticate returns the user of a session token, or ErrNoSession.
func (s *Service) Authenticate(ctx context.Context, session string) (User, error) {
	if session == "" {
		return User{}, ErrNoSession
	}
	hash := token.Hash(session)
	sess, err := s.store.SessionByHash(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, err
	}
	now := s.now()
	if now.Sub(sess.LastSeenAt) > sessionIdleTimeout || now.Sub(sess.CreatedAt) > sessionAbsoluteTimeout {
		if err := s.store.DeleteSession(ctx, hash); err != nil {
			return User{}, err
		}
		return User{}, ErrNoSession
	}
	if now.Sub(sess.LastSeenAt) > touchInterval {
		if err := s.store.TouchSession(ctx, hash, now); err != nil {
			return User{}, err
		}
	}
	return User{ID: sess.User.ID, Username: sess.User.Username}, nil
}

// ChangePassword sets a new password for the user of session after checking
// the current one, and ends every other session of that user, so a lost or
// shared browser loses access. Wrong current passwords count towards the
// same rate limit as logins. It returns ErrWrongPassword, ErrRateLimited, or
// a *validation.Error for the new password (field "newPassword").
func (s *Service) ChangePassword(ctx context.Context, session, current, next, clientIP string) error {
	user, err := s.Authenticate(ctx, session)
	if err != nil {
		return err
	}
	keys := []string{"ip|" + clientIP, "user|" + strings.ToLower(user.Username)}
	// The attempt counts as a failure until the password is proven right.
	if !s.limiter.Attempt(keys...) {
		return ErrRateLimited
	}
	stored, err := s.store.UserByName(ctx, user.Username)
	if err != nil {
		return err
	}
	ok := false
	if len(current) <= maxPasswordBytes {
		if ok, err = verifyPassword(stored.PasswordHash, current); err != nil {
			return fmt.Errorf("verify password of user %d: %w", stored.ID, err)
		}
	}
	if !ok {
		return ErrWrongPassword
	}
	var errs validation.Collector
	if msg := passwordProblem(next); msg != "" {
		errs.Add("newPassword", msg)
		return errs.Err()
	}
	hash, err := hashPassword(next)
	if err != nil {
		return err
	}
	// Only the account's own count: the address may have failed logins
	// for other usernames that must keep counting.
	s.limiter.Reset("user|" + strings.ToLower(user.Username))
	s.limiter.Release("ip|" + clientIP)
	return s.store.ChangePassword(ctx, stored.ID, hash, token.Hash(session))
}

// Logout ends a session.
func (s *Service) Logout(ctx context.Context, session string) error {
	return s.store.DeleteSession(ctx, token.Hash(session))
}

func (s *Service) startSession(ctx context.Context, u store.User) (string, User, error) {
	session, err := token.New()
	if err != nil {
		return "", User{}, err
	}
	if err := s.store.CreateSession(ctx, token.Hash(session), u.ID, s.now()); err != nil {
		return "", User{}, err
	}
	return session, User{ID: u.ID, Username: u.Username}, nil
}

func validateAccount(c Credentials) error {
	var errs validation.Collector
	if !validUsername.MatchString(c.Username) {
		errs.Add("username", "use 1 to 32 letters, digits, dots, dashes or underscores")
	}
	if msg := passwordProblem(c.Password); msg != "" {
		errs.Add("password", msg)
	}
	return errs.Err()
}

// passwordProblem returns why a new password is not acceptable, or "".
func passwordProblem(password string) string {
	switch {
	case len([]rune(password)) < minPasswordLength:
		return fmt.Sprintf("use at least %d characters", minPasswordLength)
	case len(password) > maxPasswordBytes:
		return fmt.Sprintf("use at most %d bytes", maxPasswordBytes)
	}
	return ""
}
