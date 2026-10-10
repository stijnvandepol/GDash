package auth

import (
	"context"
	"fmt"
)

// Confirm checks the current password of the user of session again, before
// an action that should not rest on a session alone, such as changing the
// password or downloading a backup with the secret key. Wrong passwords
// count towards the same limits as logins. It returns the user, or
// ErrWrongPassword, ErrRateLimited or ErrNoSession.
func (s *Service) Confirm(ctx context.Context, session, password, clientIP string) (User, error) {
	user, err := s.Authenticate(ctx, session)
	if err != nil {
		return User{}, err
	}
	keys, account := attemptKeys(user.Username, clientIP)
	if !s.attempt(keys, account) {
		return User{}, ErrRateLimited
	}
	stored, err := s.store.UserByName(ctx, user.Username)
	if err != nil {
		return User{}, err
	}
	ok := false
	if len(password) <= maxPasswordBytes {
		if ok, err = verifyPassword(stored.PasswordHash, password); err != nil {
			return User{}, fmt.Errorf("verify password of user %d: %w", stored.ID, err)
		}
	}
	if !ok {
		return User{}, ErrWrongPassword
	}
	// Only the account's own counts: the address may have failed logins
	// for other usernames that must keep counting.
	s.limiter.Reset(keys[1])
	s.limiter.Release(keys[0])
	s.accountLimiter.Reset(account)
	return User{ID: stored.ID, Username: stored.Username}, nil
}
