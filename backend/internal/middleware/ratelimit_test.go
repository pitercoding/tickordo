package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/pitercoding/tickordo/internal/ratelimit"
)

// fakeLimiter records the keys it is asked about and answers every call with
// decision and err, so tests choose ResetAfter without waiting for a window.
type fakeLimiter struct {
	decision ratelimit.Decision
	err      error
	keys     []string
}

func (l *fakeLimiter) Allow(key string) (ratelimit.Decision, error) {
	l.keys = append(l.keys, key)
	return l.decision, l.err
}

// newRealLimiter returns a ratelimit.Limiter with a window long enough that
// it never expires during a test.
func newRealLimiter(t *testing.T, limit int) *ratelimit.Limiter {
	t.Helper()

	l, err := ratelimit.New(limit, time.Hour)
	if err != nil {
		t.Fatalf("ratelimit.New returned error: %v", err)
	}

	return l
}

// serveRateLimited sends a POST /tickets from remoteAddr, with optional
// X-Forwarded-For lines, through RateLimit wrapping next.
func serveRateLimited(limiter Limiter, trusted []netip.Prefix, next http.Handler, remoteAddr string, xff ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/tickets", nil)
	req.RemoteAddr = remoteAddr

	for _, line := range xff {
		req.Header.Add("X-Forwarded-For", line)
	}

	rec := httptest.NewRecorder()

	RateLimit(limiter, trusted)(next).ServeHTTP(rec, req)

	return rec
}

// assertJSONError checks the status and the {"error": ...} body the handlers
// also write. json.Encoder ends its output with a newline.
func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	if got, want := rec.Body.String(), "{\"error\":\""+wantMessage+"\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

const tooManyRequestsMessage = "too many requests, please try again later"

func TestRateLimit_AllowsRequestsWithinLimit(t *testing.T) {
	limiter := newRealLimiter(t, 3)

	for i := range 3 {
		next := &recordingHandler{}

		rec := serveRateLimited(limiter, nil, next, "192.0.2.1:1234")

		if !next.called {
			t.Fatalf("request %d: next handler was not called", i+1)
		}

		// The response is next's, untouched by the middleware.
		if rec.Code != http.StatusCreated {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusCreated)
		}

		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("request %d: Retry-After = %q, want empty", i+1, got)
		}
	}
}

func TestRateLimit_RejectsRequestOverLimit(t *testing.T) {
	limiter := newRealLimiter(t, 2)

	for range 2 {
		serveRateLimited(limiter, nil, &recordingHandler{}, "192.0.2.1:1234")
	}

	next := &recordingHandler{}

	rec := serveRateLimited(limiter, nil, next, "192.0.2.1:1234")

	if next.called {
		t.Error("next handler was called for a rejected request")
	}

	assertJSONError(t, rec, http.StatusTooManyRequests, tooManyRequestsMessage)

	// The window is one hour and has barely started.
	if got, want := rec.Header().Get("Retry-After"), "3600"; got != want {
		t.Errorf("Retry-After = %q, want %q", got, want)
	}
}

func TestRateLimit_RetryAfterRoundsUp(t *testing.T) {
	tests := []struct {
		name       string
		resetAfter time.Duration
		want       string
	}{
		{name: "one nanosecond", resetAfter: time.Nanosecond, want: "1"},
		{name: "under one second", resetAfter: 300 * time.Millisecond, want: "1"},
		{name: "exactly one second", resetAfter: time.Second, want: "1"},
		{name: "just over one second", resetAfter: time.Second + time.Nanosecond, want: "2"},
		{name: "fractional seconds", resetAfter: 59*time.Second + 500*time.Millisecond, want: "60"},
		{name: "whole minutes", resetAfter: 10 * time.Minute, want: "600"},
		// The limiter never returns these for a rejection, but a rejected
		// client must still be told to wait.
		{name: "zero", resetAfter: 0, want: "1"},
		{name: "negative", resetAfter: -time.Second, want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &fakeLimiter{decision: ratelimit.Decision{ResetAfter: tt.resetAfter}}
			next := &recordingHandler{}

			rec := serveRateLimited(limiter, nil, next, "192.0.2.1:1234")

			if next.called {
				t.Error("next handler was called for a rejected request")
			}

			assertJSONError(t, rec, http.StatusTooManyRequests, tooManyRequestsMessage)

			if got := rec.Header().Get("Retry-After"); got != tt.want {
				t.Errorf("Retry-After = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimit_DifferentIPsHaveIndependentCounters(t *testing.T) {
	limiter := newRealLimiter(t, 1)

	if rec := serveRateLimited(limiter, nil, &recordingHandler{}, "192.0.2.1:1234"); rec.Code != http.StatusCreated {
		t.Fatalf("first IP: status = %d, want %d", rec.Code, http.StatusCreated)
	}

	if rec := serveRateLimited(limiter, nil, &recordingHandler{}, "192.0.2.1:1234"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("first IP again: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Another client, and the same client on another port, are counted
	// separately and by address only.
	next := &recordingHandler{}

	rec := serveRateLimited(limiter, nil, next, "198.51.100.2:1234")

	if !next.called || rec.Code != http.StatusCreated {
		t.Errorf("second IP: status = %d, called = %v, want %d and called", rec.Code, next.called, http.StatusCreated)
	}

	if rec := serveRateLimited(limiter, nil, &recordingHandler{}, "192.0.2.1:9999"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("first IP on another port: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_UntrustedPeerCannotSpoofXForwardedFor(t *testing.T) {
	limiter := newRealLimiter(t, 1)

	serveRateLimited(limiter, trustedProxies, &recordingHandler{}, "192.0.2.1:1234", "203.0.113.1")

	// A new X-Forwarded-For value per request does not give the untrusted
	// peer a fresh counter.
	for _, spoofed := range []string{"203.0.113.2", "203.0.113.3, 10.0.0.1"} {
		next := &recordingHandler{}

		rec := serveRateLimited(limiter, trustedProxies, next, "192.0.2.1:1234", spoofed)

		if next.called {
			t.Errorf("X-Forwarded-For %q: next handler was called", spoofed)
		}

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("X-Forwarded-For %q: status = %d, want %d", spoofed, rec.Code, http.StatusTooManyRequests)
		}
	}

	// The key is the peer address, never the header.
	fake := &fakeLimiter{decision: ratelimit.Decision{Allowed: true}}
	serveRateLimited(fake, trustedProxies, &recordingHandler{}, "192.0.2.1:1234", "203.0.113.9")

	if want := []string{"192.0.2.1"}; !slices.Equal(fake.keys, want) {
		t.Errorf("keys = %q, want %q", fake.keys, want)
	}
}

func TestRateLimit_TrustedProxyUsesForwardedClient(t *testing.T) {
	t.Run("key follows ClientIP", func(t *testing.T) {
		tests := []struct {
			name       string
			remoteAddr string
			xff        []string
		}{
			{name: "single trusted hop", remoteAddr: "10.0.0.1:443", xff: []string{"203.0.113.7"}},
			{name: "spoofed entry left of client", remoteAddr: "10.0.0.1:443", xff: []string{"198.51.100.1, 203.0.113.7"}},
			{name: "chain of trusted proxies", remoteAddr: "10.0.0.1:443", xff: []string{"203.0.113.7, 10.0.0.2"}},
			{name: "IPv6 proxy", remoteAddr: "[2001:db8:ffff::1]:443", xff: []string{"203.0.113.7"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				fake := &fakeLimiter{decision: ratelimit.Decision{Allowed: true}}

				serveRateLimited(fake, trustedProxies, &recordingHandler{}, tt.remoteAddr, tt.xff...)

				req := newClientIPRequest(tt.remoteAddr, tt.xff, "")
				want := []string{ClientIP(req, trustedProxies).String()}

				if !slices.Equal(fake.keys, want) || want[0] != "203.0.113.7" {
					t.Errorf("keys = %q, want %q (ClientIP)", fake.keys, want)
				}
			})
		}
	})

	t.Run("clients behind one proxy are counted separately", func(t *testing.T) {
		limiter := newRealLimiter(t, 1)

		serveRateLimited(limiter, trustedProxies, &recordingHandler{}, "10.0.0.1:443", "203.0.113.7")

		if rec := serveRateLimited(limiter, trustedProxies, &recordingHandler{}, "10.0.0.1:443", "203.0.113.7"); rec.Code != http.StatusTooManyRequests {
			t.Errorf("same client: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}

		if rec := serveRateLimited(limiter, trustedProxies, &recordingHandler{}, "10.0.0.1:443", "203.0.113.8"); rec.Code != http.StatusCreated {
			t.Errorf("other client: status = %d, want %d", rec.Code, http.StatusCreated)
		}
	})
}

func TestRateLimit_InvalidClientIPIsRejectedWithoutSharedKey(t *testing.T) {
	remoteAddrs := []string{"", "not-an-ip", "@", "192.0.2.1:1234:5678"}

	t.Run("limiter is not called", func(t *testing.T) {
		for _, remoteAddr := range remoteAddrs {
			fake := &fakeLimiter{decision: ratelimit.Decision{Allowed: true}}
			next := &recordingHandler{}

			// X-Forwarded-For is never read when RemoteAddr is invalid.
			rec := serveRateLimited(fake, trustedProxies, next, remoteAddr, "203.0.113.7")

			if next.called {
				t.Errorf("RemoteAddr %q: next handler was called", remoteAddr)
			}

			if len(fake.keys) != 0 {
				t.Errorf("RemoteAddr %q: limiter called with keys %q, want none", remoteAddr, fake.keys)
			}

			assertJSONError(t, rec, http.StatusInternalServerError, "failed to identify client")
		}
	})

	t.Run("no shared bucket is consumed", func(t *testing.T) {
		limiter := newRealLimiter(t, 1)

		for _, remoteAddr := range remoteAddrs {
			serveRateLimited(limiter, nil, &recordingHandler{}, remoteAddr)
		}

		// Had the invalid requests been keyed as "invalid IP", this key
		// would already be exhausted.
		if d, err := limiter.Allow(netip.Addr{}.String()); err != nil || !d.Allowed {
			t.Errorf("Allow(%q) = %+v, %v; want allowed", netip.Addr{}.String(), d, err)
		}
	})
}

func TestRateLimit_LimiterErrorFailsClosed(t *testing.T) {
	fake := &fakeLimiter{err: errors.New("fake limiter error")}
	next := &recordingHandler{}

	rec := serveRateLimited(fake, nil, next, "192.0.2.1:1234")

	if next.called {
		t.Error("next handler was called after a limiter error")
	}

	assertJSONError(t, rec, http.StatusInternalServerError, "failed to identify client")
}

func TestRateLimit_RejectionCarriesCORSHeaders(t *testing.T) {
	// main wraps the router in CORS, so the frontend can read the 429 body.
	limiter := newRealLimiter(t, 1)
	handler := CORS(wantAllowOrigin, RateLimit(limiter, nil)(&recordingHandler{}))

	send := func(method string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/tickets", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Origin", wantAllowOrigin)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	send(http.MethodPost)

	rec := send(http.MethodPost)

	assertJSONError(t, rec, http.StatusTooManyRequests, tooManyRequestsMessage)
	assertCORSHeaders(t, rec, wantAllowOrigin)

	// Preflights are answered by CORS and never reach the limiter.
	if rec := send(http.MethodOptions); rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
