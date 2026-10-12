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
	"time"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/secret"
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

	// maxFailures limits guesses per address, and per account from one
	// address. A device on the network can therefore not lock the
	// household out by failing its username five times.
	maxFailures   = 5
	failureWindow = 15 * time.Minute
	// maxAccountFailures limits guesses per account from all addresses
	// together, against guessing spread over many devices.
	maxAccountFailures = 50
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
	store   *store.Store
	limiter *ratelimit.Limiter
	// accountLimiter counts failures per account over all addresses.
	accountLimiter *ratelimit.Limiter
	now            func() time.Time
	setupToken     string
	// dummyHash is verified when a username does not exist, so a login for
	// an unknown user takes as long as one with a wrong password.
	dummyHash string
	// box seals the secrets of two-step login.
	box     *secret.Box
	pending pendingLogins
}

// NewService creates the service. box seals the secrets of two-step login.
// setupToken must be a random secret shown to the person installing GDash;
// it is only accepted while no account exists.
func NewService(st *store.Store, box *secret.Box, setupToken string) (*Service, error) {
	dummy, err := hashPassword("gdash-timing-equaliser")
	if err != nil {
		return nil, err
	}
	return &Service{
		store:          st,
		limiter:        ratelimit.New(time.Now, maxFailures, failureWindow),
		accountLimiter: ratelimit.New(time.Now, maxAccountFailures, failureWindow),
		now:            time.Now, setupToken: setupToken, dummyHash: dummy, box: box,
	}, nil
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
	key := "setup|" + ratelimit.AddressKey(clientIP)
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

// Login checks credentials and starts a session. For an account with
// two-step login on it starts no session but returns a *CodeNeeded, which
// FinishLogin completes.
func (s *Service) Login(ctx context.Context, c Credentials, clientIP string) (string, User, error) {
	keys, account := attemptKeys(c.Username, clientIP)
	if !s.attempt(keys, account) {
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
	twoStep, err := s.twoStepOn(ctx, u.ID)
	if err != nil {
		return "", User{}, err
	}
	if twoStep {
		// The right password is not counted as a failure, but it does not
		// clear the earlier ones either: otherwise someone who knows the
		// password could wipe the count of wrong codes at will.
		s.limiter.Release(keys...)
		s.accountLimiter.Release(account)
		pending, err := s.pending.start(u, s.now())
		if err != nil {
			return "", User{}, err
		}
		return "", User{}, &CodeNeeded{Pending: pending}
	}
	s.limiter.Reset(keys...)
	s.accountLimiter.Reset(account)
	if err := s.deleteExpiredSessions(ctx); err != nil {
		return "", User{}, err
	}
	return s.startSession(ctx, u)
}

func (s *Service) deleteExpiredSessions(ctx context.Context) error {
	return s.store.DeleteExpiredSessions(ctx, s.now().Add(-sessionIdleTimeout), s.now().Add(-sessionAbsoluteTimeout))
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
	user, err := s.Confirm(ctx, session, current, clientIP)
	if err != nil {
		return err
	}
	var errs validation.Collector
	if msg := passwordProblem(next, user.Username); msg != "" {
		errs.Add("newPassword", msg)
		return errs.Err()
	}
	hash, err := hashPassword(next)
	if err != nil {
		return err
	}
	return s.store.ChangePassword(ctx, user.ID, hash, token.Hash(session))
}

// ResetPassword gives an account a new random password, ends all its
// sessions and turns two-step login off, for an admin who lost the
// password, or the phone and the recovery codes. It is only reachable from
// the command line on the server: whoever can run that already controls
// GDash. It returns the new password and whether two-step login was on, or
// store.ErrNotFound for an unknown user.
func (s *Service) ResetPassword(ctx context.Context, username string) (password string, twoStepWasOn bool, err error) {
	u, err := s.store.UserByName(ctx, username)
	if err != nil {
		return "", false, err
	}
	password, err = token.New()
	if err != nil {
		return "", false, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", false, err
	}
	// No session is kept: every browser of the account has to log in again.
	twoStepWasOn, err = s.store.ResetPassword(ctx, u.ID, hash)
	if err != nil {
		return "", false, err
	}
	return password, twoStepWasOn, nil
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
	if msg := passwordProblem(c.Password, c.Username); msg != "" {
		errs.Add("password", msg)
	}
	return errs.Err()
}
