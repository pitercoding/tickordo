// Package ratelimit limits how many requests each key may make within a time
// window. It knows nothing about HTTP: callers resolve the key (for example the
// client IP from middleware.ClientIP) and decide how to answer a rejection.
//
// The strategy is a fixed window per key. A key's window starts at its first
// request and lasts for the configured duration; up to limit requests are
// admitted inside it and the rest are rejected. Once the window expires, the
// next request starts a new window with a fresh count. Rejected requests are
// not counted, so they never push the window further out.
//
// Because windows are fixed, a key can make up to 2*limit requests in a span
// slightly longer than one window: limit at the end of a window and limit at
// the start of the next. A sliding window would avoid that burst at the cost
// of storing a timestamp per request, which is not worth it for these limits.
//
// State lives in memory and is per process: it is lost on restart and not
// shared between replicas.
package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrEmptyKey is returned by Allow when the key is empty. An empty key usually
// means the caller failed to resolve the client, and giving all such requests
// one shared counter would let a single client exhaust it for everyone else.
var ErrEmptyKey = errors.New("rate limit key cannot be empty")

// Decision is the outcome of Allow for one request.
type Decision struct {
	// Allowed reports whether the request is within the limit.
	Allowed bool

	// Remaining is how many more requests the key may make in its current
	// window. It is zero when the request was rejected.
	Remaining int

	// ResetAfter is how long until the key's current window expires. For a
	// rejected request it is the wait before the key is admitted again.
	ResetAfter time.Duration
}

// Limiter admits up to limit requests per key within each window. It is safe
// for concurrent use. Create it with New; the zero value is not usable.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]entry

	// nextSweep is when Allow next removes expired entries.
	nextSweep time.Time
}

// entry is the state of one key's current window.
type entry struct {
	start time.Time
	count int
}

// New returns a Limiter that admits up to limit requests per key within each
// window. Both must be positive; otherwise New returns an error instead of a
// limiter that would block or admit everything.
func New(limit int, window time.Duration) (*Limiter, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("invalid rate limit: must be a positive integer, got %d", limit)
	}

	if window <= 0 {
		return nil, fmt.Errorf("invalid rate limit window: must be a positive duration, got %s", window)
	}

	return &Limiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		entries: make(map[string]entry),
	}, nil
}

// Allow records a request for key and reports whether it is within the limit.
// It returns ErrEmptyKey, and records nothing, when key is empty.
func (l *Limiter) Allow(key string) (Decision, error) {
	if key == "" {
		return Decision{}, ErrEmptyKey
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	l.sweep(now)

	e, ok := l.entries[key]
	if !ok || l.expired(e, now) {
		e = entry{start: now}
	}

	resetAfter := e.start.Add(l.window).Sub(now)

	if e.count >= l.limit {
		return Decision{ResetAfter: resetAfter}, nil
	}

	e.count++
	l.entries[key] = e

	return Decision{
		Allowed:    true,
		Remaining:  l.limit - e.count,
		ResetAfter: resetAfter,
	}, nil
}

// expired reports whether e's window has ended at now.
func (l *Limiter) expired(e entry, now time.Time) bool {
	return now.Sub(e.start) >= l.window
}

// sweep removes expired entries, at most once per window, so memory follows
// the keys seen recently instead of every key ever seen. It runs only inside
// Allow, so there is no background cleanup: without traffic nothing is
// removed, and while Allow keeps being called an entry is removed at most two
// windows after it started. Between sweeps the map grows by one entry per
// distinct key, with no upper bound. The cost is one pass over the map per
// window, made while holding the mutex.
func (l *Limiter) sweep(now time.Time) {
	if now.Before(l.nextSweep) {
		return
	}

	for key, e := range l.entries {
		if l.expired(e, now) {
			delete(l.entries, key)
		}
	}

	l.nextSweep = now.Add(l.window)
}
