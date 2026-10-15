package ratelimit

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock, so tests control time without
// sleeping.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

// newTestLimiter returns a limiter driven by a fake clock.
func newTestLimiter(t *testing.T, limit int, window time.Duration) (*Limiter, *fakeClock) {
	t.Helper()

	l, err := New(limit, window)
	if err != nil {
		t.Fatalf("New(%d, %s) returned error: %v", limit, window, err)
	}

	clock := &fakeClock{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	l.now = clock.Now

	return l, clock
}

// allow calls l.Allow and fails the test on error.
func allow(t *testing.T, l *Limiter, key string) Decision {
	t.Helper()

	d, err := l.Allow(key)
	if err != nil {
		t.Fatalf("Allow(%q) returned error: %v", key, err)
	}

	return d
}

func assertDecision(t *testing.T, got, want Decision) {
	t.Helper()

	if got != want {
		t.Fatalf("decision = %+v, want %+v", got, want)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{name: "zero limit", limit: 0, window: time.Minute},
		{name: "negative limit", limit: -1, window: time.Minute},
		{name: "zero window", limit: 5, window: 0},
		{name: "negative window", limit: 5, window: -time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := New(tt.limit, tt.window)
			if err == nil {
				t.Fatal("New returned nil error")
			}

			if l != nil {
				t.Fatalf("New returned limiter %+v with error", l)
			}
		})
	}
}

func TestAllowBelowLimit(t *testing.T) {
	l, clock := newTestLimiter(t, 3, 10*time.Minute)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: true, Remaining: 2, ResetAfter: 10 * time.Minute})

	clock.Advance(time.Minute)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: true, Remaining: 1, ResetAfter: 9 * time.Minute})
}

func TestAllowAtLimit(t *testing.T) {
	l, _ := newTestLimiter(t, 3, 10*time.Minute)

	allow(t, l, "192.0.2.1")
	allow(t, l, "192.0.2.1")

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: true, Remaining: 0, ResetAfter: 10 * time.Minute})
}

func TestAllowRejectsOverLimit(t *testing.T) {
	l, clock := newTestLimiter(t, 3, 10*time.Minute)

	for range 3 {
		allow(t, l, "192.0.2.1")
	}

	clock.Advance(4 * time.Minute)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: false, Remaining: 0, ResetAfter: 6 * time.Minute})

	// Further rejections keep reporting the same window.
	clock.Advance(time.Minute)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: false, Remaining: 0, ResetAfter: 5 * time.Minute})
}

func TestAllowResetsAfterWindow(t *testing.T) {
	l, clock := newTestLimiter(t, 2, 10*time.Minute)

	allow(t, l, "192.0.2.1")
	allow(t, l, "192.0.2.1")

	clock.Advance(10*time.Minute - time.Nanosecond)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: false, Remaining: 0, ResetAfter: time.Nanosecond})

	clock.Advance(time.Nanosecond)

	// A new window starts at this request, with a fresh count.
	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: true, Remaining: 1, ResetAfter: 10 * time.Minute})

	clock.Advance(time.Minute)

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: true, Remaining: 0, ResetAfter: 9 * time.Minute})
	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: false, Remaining: 0, ResetAfter: 9 * time.Minute})
}

func TestAllowIsolatesKeys(t *testing.T) {
	l, _ := newTestLimiter(t, 1, 10*time.Minute)

	allow(t, l, "192.0.2.1")

	assertDecision(t, allow(t, l, "192.0.2.1"), Decision{Allowed: false, Remaining: 0, ResetAfter: 10 * time.Minute})
	assertDecision(t, allow(t, l, "192.0.2.2"), Decision{Allowed: true, Remaining: 0, ResetAfter: 10 * time.Minute})
	assertDecision(t, allow(t, l, "2001:db8::1"), Decision{Allowed: true, Remaining: 0, ResetAfter: 10 * time.Minute})
}

func TestAllowRejectsEmptyKey(t *testing.T) {
	l, _ := newTestLimiter(t, 1, 10*time.Minute)

	d, err := l.Allow("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("Allow(\"\") error = %v, want %v", err, ErrEmptyKey)
	}

	assertDecision(t, d, Decision{})

	if len(l.entries) != 0 {
		t.Fatalf("entries = %v, want none recorded for an empty key", l.entries)
	}
}

func TestAllowConcurrentSameKey(t *testing.T) {
	const (
		limit      = 10
		goroutines = 200
	)

	l, _ := newTestLimiter(t, limit, 10*time.Minute)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
		start   = make(chan struct{})
	)

	for range goroutines {
		wg.Go(func() {
			<-start

			d, err := l.Allow("192.0.2.1")
			if err != nil {
				t.Errorf("Allow returned error: %v", err)
				return
			}

			if d.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}

	close(start)
	wg.Wait()

	if allowed != limit {
		t.Fatalf("allowed = %d, want %d", allowed, limit)
	}
}

func TestAllowRemovesExpiredEntries(t *testing.T) {
	l, clock := newTestLimiter(t, 5, 10*time.Minute)

	// t0: the first call sweeps the empty map; the next sweep is due at t0+10m.
	allow(t, l, "a")

	// t0+5m: no sweep is due.
	clock.Advance(5 * time.Minute)

	allow(t, l, "b")

	assertKeys(t, l, "a", "b")

	// t0+10m: the sweep removes a, which expires exactly now, and keeps b,
	// which started at t0+5m. The next sweep is due at t0+20m.
	clock.Advance(5 * time.Minute)

	allow(t, l, "c")

	assertKeys(t, l, "b", "c")

	// t0+16m: b expired at t0+15m, but no sweep is due yet, so it stays.
	// Sweeping on every call would remove it here.
	clock.Advance(6 * time.Minute)

	allow(t, l, "d")

	assertKeys(t, l, "b", "c", "d")

	// t0+20m: the sweep removes b and c, which expires exactly now, and
	// keeps d, which started at t0+16m.
	clock.Advance(4 * time.Minute)

	allow(t, l, "e")

	assertKeys(t, l, "d", "e")

	// Long after everything expired, only the current key remains.
	clock.Advance(time.Hour)

	allow(t, l, "f")

	assertKeys(t, l, "f")
}

func assertKeys(t *testing.T, l *Limiter, want ...string) {
	t.Helper()

	var got []string
	for key := range l.entries {
		got = append(got, key)
	}

	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}
