package ratelimit

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestLimiterBlocksAfterMaxFailuresAndRecovers(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)}
	l := New(c.now, 3, time.Minute)

	for range 3 {
		if !l.Allowed("ip|1") {
			t.Fatal("blocked before the limit")
		}
		l.Fail("ip|1")
	}
	if l.Allowed("ip|1") {
		t.Fatal("not blocked after 3 failures")
	}
	if !l.Allowed("ip|2") {
		t.Fatal("another key is blocked too")
	}
	if l.Allowed("ip|2", "ip|1") {
		t.Fatal("Allowed must fail when any key is blocked")
	}

	c.t = c.t.Add(time.Minute + time.Second)
	if !l.Allowed("ip|1") {
		t.Fatal("still blocked after the window passed")
	}
}

func TestResetClearsFailures(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)}
	l := New(c.now, 1, time.Hour)
	l.Fail("user|stijn")
	l.Reset("user|stijn")
	if !l.Allowed("user|stijn") {
		t.Fatal("blocked after Reset")
	}
}

func TestLimiterPrunesOldKeys(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)}
	l := New(c.now, 5, time.Minute)
	for i := range pruneThreshold + 1 {
		l.Fail(fmt.Sprintf("code|%d", i))
	}
	c.t = c.t.Add(2 * time.Minute)
	l.Fail("code|new")
	if n := len(l.failures); n != 1 {
		t.Fatalf("tracked keys = %d after pruning, want 1", n)
	}
}

func TestAttemptCountsParallelRequests(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 9, 13, 0, 0, 0, time.UTC)}
	l := New(c.now, 5, time.Minute)
	var wg sync.WaitGroup
	var passed atomic.Int32
	for range 50 {
		wg.Go(func() {
			if l.Attempt("ip|1") {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 5 {
		t.Fatalf("%d of 50 parallel attempts passed, want exactly 5", passed.Load())
	}
}

func TestReleaseTakesBackOneAttempt(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 9, 13, 0, 0, 0, time.UTC)}
	l := New(c.now, 2, time.Minute)
	l.Fail("ip|1")
	l.Attempt("ip|1")
	l.Release("ip|1")
	if !l.Allowed("ip|1") {
		t.Fatal("blocked after the second attempt was released")
	}
	l.Release("ip|unknown")
}
