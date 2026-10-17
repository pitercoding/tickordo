package ratelimit

import "time"

// globalKey is the single key a Global counts under. It never leaves this
// file, so callers cannot collide with it or need to know it exists.
const globalKey = "global"

// Global admits up to limit requests within each window across all callers,
// instead of per key. It is a Limiter with one fixed key, so it has the same
// semantics: the window starts at the first admitted request, rejected
// requests are not counted and do not extend it, and the next request after
// it expires starts a new window. It is safe for concurrent use.
//
// State lives in memory and is per process: it is lost on restart, and each
// replica has its own count, so N replicas admit up to N*limit requests.
// There is no way to give back an admitted request.
type Global struct {
	limiter *Limiter
}

// NewGlobal returns a Global that admits up to limit requests within each
// window. Both must be positive, as for New.
func NewGlobal(limit int, window time.Duration) (*Global, error) {
	limiter, err := New(limit, window)
	if err != nil {
		return nil, err
	}

	return &Global{limiter: limiter}, nil
}

// Allow records a request and reports whether it is within the limit.
func (g *Global) Allow() Decision {
	// The only error Limiter.Allow returns is ErrEmptyKey, and globalKey is
	// not empty.
	decision, _ := g.limiter.Allow(globalKey)

	return decision
}
