package auth

import (
	"strings"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
)

// attemptKeys returns the limiter keys of a password attempt: the address,
// the account from that address, and the account over all addresses.
func attemptKeys(username, clientIP string) (perAddress []string, account string) {
	address := ratelimit.AddressKey(clientIP)
	name := strings.ToLower(username)
	return []string{"ip|" + address, "user|" + name + "|" + address}, "user|" + name
}

// attempt counts a password attempt against both limiters, or neither when
// one of them is at its limit. Each counts it as a failure until the
// password is proven right.
func (s *Service) attempt(perAddress []string, account string) bool {
	if !s.accountLimiter.Attempt(account) {
		return false
	}
	if !s.limiter.Attempt(perAddress...) {
		s.accountLimiter.Release(account)
		return false
	}
	return true
}
