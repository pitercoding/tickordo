package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pitercoding/tickordo/internal/ai"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/ratelimit"
)

var (
	// ErrTriageBusy is returned when every analysis slot is in use.
	ErrTriageBusy = errors.New("triage service is busy")

	// ErrTriageQuotaExceeded is wrapped by QuotaExceededError, so callers
	// can use errors.Is when they do not need the wait.
	ErrTriageQuotaExceeded = errors.New("triage quota exceeded")
)

// QuotaExceededError is returned when the global triage quota is used up.
type QuotaExceededError struct {
	// RetryAfter is how long until the quota window expires.
	RetryAfter time.Duration
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%v: window resets in %s", ErrTriageQuotaExceeded, e.RetryAfter)
}

func (e *QuotaExceededError) Unwrap() error {
	return ErrTriageQuotaExceeded
}

// globalQuota lists the method LimitedAnalyzer calls on the global quota.
// Production passes *ratelimit.Global; tests pass a fake to count calls.
type globalQuota interface {
	Allow() ratelimit.Decision
}

// The real implementations must keep satisfying these interfaces.
var (
	_ globalQuota       = (*ratelimit.Global)(nil)
	_ ai.TicketAnalyzer = (*LimitedAnalyzer)(nil)
)

// LimitedAnalyzer wraps a TicketAnalyzer so at most maxConcurrent analyses
// run at once and each one first takes a unit of the global quota.
//
// A request first takes a slot, without waiting: when all are in use it gets
// ErrTriageBusy and the quota is left untouched. It then takes from the
// quota, and gets a *QuotaExceededError without calling the analyzer when
// none is left. Once the analyzer has been called the quota is not given
// back, even if the call fails, since OpenAI may already have billed it.
//
// Slots and quota live in memory and are per process.
type LimitedAnalyzer struct {
	next  ai.TicketAnalyzer
	quota globalQuota
	slots chan struct{}
}

// NewLimitedAnalyzer returns a LimitedAnalyzer around next. It returns an
// error when next or quota is nil or maxConcurrent is not positive.
func NewLimitedAnalyzer(
	next ai.TicketAnalyzer,
	quota globalQuota,
	maxConcurrent int,
) (*LimitedAnalyzer, error) {
	if next == nil {
		return nil, fmt.Errorf("ticket analyzer cannot be nil")
	}

	if quota == nil {
		return nil, fmt.Errorf("triage quota cannot be nil")
	}

	if maxConcurrent <= 0 {
		return nil, fmt.Errorf("invalid max concurrent triages: must be a positive integer, got %d", maxConcurrent)
	}

	return &LimitedAnalyzer{
		next:  next,
		quota: quota,
		slots: make(chan struct{}, maxConcurrent),
	}, nil
}

func (a *LimitedAnalyzer) Analyze(
	ctx context.Context,
	ticket *models.Ticket,
) (*models.TicketTriage, error) {
	// Take a slot without waiting: an analysis can take up to a minute, so
	// a waiting request would hold its connection for nothing.
	select {
	case a.slots <- struct{}{}:
	default:
		return nil, ErrTriageBusy
	}

	// Deferred so the slot is freed on every return, and during a panic.
	defer func() { <-a.slots }()

	// A request whose client already left should not use up the quota.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if decision := a.quota.Allow(); !decision.Allowed {
		return nil, &QuotaExceededError{RetryAfter: decision.ResetAfter}
	}

	return a.next.Analyze(ctx, ticket)
}
