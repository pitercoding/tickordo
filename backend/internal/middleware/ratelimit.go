package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/pitercoding/tickordo/internal/ratelimit"
)

// Limiter is the part of *ratelimit.Limiter that RateLimit uses. Tests pass
// a fake so they can return any Decision without waiting for a window.
type Limiter interface {
	Allow(key string) (ratelimit.Decision, error)
}

// The real limiter must keep satisfying Limiter.
var _ Limiter = (*ratelimit.Limiter)(nil)

// RateLimit returns a middleware that admits each client IP, resolved by
// ClientIP with the trusted proxies, according to limiter. A rejected request
// gets a 429 with a JSON error and a Retry-After header, and next is not
// called.
//
// When the client IP cannot be resolved, the request is refused with a 500
// instead of being counted: keying it as "invalid IP" would put every such
// request in one shared bucket, and letting it through would bypass the limit.
func RateLimit(limiter Limiter, trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, trusted)
			if !ip.IsValid() {
				log.Printf("rate limit: cannot resolve client IP from RemoteAddr %q", r.RemoteAddr)
				writeJSONError(w, "failed to identify client", http.StatusInternalServerError)
				return
			}

			decision, err := limiter.Allow(ip.String())
			if err != nil {
				log.Printf("rate limit: %v", err)
				writeJSONError(w, "failed to identify client", http.StatusInternalServerError)
				return
			}

			if !decision.Allowed {
				w.Header().Set("Retry-After", retryAfterSeconds(decision.ResetAfter))
				writeJSONError(w, "too many requests, please try again later", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// retryAfterSeconds rounds d up to whole seconds, with a minimum of one: a
// rejected client told to retry after 0 seconds would retry immediately and
// be rejected again.
func retryAfterSeconds(d time.Duration) string {
	seconds := (d + time.Second - 1) / time.Second
	if seconds < 1 {
		seconds = 1
	}

	return strconv.FormatInt(int64(seconds), 10)
}

// writeJSONError sends {"error": message} with status, the same body the
// handlers write, so the frontend reads middleware errors the same way.
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status code is already sent, so an encoding error can only be logged.
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("failed to write error response: %v", err)
	}
}
