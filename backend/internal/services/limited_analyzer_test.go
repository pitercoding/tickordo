package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/ai"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/ratelimit"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// waitTimeout only guards against a hung test; no test relies on it passing.
const waitTimeout = 5 * time.Second

var limitedTestTicket = &models.Ticket{ID: uuid.New(), Title: "Login fails", Description: "I cannot sign in."}

// fakeQuota stands in for *ratelimit.Global. It counts calls and returns
// decision. It is safe for concurrent use.
type fakeQuota struct {
	mu       sync.Mutex
	decision ratelimit.Decision
	calls    int
}

func (q *fakeQuota) Allow() ratelimit.Decision {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.calls++

	return q.decision
}

func (q *fakeQuota) callCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.calls
}

// allowingQuota returns a fakeQuota that admits every request.
func allowingQuota() *fakeQuota {
	return &fakeQuota{decision: ratelimit.Decision{Allowed: true}}
}

// analyzerFunc adapts a function to ai.TicketAnalyzer.
type analyzerFunc func(ctx context.Context, ticket *models.Ticket) (*models.TicketTriage, error)

func (f analyzerFunc) Analyze(ctx context.Context, ticket *models.Ticket) (*models.TicketTriage, error) {
	return f(ctx, ticket)
}

// blockingAnalyzer holds every call until a value is sent on release, or
// until its context is done. Each call signals started once it is counted
// as in flight, so a test can wait for calls to be running without sleeping.
type blockingAnalyzer struct {
	started chan struct{}
	release chan struct{}

	calls       atomic.Int32
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
}

func newBlockingAnalyzer() *blockingAnalyzer {
	return &blockingAnalyzer{
		started: make(chan struct{}, 100),
		release: make(chan struct{}),
	}
}

func (b *blockingAnalyzer) Analyze(ctx context.Context, ticket *models.Ticket) (*models.TicketTriage, error) {
	b.calls.Add(1)

	n := b.inFlight.Add(1)
	defer b.inFlight.Add(-1)

	for {
		highest := b.maxInFlight.Load()
		if n <= highest || b.maxInFlight.CompareAndSwap(highest, n) {
			break
		}
	}

	b.started <- struct{}{}

	select {
	case <-b.release:
		return &models.TicketTriage{TicketID: ticket.ID}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// waitStarted waits until n more calls to b are running.
func (b *blockingAnalyzer) waitStarted(t *testing.T, n int) {
	t.Helper()

	for i := range n {
		select {
		case <-b.started:
		case <-time.After(waitTimeout):
			t.Fatalf("only %d of %d analyses started", i, n)
		}
	}
}

type analyzeResult struct {
	triage *models.TicketTriage
	err    error
}

// analyzeAsync calls a.Analyze in a new goroutine and sends the result to
// results. The slot is freed before the result is sent.
func analyzeAsync(ctx context.Context, a *LimitedAnalyzer, results chan<- analyzeResult) {
	go func() {
		triage, err := a.Analyze(ctx, limitedTestTicket)
		results <- analyzeResult{triage: triage, err: err}
	}()
}

func receiveResult(t *testing.T, results <-chan analyzeResult) analyzeResult {
	t.Helper()

	select {
	case r := <-results:
		return r
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting for an analysis to return")
		return analyzeResult{}
	}
}

func newTestLimitedAnalyzer(t *testing.T, next ai.TicketAnalyzer, quota globalQuota, maxConcurrent int) *LimitedAnalyzer {
	t.Helper()

	a, err := NewLimitedAnalyzer(next, quota, maxConcurrent)
	if err != nil {
		t.Fatalf("NewLimitedAnalyzer returned error: %v", err)
	}

	return a
}

// assertSlotAvailable checks that no slot is held and that a new request is
// not turned away as busy. The probe uses a canceled context, so it reaches
// neither the quota nor the analyzer.
func assertSlotAvailable(t *testing.T, a *LimitedAnalyzer) {
	t.Helper()

	if held := len(a.slots); held != 0 {
		t.Fatalf("slots held = %d, want 0", held)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := a.Analyze(ctx, limitedTestTicket); !errors.Is(err, context.Canceled) {
		t.Fatalf("probe error = %v, want %v", err, context.Canceled)
	}
}

func TestNewLimitedAnalyzer_RejectsInvalidConfig(t *testing.T) {
	analyzer := &fakeTicketAnalyzer{}
	quota := allowingQuota()

	tests := []struct {
		name          string
		next          ai.TicketAnalyzer
		quota         globalQuota
		maxConcurrent int
	}{
		{name: "nil analyzer", next: nil, quota: quota, maxConcurrent: 3},
		{name: "nil quota", next: analyzer, quota: nil, maxConcurrent: 3},
		{name: "zero concurrency", next: analyzer, quota: quota, maxConcurrent: 0},
		{name: "negative concurrency", next: analyzer, quota: quota, maxConcurrent: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := NewLimitedAnalyzer(tt.next, tt.quota, tt.maxConcurrent)
			if err == nil {
				t.Fatal("NewLimitedAnalyzer returned nil error")
			}

			if a != nil {
				t.Fatalf("NewLimitedAnalyzer returned analyzer %+v with error", a)
			}
		})
	}
}

func TestLimitedAnalyzer_PassesThroughResult(t *testing.T) {
	want := &models.TicketTriage{TicketID: limitedTestTicket.ID, Category: "authentication"}
	analyzer := &fakeTicketAnalyzer{triage: want}
	quota := allowingQuota()
	a := newTestLimitedAnalyzer(t, analyzer, quota, 3)

	got, err := a.Analyze(context.Background(), limitedTestTicket)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if got != want {
		t.Errorf("triage = %+v, want %+v", got, want)
	}

	if analyzer.gotTicket != limitedTestTicket {
		t.Errorf("analyzer got ticket %+v, want %+v", analyzer.gotTicket, limitedTestTicket)
	}

	if quota.callCount() != 1 {
		t.Errorf("quota calls = %d, want 1", quota.callCount())
	}

	assertSlotAvailable(t, a)
}

func TestLimitedAnalyzer_RejectsWhenAllSlotsBusy(t *testing.T) {
	analyzer := newBlockingAnalyzer()
	quota := allowingQuota()
	a := newTestLimitedAnalyzer(t, analyzer, quota, 3)
	results := make(chan analyzeResult, 10)

	for range 3 {
		analyzeAsync(context.Background(), a, results)
	}

	analyzer.waitStarted(t, 3)

	// A fourth analysis is turned away at once, without waiting for a slot.
	if _, err := a.Analyze(context.Background(), limitedTestTicket); !errors.Is(err, ErrTriageBusy) {
		t.Fatalf("fourth analysis error = %v, want %v", err, ErrTriageBusy)
	}

	if got := analyzer.calls.Load(); got != 3 {
		t.Errorf("analyzer calls = %d, want 3", got)
	}

	// The rejected request did not touch the quota.
	if got := quota.callCount(); got != 3 {
		t.Errorf("quota calls = %d, want 3", got)
	}

	// Finishing one analysis frees its slot for a new one.
	analyzer.release <- struct{}{}

	if r := receiveResult(t, results); r.err != nil {
		t.Fatalf("released analysis returned error: %v", r.err)
	}

	analyzeAsync(context.Background(), a, results)
	analyzer.waitStarted(t, 1)

	for range 3 {
		analyzer.release <- struct{}{}

		if r := receiveResult(t, results); r.err != nil {
			t.Fatalf("analysis returned error: %v", r.err)
		}
	}

	if got := quota.callCount(); got != 4 {
		t.Errorf("quota calls = %d, want 4", got)
	}

	assertSlotAvailable(t, a)
}

func TestLimitedAnalyzer_NeverExceedsMaxConcurrent(t *testing.T) {
	const (
		maxConcurrent = 3
		requests      = 50
		rounds        = 3
	)

	analyzer := newBlockingAnalyzer()
	quota := allowingQuota()
	a := newTestLimitedAnalyzer(t, analyzer, quota, maxConcurrent)

	// Each round shows the slots of the previous one were all freed.
	for round := range rounds {
		results := make(chan analyzeResult, requests)

		for range requests {
			analyzeAsync(context.Background(), a, results)
		}

		// The admitted analyses block until released, so every other
		// request must come back busy first.
		for range requests - maxConcurrent {
			if r := receiveResult(t, results); !errors.Is(r.err, ErrTriageBusy) {
				t.Fatalf("round %d: error = %v, want %v", round, r.err, ErrTriageBusy)
			}
		}

		analyzer.waitStarted(t, maxConcurrent)

		for range maxConcurrent {
			analyzer.release <- struct{}{}

			if r := receiveResult(t, results); r.err != nil {
				t.Fatalf("round %d: admitted analysis returned error: %v", round, r.err)
			}
		}
	}

	if got := analyzer.maxInFlight.Load(); got != maxConcurrent {
		t.Errorf("max analyses in flight = %d, want %d", got, maxConcurrent)
	}

	if got := analyzer.calls.Load(); got != maxConcurrent*rounds {
		t.Errorf("analyzer calls = %d, want %d", got, maxConcurrent*rounds)
	}

	if got := quota.callCount(); got != maxConcurrent*rounds {
		t.Errorf("quota calls = %d, want %d", got, maxConcurrent*rounds)
	}

	assertSlotAvailable(t, a)
}

func TestLimitedAnalyzer_FreesSlotOnFailure(t *testing.T) {
	t.Run("analyzer error", func(t *testing.T) {
		analyzerErr := errors.New("openai timeout")
		a := newTestLimitedAnalyzer(t, &fakeTicketAnalyzer{err: analyzerErr}, allowingQuota(), 1)

		if _, err := a.Analyze(context.Background(), limitedTestTicket); !errors.Is(err, analyzerErr) {
			t.Fatalf("error = %v, want %v", err, analyzerErr)
		}

		assertSlotAvailable(t, a)
	})

	t.Run("context canceled during analysis", func(t *testing.T) {
		analyzer := newBlockingAnalyzer()
		a := newTestLimitedAnalyzer(t, analyzer, allowingQuota(), 1)
		results := make(chan analyzeResult, 1)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		analyzeAsync(ctx, a, results)
		analyzer.waitStarted(t, 1)

		cancel()

		if r := receiveResult(t, results); !errors.Is(r.err, context.Canceled) {
			t.Fatalf("error = %v, want %v", r.err, context.Canceled)
		}

		assertSlotAvailable(t, a)
	})

	t.Run("analyzer panic", func(t *testing.T) {
		analyzer := analyzerFunc(func(context.Context, *models.Ticket) (*models.TicketTriage, error) {
			panic("analyzer exploded")
		})
		a := newTestLimitedAnalyzer(t, analyzer, allowingQuota(), 1)

		func() {
			defer func() {
				if p := recover(); p == nil {
					t.Fatal("Analyze did not propagate the analyzer panic")
				}
			}()

			_, _ = a.Analyze(context.Background(), limitedTestTicket)
		}()

		assertSlotAvailable(t, a)
	})
}

func TestLimitedAnalyzer_CanceledContextDoesNotConsumeQuota(t *testing.T) {
	analyzer := &fakeTicketAnalyzer{}
	quota := allowingQuota()
	a := newTestLimitedAnalyzer(t, analyzer, quota, 3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := a.Analyze(ctx, limitedTestTicket); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}

	if quota.callCount() != 0 {
		t.Errorf("quota calls = %d, want 0", quota.callCount())
	}

	if analyzer.calls != 0 {
		t.Errorf("analyzer calls = %d, want 0", analyzer.calls)
	}
}

func TestLimitedAnalyzer_QuotaExceededSkipsAnalyzer(t *testing.T) {
	analyzer := &fakeTicketAnalyzer{}
	quota := &fakeQuota{decision: ratelimit.Decision{Allowed: false, ResetAfter: 90 * time.Minute}}
	a := newTestLimitedAnalyzer(t, analyzer, quota, 3)

	triage, err := a.Analyze(context.Background(), limitedTestTicket)

	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("error = %v, want a *QuotaExceededError", err)
	}

	if quotaErr.RetryAfter != 90*time.Minute {
		t.Errorf("RetryAfter = %s, want %s", quotaErr.RetryAfter, 90*time.Minute)
	}

	if !errors.Is(err, ErrTriageQuotaExceeded) {
		t.Errorf("error = %v, want it to wrap %v", err, ErrTriageQuotaExceeded)
	}

	if triage != nil {
		t.Errorf("triage = %+v, want nil", triage)
	}

	if analyzer.calls != 0 {
		t.Errorf("analyzer calls = %d, want 0", analyzer.calls)
	}

	assertSlotAvailable(t, a)
}

func TestLimitedAnalyzer_AnalyzerErrorKeepsQuotaUsed(t *testing.T) {
	analyzerErr := errors.New("openai timeout")
	analyzer := &fakeTicketAnalyzer{err: analyzerErr}

	quota, err := ratelimit.NewGlobal(1, time.Hour)
	if err != nil {
		t.Fatalf("NewGlobal returned error: %v", err)
	}

	a := newTestLimitedAnalyzer(t, analyzer, quota, 3)

	if _, err := a.Analyze(context.Background(), limitedTestTicket); !errors.Is(err, analyzerErr) {
		t.Fatalf("first error = %v, want %v", err, analyzerErr)
	}

	// The failed call still used the only unit of quota.
	if _, err := a.Analyze(context.Background(), limitedTestTicket); !errors.Is(err, ErrTriageQuotaExceeded) {
		t.Fatalf("second error = %v, want %v", err, ErrTriageQuotaExceeded)
	}

	if analyzer.calls != 1 {
		t.Errorf("analyzer calls = %d, want 1", analyzer.calls)
	}
}

func TestTicketTriageService_WithLimitedAnalyzer(t *testing.T) {
	t.Run("unknown ticket does not consume quota", func(t *testing.T) {
		quota := allowingQuota()
		a := newTestLimitedAnalyzer(t, &fakeTicketAnalyzer{}, quota, 3)
		tickets := &fakeTicketRepository{err: repositories.ErrTicketNotFound}
		service := NewTicketTriageService(tickets, &fakeTicketTriageRepository{}, a)

		if _, err := service.AnalyzeTicket(context.Background(), uuid.New()); !errors.Is(err, repositories.ErrTicketNotFound) {
			t.Fatalf("error = %v, want %v", err, repositories.ErrTicketNotFound)
		}

		if quota.callCount() != 0 {
			t.Errorf("quota calls = %d, want 0", quota.callCount())
		}
	})

	t.Run("quota error reaches the caller", func(t *testing.T) {
		quota := &fakeQuota{decision: ratelimit.Decision{Allowed: false, ResetAfter: time.Minute}}
		a := newTestLimitedAnalyzer(t, &fakeTicketAnalyzer{}, quota, 3)
		tickets := &fakeTicketRepository{ticket: limitedTestTicket}
		triages := &fakeTicketTriageRepository{}
		service := NewTicketTriageService(tickets, triages, a)

		_, err := service.AnalyzeTicket(context.Background(), limitedTestTicket.ID)

		var quotaErr *QuotaExceededError
		if !errors.As(err, &quotaErr) || quotaErr.RetryAfter != time.Minute {
			t.Fatalf("error = %v, want a *QuotaExceededError with RetryAfter %s", err, time.Minute)
		}

		if triages.createCalls != 0 {
			t.Errorf("triage Create calls = %d, want 0", triages.createCalls)
		}
	})
}
