package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
)

// pendingLifetime is how long the code may take after the password: long
// enough to find the phone, short enough that a password seen over a
// shoulder is no lasting half-login.
const pendingLifetime = 5 * time.Minute

// CodeNeeded is the error Login returns when the password is right and the
// account has two-step login on. No session exists yet: the client sends
// Pending back with a code to FinishLogin. Pending is a secret for a few
// minutes, so it is not part of the error text and must not be logged.
type CodeNeeded struct {
	Pending string
}

func (e *CodeNeeded) Error() string { return "a two-step code is needed" }

// pendingLogin is a login whose password was right and whose code is still
// to come. passwordHash is the hash it was proven against, so a password
// changed in between ends it.
type pendingLogin struct {
	userID       int64
	username     string
	passwordHash string
	expires      time.Time
}

// pendingLogins holds the half-done logins in memory: a restart only means
// typing the password again. There is at most one per account, the newest,
// so the map cannot grow beyond the number of accounts, and only someone
// with the right password can add to it.
type pendingLogins struct {
	mu     sync.Mutex
	logins map[string]pendingLogin
}

func (p *pendingLogins) start(u store.User, now time.Time) (string, error) {
	secret, err := token.New()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.logins == nil {
		p.logins = map[string]pendingLogin{}
	}
	for key, login := range p.logins {
		if login.userID == u.ID || !now.Before(login.expires) {
			delete(p.logins, key)
		}
	}
	p.logins[token.Hash(secret)] = pendingLogin{
		userID: u.ID, username: u.Username, passwordHash: u.PasswordHash, expires: now.Add(pendingLifetime),
	}
	return secret, nil
}

// find returns the pending login of secret, or ErrLoginExpired.
func (p *pendingLogins) find(secret string, now time.Time) (pendingLogin, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := token.Hash(secret)
	login, ok := p.logins[key]
	if !ok {
		return pendingLogin{}, ErrLoginExpired
	}
	if !now.Before(login.expires) {
		delete(p.logins, key)
		return pendingLogin{}, ErrLoginExpired
	}
	return login, nil
}

func (p *pendingLogins) end(secret string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.logins, token.Hash(secret))
}

// FinishLogin completes a login that Login answered with CodeNeeded, with a
// code from the authenticator app or a recovery code, and starts the
// session. Wrong codes count towards the same limits as wrong passwords.
// It returns ErrWrongCode, ErrRateLimited or ErrLoginExpired; the pending
// login survives a wrong code, so the person can try again.
func (s *Service) FinishLogin(ctx context.Context, pending, code, clientIP string) (string, User, CodeUse, error) {
	login, err := s.pending.find(pending, s.now())
	if err != nil {
		return "", User{}, CodeUse{}, err
	}
	u, err := s.store.UserByName(ctx, login.username)
	if errors.Is(err, store.ErrNotFound) || err == nil && (u.ID != login.userID || u.PasswordHash != login.passwordHash) {
		s.pending.end(pending)
		return "", User{}, CodeUse{}, ErrLoginExpired
	}
	if err != nil {
		return "", User{}, CodeUse{}, err
	}
	var use CodeUse
	err = s.limited(u.Username, clientIP, func() error {
		use, err = s.verifyCode(ctx, u.ID, code)
		return err
	})
	if errors.Is(err, ErrTwoStepOff) {
		// Turned off in between, by a reset: the password is new too.
		s.pending.end(pending)
		return "", User{}, CodeUse{}, ErrLoginExpired
	}
	if err != nil {
		return "", User{}, CodeUse{}, err
	}
	s.pending.end(pending)
	if err := s.deleteExpiredSessions(ctx); err != nil {
		return "", User{}, CodeUse{}, err
	}
	session, user, err := s.startSession(ctx, u)
	return session, user, use, err
}
