// Package pairing decides which screens may show a display. A browser
// screen pairs by showing a code that an admin enters in the web interface
// (the device flow of RFC 8628); a page opened by a cast keeper exchanges a
// single-use launch token from its URL. Either way the screen ends up with a
// display token in a cookie.
package pairing

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/token"
)

const (
	// codeAlphabet leaves out 0/O and 1/I/L, which look alike on a TV.
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength   = 8
	codeTTL      = 10 * time.Minute
	// A launch token only has to survive the TV loading the page.
	launchTTL = 2 * time.Minute
	// Bounds on the in-memory maps, so unauthenticated callers cannot grow
	// them without limit.
	maxPendingCodes    = 200
	maxPendingLaunches = 1000
	// A screen requests a new code when the old one expires; ten requests
	// per quarter hour per address is plenty for that.
	maxCodeRequests = 10
	requestWindow   = 15 * time.Minute
	touchInterval   = time.Hour
	// screenIdleTimeout unpairs a screen that has not shown its display for
	// this long: a tablet given away or a token copied off a screen stops
	// working on its own, while a screen in use renews it every hour.
	screenIdleTimeout = 90 * 24 * time.Hour
)

var (
	// ErrUnknownDisplay is returned for a display that does not exist.
	ErrUnknownDisplay = errors.New("unknown display")
	// ErrRateLimited is returned when an address requests too many codes.
	ErrRateLimited = errors.New("too many pairing requests, try again later")
	// ErrInvalidCode is returned for a code that is unknown, expired or for
	// another display.
	ErrInvalidCode = errors.New("invalid or expired pairing code")
	// ErrPending is returned while a code waits for approval.
	ErrPending = errors.New("pairing not approved yet")
	// ErrInvalidToken is returned for an unknown or expired token.
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Request is a pending pairing as the screen sees it.
type Request struct {
	Code      string    `json:"code"`
	PollToken string    `json:"pollToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type pending struct {
	displayID string
	pollHash  string
	expires   time.Time
	// approvedToken is the display token, held until the screen claims it.
	approvedToken string
}

type launch struct {
	displayID string
	expires   time.Time
	// device is the Cast device the token was sent to; only it may use it.
	device netip.Addr
}

// Service manages pairing. Pending codes and launch tokens live in memory:
// after a restart a screen simply asks for a new code.
type Service struct {
	store   *store.Store
	now     func() time.Time
	limiter *ratelimit.Limiter

	mu       sync.Mutex
	codes    map[string]*pending
	launches map[string]launch
}

// New creates the service.
func New(st *store.Store) *Service {
	return &Service{
		store:    st,
		now:      time.Now,
		limiter:  ratelimit.New(time.Now, maxCodeRequests, requestWindow),
		codes:    map[string]*pending{},
		launches: map[string]launch{},
	}
}

// Begin starts pairing a screen with a display and returns the code to show.
func (s *Service) Begin(ctx context.Context, displayID, clientIP string) (Request, error) {
	key := "pairing|" + ratelimit.AddressKey(clientIP)
	if !s.limiter.Attempt(key) {
		return Request{}, ErrRateLimited
	}
	if _, err := s.store.Display(ctx, displayID); errors.Is(err, store.ErrNotFound) {
		return Request{}, ErrUnknownDisplay
	} else if err != nil {
		return Request{}, err
	}
	pollToken, err := token.New()
	if err != nil {
		return Request{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.codes) >= maxPendingCodes {
		return Request{}, ErrRateLimited
	}
	code, err := s.uniqueCodeLocked()
	if err != nil {
		return Request{}, err
	}
	expires := s.now().Add(codeTTL)
	s.codes[code] = &pending{displayID: displayID, pollHash: token.Hash(pollToken), expires: expires}
	return Request{Code: formatCode(code), PollToken: pollToken, ExpiresAt: expires}, nil
}

// Approve pairs the screen showing code with displayID. It is the admin's
// half of the flow and needs a logged-in user.
func (s *Service) Approve(ctx context.Context, displayID, code string) error {
	code = normalizeCode(code)
	s.mu.Lock()
	p, ok := s.codes[code]
	valid := ok && p.displayID == displayID && s.now().Before(p.expires)
	alreadyApproved := valid && p.approvedToken != ""
	s.mu.Unlock()
	if !valid {
		return ErrInvalidCode
	}
	if alreadyApproved {
		return nil
	}
	issued, err := s.issue(ctx, displayID, store.TokenScreen)
	if err != nil {
		return err
	}
	s.mu.Lock()
	p.approvedToken = issued
	s.mu.Unlock()
	return nil
}

// Claim returns the display token once the code behind pollToken has been
// approved, and forgets the code. Until then it returns ErrPending.
func (s *Service) Claim(pollToken string) (displayToken, displayID string, err error) {
	pollHash := token.Hash(pollToken)
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, p := range s.codes {
		if p.pollHash != pollHash {
			continue
		}
		if !s.now().Before(p.expires) {
			delete(s.codes, code)
			return "", "", ErrInvalidToken
		}
		if p.approvedToken == "" {
			return "", "", ErrPending
		}
		delete(s.codes, code)
		return p.approvedToken, p.displayID, nil
	}
	return "", "", ErrInvalidToken
}

// NewLaunchToken returns a single-use token for the URL a cast keeper opens
// on the Cast device at device, valid for two minutes and only from that
// address: a device on the network that answers discovery in the TV's name
// receives the URL, but cannot pair a screen of its own with it.
func (s *Service) NewLaunchToken(displayID string, device netip.Addr) (string, error) {
	launchToken, err := token.New()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.launches) >= maxPendingLaunches {
		return "", errors.New("too many unused launch tokens")
	}
	s.launches[token.Hash(launchToken)] = launch{displayID: displayID, expires: s.now().Add(launchTTL), device: device.Unmap()}
	return launchToken, nil
}

// Exchange turns a launch token into a display token for a screen at
// clientIP. A launch token works once, and only from the device it was sent
// to; a try from elsewhere leaves it for that device.
func (s *Service) Exchange(ctx context.Context, launchToken, clientIP string) (displayToken, displayID string, err error) {
	hash := token.Hash(launchToken)
	client, _ := netip.ParseAddr(clientIP)
	s.mu.Lock()
	l, ok := s.launches[hash]
	if ok && client.Unmap() != l.device {
		s.mu.Unlock()
		return "", "", ErrInvalidToken
	}
	delete(s.launches, hash)
	s.mu.Unlock()
	if !ok || !s.now().Before(l.expires) {
		return "", "", ErrInvalidToken
	}
	displayToken, err = s.issue(ctx, l.displayID, store.TokenCast)
	if err != nil {
		return "", "", err
	}
	return displayToken, l.displayID, nil
}

// Screen is a paired screen as its display token identifies it.
type Screen struct {
	DisplayID string
	// Key tells the screens of one display apart without revealing their
	// tokens: "cast" for the TV a cast keeper opened, which gets a new token
	// at every launch but stays one screen, else the hash of the token.
	Key string
	// Kind is store.TokenScreen or store.TokenCast.
	Kind string
}

// castScreenKey is the key of the one TV a cast keeper keeps a display on.
const castScreenKey = "cast"

// Authenticate returns the display a display token may show.
func (s *Service) Authenticate(ctx context.Context, displayToken string) (string, error) {
	screen, err := s.Screen(ctx, displayToken)
	return screen.DisplayID, err
}

// Screen returns the screen a display token belongs to, or ErrInvalidToken.
func (s *Service) Screen(ctx context.Context, displayToken string) (Screen, error) {
	if displayToken == "" {
		return Screen{}, ErrInvalidToken
	}
	hash := token.Hash(displayToken)
	t, err := s.store.DisplayToken(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return Screen{}, ErrInvalidToken
	}
	if err != nil {
		return Screen{}, err
	}
	if s.now().Sub(t.LastSeenAt) > screenIdleTimeout {
		if err := s.store.DeleteDisplayToken(ctx, hash); err != nil {
			return Screen{}, err
		}
		return Screen{}, ErrInvalidToken
	}
	if s.now().Sub(t.LastSeenAt) > touchInterval {
		if err := s.store.TouchDisplayToken(ctx, hash, s.now()); err != nil {
			return Screen{}, err
		}
	}
	key := hash
	if t.Kind == store.TokenCast {
		key = castScreenKey
	}
	return Screen{DisplayID: t.DisplayID, Key: key, Kind: t.Kind}, nil
}

// Unpair removes the screens paired with a code and returns how many there
// were. A TV that a cast keeper opened stays paired, so unpairing the extra
// screens of a Cast display never makes its TV ask for a code.
func (s *Service) Unpair(ctx context.Context, displayID string) (int64, error) {
	return s.store.DeleteScreenTokens(ctx, displayID)
}

func (s *Service) issue(ctx context.Context, displayID, kind string) (string, error) {
	issued, err := token.New()
	if err != nil {
		return "", err
	}
	err = s.store.CreateDisplayToken(ctx, store.NewDisplayToken{Hash: token.Hash(issued), DisplayID: displayID, Kind: kind, Created: s.now()})
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrUnknownDisplay
	}
	if err != nil {
		return "", err
	}
	return issued, nil
}

func (s *Service) pruneLocked() {
	now := s.now()
	for code, p := range s.codes {
		if !now.Before(p.expires) {
			delete(s.codes, code)
		}
	}
	for hash, l := range s.launches {
		if !now.Before(l.expires) {
			delete(s.launches, hash)
		}
	}
}

func (s *Service) uniqueCodeLocked() (string, error) {
	for range 10 {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		if _, taken := s.codes[code]; !taken {
			return code, nil
		}
	}
	return "", errors.New("could not find a free pairing code")
}
