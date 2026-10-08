// Package ratelimit blocks a key, such as an IP address, a username or a
// pairing code, after too many failed attempts within a sliding window.
package ratelimit

import (
	"sync"
	"time"
)

// pruneThreshold bounds memory: above this many tracked keys, keys without
// recent failures are dropped on the next failure, so a flood of random
// usernames or codes cannot grow the map forever.
const pruneThreshold = 1000

// Limiter counts failures per key.
type Limiter struct {
	maxFailures int
	window      time.Duration
	now         func() time.Time

	mu       sync.Mutex
	failures map[string][]time.Time
}

// New returns a limiter that blocks a key after maxFailures failures within
// window.
func New(now func() time.Time, maxFailures int, window time.Duration) *Limiter {
	return &Limiter{maxFailures: maxFailures, window: window, now: now, failures: map[string][]time.Time{}}
}

// Allowed reports whether every key is below the failure limit.
func (l *Limiter) Allowed(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-l.window)
	for _, key := range keys {
		if len(recent(l.failures[key], cutoff)) >= l.maxFailures {
			return false
		}
	}
	return true
}

// Fail records a failed attempt for every key.
func (l *Limiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-l.window)
	if len(l.failures) > pruneThreshold {
		for key, times := range l.failures {
			if len(recent(times, cutoff)) == 0 {
				delete(l.failures, key)
			}
		}
	}
	for _, key := range keys {
		l.failures[key] = append(recent(l.failures[key], cutoff), now)
	}
}

// Reset forgets the failures of every key, after a successful attempt.
func (l *Limiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		delete(l.failures, key)
	}
}

func recent(times []time.Time, cutoff time.Time) []time.Time {
	for i, t := range times {
		if t.After(cutoff) {
			return times[i:]
		}
	}
	return nil
}
