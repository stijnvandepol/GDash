package auth

import (
	"sync"
	"time"
)

const (
	maxFailures   = 5
	failureWindow = 15 * time.Minute
	// Above this many tracked keys, old entries are pruned on the next
	// failure, so a flood of random usernames cannot grow memory forever.
	pruneThreshold = 1000
)

// limiter blocks a key (an IP address or a username) after too many failed
// attempts within a sliding window.
type limiter struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[string][]time.Time
}

func newLimiter(now func() time.Time) *limiter {
	return &limiter{now: now, failures: map[string][]time.Time{}}
}

// allowed reports whether every key is below the failure limit.
func (l *limiter) allowed(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-failureWindow)
	for _, key := range keys {
		if len(recent(l.failures[key], cutoff)) >= maxFailures {
			return false
		}
	}
	return true
}

func (l *limiter) fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-failureWindow)
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

func (l *limiter) reset(keys ...string) {
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
