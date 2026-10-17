package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// newTestGlobal returns a Global driven by a fake clock.
func newTestGlobal(t *testing.T, limit int, window time.Duration) (*Global, *fakeClock) {
	t.Helper()

	g, err := NewGlobal(limit, window)
	if err != nil {
		t.Fatalf("NewGlobal(%d, %s) returned error: %v", limit, window, err)
	}

	clock := &fakeClock{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	g.limiter.now = clock.Now

	return g, clock
}

func TestNewGlobalRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{name: "zero limit", limit: 0, window: time.Hour},
		{name: "negative limit", limit: -1, window: time.Hour},
		{name: "zero window", limit: 100, window: 0},
		{name: "negative window", limit: 100, window: -time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGlobal(tt.limit, tt.window)
			if err == nil {
				t.Fatal("NewGlobal returned nil error")
			}

			if g != nil {
				t.Fatalf("NewGlobal returned limiter %+v with error", g)
			}
		})
	}
}

func TestGlobalAllowRejectsOverLimit(t *testing.T) {
	g, clock := newTestGlobal(t, 3, 24*time.Hour)

	assertDecision(t, g.Allow(), Decision{Allowed: true, Remaining: 2, ResetAfter: 24 * time.Hour})

	clock.Advance(time.Hour)

	assertDecision(t, g.Allow(), Decision{Allowed: true, Remaining: 1, ResetAfter: 23 * time.Hour})
	assertDecision(t, g.Allow(), Decision{Allowed: true, Remaining: 0, ResetAfter: 23 * time.Hour})

	// The next request is rejected and told how long the window has left.
	assertDecision(t, g.Allow(), Decision{Allowed: false, Remaining: 0, ResetAfter: 23 * time.Hour})
}

func TestGlobalAllowRenewsExactlyAtWindowEnd(t *testing.T) {
	g, clock := newTestGlobal(t, 2, 24*time.Hour)

	g.Allow()
	g.Allow()

	clock.Advance(24*time.Hour - time.Nanosecond)

	assertDecision(t, g.Allow(), Decision{Allowed: false, Remaining: 0, ResetAfter: time.Nanosecond})

	clock.Advance(time.Nanosecond)

	// A new window starts at this request, with a fresh count.
	assertDecision(t, g.Allow(), Decision{Allowed: true, Remaining: 1, ResetAfter: 24 * time.Hour})
}

func TestGlobalAllowRejectionsDoNotExtendWindow(t *testing.T) {
	g, clock := newTestGlobal(t, 1, 24*time.Hour)

	g.Allow()

	// Rejections spread across the window keep counting down to the same
	// end, which was fixed by the first admitted request.
	for _, advance := range []time.Duration{time.Hour, 10 * time.Hour, 12 * time.Hour} {
		clock.Advance(advance)
		g.Allow()
	}

	// t0+23h: one hour left, despite the rejections.
	assertDecision(t, g.Allow(), Decision{Allowed: false, Remaining: 0, ResetAfter: time.Hour})

	clock.Advance(time.Hour)

	assertDecision(t, g.Allow(), Decision{Allowed: true, Remaining: 0, ResetAfter: 24 * time.Hour})
}

func TestGlobalAllowConcurrent(t *testing.T) {
	const (
		limit      = 100
		goroutines = 200
	)

	g, _ := newTestGlobal(t, limit, 24*time.Hour)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
		start   = make(chan struct{})
	)

	for range goroutines {
		wg.Go(func() {
			<-start

			if g.Allow().Allowed {
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

func TestRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "one nanosecond", d: time.Nanosecond, want: "1"},
		{name: "under one second", d: 300 * time.Millisecond, want: "1"},
		{name: "exactly one second", d: time.Second, want: "1"},
		{name: "just over one second", d: time.Second + time.Nanosecond, want: "2"},
		{name: "fractional seconds", d: 59*time.Second + 500*time.Millisecond, want: "60"},
		{name: "one day", d: 24 * time.Hour, want: "86400"},
		{name: "zero", d: 0, want: "1"},
		{name: "negative", d: -time.Second, want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RetryAfterSeconds(tt.d); got != tt.want {
				t.Errorf("RetryAfterSeconds(%s) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
