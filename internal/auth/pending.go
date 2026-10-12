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

// errLoginSpent is returned by pendingLogins.take for a login that had
// maxPendingCodes codes already.
var errLoginSpent = errors.New("the login had all its codes")

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
// changed in between ends it. codes counts the codes checked or being
// checked for it.
type pendingLogin struct {
	userID       int64
	username     string
	passwordHash string
	expires      time.Time
	codes        int
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

// take returns the pending login of secret and counts a code for it, under
// one lock, so parallel codes cannot get past maxPendingCodes. It returns
// ErrLoginExpired, or errLoginSpent with the login: a spent login stays
// until it expires or the next one replaces it, so FinishLogin still knows
// whose it was.
func (p *pendingLogins) take(secret string, now time.Time) (pendingLogin, error) {
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
	if login.codes >= maxPendingCodes {
		return login, errLoginSpent
	}
	login.codes++
	p.logins[key] = login
	return login, nil
}

// giveBack uncounts a code that take counted but that was never checked.
func (p *pendingLogins) giveBack(secret string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := token.Hash(secret)
	if login, ok := p.logins[key]; ok && login.codes > 0 {
		login.codes--
		p.logins[key] = login
	}
}

// spent reports whether the login of secret had all its codes.
func (p *pendingLogins) spent(secret string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	login, ok := p.logins[token.Hash(secret)]
	return !ok || login.codes >= maxPendingCodes
}

func (p *pendingLogins) end(secret string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.logins, token.Hash(secret))
}

// awaitCode answers a right password of u, whose account has two-step login
// on, with a *CodeNeeded. The password attempt is not counted as a failure,
// but it does not clear the earlier ones either: otherwise someone who
// knows the password could wipe the count of wrong codes at will.
func (s *Service) awaitCode(u store.User, keys []string, account string) error {
	s.limiter.Release(keys...)
	s.accountLimiter.Release(account)
	pending, err := s.pending.start(u, s.now())
	if err != nil {
		return err
	}
	return &CodeNeeded{Pending: pending}
}

// FinishLogin completes a login that Login answered with CodeNeeded, with a
// code from the authenticator app or a recovery code, and starts the
// session. Wrong codes count towards the same limits as wrong passwords,
// and towards those of codelimit.go. It returns ErrWrongCode (the pending
// login survives it, so the person can try again), ErrWrongCode joined with
// ErrLoginExpired for the wrong code that ends the pending login, the
// errors of verifyCode, ErrRateLimited or ErrLoginExpired. With
// ErrWrongCode it also returns the user: a wrong code after the right
// password is worth a line in the audit log.
func (s *Service) FinishLogin(ctx context.Context, pending, code, clientIP string) (string, User, CodeUse, error) {
	login, err := s.pending.take(pending, s.now())
	if errors.Is(err, errLoginSpent) {
		return "", User{}, CodeUse{}, s.spentLogin(login, clientIP)
	}
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
	err = s.limited(u.Username, clientIP, finishesLogin, func() error {
		use, err = s.verifyCode(ctx, u.ID, code)
		return err
	})
	switch {
	case errors.Is(err, ErrTwoStepOff):
		// Turned off in between, by a reset: the password is new too.
		s.pending.end(pending)
		return "", User{}, CodeUse{}, ErrLoginExpired
	case errors.Is(err, ErrWrongCode):
		if s.pending.spent(pending) {
			err = errors.Join(err, ErrLoginExpired)
		}
		return "", User{ID: u.ID, Username: u.Username}, CodeUse{}, err
	case err != nil:
		if refused(err) {
			s.pending.giveBack(pending)
		}
		return "", User{}, CodeUse{}, err
	}
	s.pending.end(pending)
	if err := s.deleteExpiredSessions(ctx); err != nil {
		return "", User{}, CodeUse{}, err
	}
	session, user, err := s.startSession(ctx, u)
	return session, user, use, err
}

// spentLogin answers a code for a pending login that had all its codes.
// The login limits answer first, so an address that is blocked hears that,
// rather than to type a password it will be refused for.
func (s *Service) spentLogin(login pendingLogin, clientIP string) error {
	keys, account := attemptKeys(login.username, clientIP)
	if !s.limiter.Allowed(keys...) || !s.accountLimiter.Allowed(account) {
		return ErrRateLimited
	}
	return ErrLoginExpired
}
