package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/middleware"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/ratelimit"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// triageTarget is a POST /tickets/{id}/triage path that gets past the
// handler's ID check.
const triageTarget = "/tickets/" + validTicketID + "/triage"

const tooManyRequestsMessage = "too many requests, please try again later"

// countingTicketTriageService counts AnalyzeTicket calls, so a test can tell
// whether a request reached the service (and so the AI analyzer). It answers
// with err when set, and with a new triage otherwise. The read methods fail
// like failingTicketTriageService.
type countingTicketTriageService struct {
	failingTicketTriageService

	err   error
	calls int
}

func (s *countingTicketTriageService) AnalyzeTicket(_ context.Context, ticketID uuid.UUID) (*models.TicketTriage, error) {
	s.calls++

	if s.err != nil {
		return nil, s.err
	}

	return &models.TicketTriage{ID: uuid.New(), TicketID: ticketID}, nil
}

// newLimit returns middleware.RateLimit over a new limiter of limit requests
// per hour, a window that never expires during a test.
func newLimit(t *testing.T, limit int, trusted []netip.Prefix) func(http.Handler) http.Handler {
	t.Helper()

	limiter, err := ratelimit.New(limit, time.Hour)
	if err != nil {
		t.Fatalf("ratelimit.New returned error: %v", err)
	}

	return middleware.RateLimit(limiter, trusted)
}

// newRateLimitedMux builds the router like main, with its own limiter per
// route: createLimit for POST /tickets and triageLimit for
// POST /tickets/{id}/triage, neither trusting any proxy. The other services
// fail, so every handler answers without a database or OpenAI.
func newRateLimitedMux(t *testing.T, createLimit, triageLimit int, triage *countingTicketTriageService) *http.ServeMux {
	t.Helper()

	return newRateLimitedMuxWith(t, RateLimits{
		CreateTicket: newLimit(t, createLimit, nil),
		TriageTicket: newLimit(t, triageLimit, nil),
	}, triage)
}

// newRateLimitedMuxWith is newRateLimitedMux with the middlewares given.
func newRateLimitedMuxWith(t *testing.T, limits RateLimits, triage *countingTicketTriageService) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()

	RegisterRoutes(
		mux,
		handlers.NewTicketHandler(failingTicketService{}),
		handlers.NewTicketTriageHandler(triage),
		handlers.NewDashboardHandler(failingDashboardService{}),
		limits,
	)

	return mux
}

// newClientRequest builds a request from a fixed client address, so every
// request in a test shares one rate limit key.
func newClientRequest(method, target, body string) *http.Request {
	return newRequestFrom("192.0.2.1:1234", method, target, body)
}

// newRequestFrom builds a request from the peer remoteAddr, with optional
// X-Forwarded-For lines.
func newRequestFrom(remoteAddr, method, target, body string, xff ...string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Origin", "http://localhost:3000")

	for _, line := range xff {
		req.Header.Add("X-Forwarded-For", line)
	}

	return req
}

// assertTriageCreated checks that rec is the 201 the triage handler writes
// on success, so the request was admitted and reached the service.
func assertTriageCreated(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

// assertTooManyRequests checks the 429 written by middleware.RateLimit for a
// one-hour window that has barely started.
func assertTooManyRequests(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	assertHandlerError(t, rec, http.StatusTooManyRequests, tooManyRequestsMessage)

	if got, want := rec.Header().Get("Retry-After"), "3600"; got != want {
		t.Errorf("Retry-After = %q, want %q", got, want)
	}
}

// assertAnalyzeCalls checks how many requests reached AnalyzeTicket.
func assertAnalyzeCalls(t *testing.T, triage *countingTicketTriageService, want int) {
	t.Helper()

	if triage.calls != want {
		t.Errorf("AnalyzeTicket calls = %d, want %d", triage.calls, want)
	}
}

func TestRateLimitedRoutes_PostTicketsIsLimited(t *testing.T) {
	handler := middleware.CORS("http://localhost:3000", newRateLimitedMux(t, 1, 1, &countingTicketTriageService{}))

	// Within the limit, the request reaches Create's own validation.
	rec := serveMux(t, handler, newClientRequest(http.MethodPost, "/tickets", `{}`))
	assertHandlerError(t, rec, http.StatusBadRequest, "title and description are required")

	// Over the limit, Create is not reached: its validation error would
	// otherwise be returned again.
	rec = serveMux(t, handler, newClientRequest(http.MethodPost, "/tickets", `{}`))
	assertTooManyRequests(t, rec)
	assertCORSHeaders(t, rec)
}

func TestRateLimitedRoutes_PostTriageIsLimited(t *testing.T) {
	triage := &countingTicketTriageService{}
	handler := middleware.CORS("http://localhost:3000", newRateLimitedMux(t, 1, 2, triage))

	// Within the limit, every request reaches the service.
	for i := range 2 {
		rec := serveMux(t, handler, newClientRequest(http.MethodPost, triageTarget, ""))
		assertTriageCreated(t, rec)
		assertAnalyzeCalls(t, triage, i+1)
	}

	// Over the limit, the request is answered before the handler, so neither
	// the service nor the AI analyzer behind it runs.
	rec := serveMux(t, handler, newClientRequest(http.MethodPost, triageTarget, ""))
	assertTooManyRequests(t, rec)
	assertCORSHeaders(t, rec)
	assertAnalyzeCalls(t, triage, 2)
}

func TestRateLimitedRoutes_TriageIsCountedPerClientIP(t *testing.T) {
	triage := &countingTicketTriageService{}
	mux := newRateLimitedMux(t, 1, 1, triage)

	send := func(remoteAddr string) *httptest.ResponseRecorder {
		return serveMux(t, mux, newRequestFrom(remoteAddr, http.MethodPost, triageTarget, ""))
	}

	assertTriageCreated(t, send("192.0.2.1:1234"))
	assertTooManyRequests(t, send("192.0.2.1:1234"))

	// Another client has its own counter.
	assertTriageCreated(t, send("198.51.100.2:1234"))

	// The key is the address only, so another port shares the quota.
	assertTooManyRequests(t, send("192.0.2.1:9999"))

	assertAnalyzeCalls(t, triage, 2)
}

func TestRateLimitedRoutes_TriageUsesTrustedProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}

	newMux := func(triage *countingTicketTriageService) *http.ServeMux {
		return newRateLimitedMuxWith(t, RateLimits{
			CreateTicket: newLimit(t, 1, trusted),
			TriageTicket: newLimit(t, 1, trusted),
		}, triage)
	}

	t.Run("clients behind a trusted proxy are counted separately", func(t *testing.T) {
		triage := &countingTicketTriageService{}
		mux := newMux(triage)

		send := func(client string) *httptest.ResponseRecorder {
			return serveMux(t, mux, newRequestFrom("10.0.0.1:443", http.MethodPost, triageTarget, "", client))
		}

		assertTriageCreated(t, send("203.0.113.7"))
		assertTooManyRequests(t, send("203.0.113.7"))
		assertTriageCreated(t, send("203.0.113.8"))

		assertAnalyzeCalls(t, triage, 2)
	})

	t.Run("untrusted peer cannot escape the limit with X-Forwarded-For", func(t *testing.T) {
		triage := &countingTicketTriageService{}
		mux := newMux(triage)

		send := func(xff string) *httptest.ResponseRecorder {
			return serveMux(t, mux, newRequestFrom("192.0.2.1:1234", http.MethodPost, triageTarget, "", xff))
		}

		assertTriageCreated(t, send("203.0.113.1"))

		// A new X-Forwarded-For per request, even one ending in a trusted
		// proxy, does not give the peer a fresh counter.
		for _, spoofed := range []string{"203.0.113.2", "203.0.113.3, 10.0.0.1"} {
			assertTooManyRequests(t, send(spoofed))
		}

		assertAnalyzeCalls(t, triage, 1)
	})
}

func TestRateLimitedRoutes_LimitersAreIndependent(t *testing.T) {
	// The limits differ, so a mux whose RateLimits fields were swapped would
	// reject the second triage below (limit 1) or admit a second ticket
	// creation (limit 3).
	const createLimit, triageLimit = 1, 3

	t.Run("exhausted ticket creation does not limit triage", func(t *testing.T) {
		triage := &countingTicketTriageService{}
		mux := newRateLimitedMux(t, createLimit, triageLimit, triage)

		serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`))
		assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`)))

		for range triageLimit {
			assertTriageCreated(t, serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, "")))
		}

		assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, "")))
		assertAnalyzeCalls(t, triage, triageLimit)
	})

	t.Run("exhausted triage does not limit ticket creation", func(t *testing.T) {
		triage := &countingTicketTriageService{}
		mux := newRateLimitedMux(t, createLimit, triageLimit, triage)

		for range triageLimit {
			serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, ""))
		}

		assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, "")))

		// Within its own limit, the request reaches Create's validation.
		rec := serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`))
		assertHandlerError(t, rec, http.StatusBadRequest, "title and description are required")

		assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`)))
	})
}

func TestRateLimitedRoutes_OtherRoutesAreNotLimited(t *testing.T) {
	tests := []routeCase{
		{
			name:        "GET /health",
			method:      http.MethodGet,
			target:      "/health",
			wantStatus:  http.StatusOK,
			wantMessage: "",
		},
		{
			name:        "GET /tickets",
			method:      http.MethodGet,
			target:      "/tickets",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to list tickets",
		},
		{
			name:        "GET /tickets/{id}",
			method:      http.MethodGet,
			target:      "/tickets/" + validTicketID,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to get ticket",
		},
		{
			name:        "PATCH /tickets/{id}",
			method:      http.MethodPatch,
			target:      "/tickets/" + validTicketID,
			body:        `{"status": "resolved"}`,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to update ticket status",
		},
		{
			// Same path as the limited POST, but another pattern.
			name:        "GET /tickets/{id}/triage",
			method:      http.MethodGet,
			target:      triageTarget,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve latest ticket triage",
		},
		{
			name:        "GET /tickets/{id}/triages",
			method:      http.MethodGet,
			target:      "/tickets/" + validTicketID + "/triages",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve ticket triage history",
		},
		{
			name:        "GET /dashboard/stats",
			method:      http.MethodGet,
			target:      "/dashboard/stats",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to get dashboard statistics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := newRateLimitedMux(t, 1, 1, &countingTicketTriageService{})

			// Exhaust both limits for this client first.
			for range 2 {
				serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`))
				serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, ""))
			}

			// Several requests, more than either limit, all reach the handler.
			for range 3 {
				rec := serveMux(t, mux, newClientRequest(tt.method, tt.target, tt.body))

				if tt.wantMessage == "" {
					if rec.Code != tt.wantStatus {
						t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
					}
					continue
				}

				assertHandlerError(t, rec, tt.wantStatus, tt.wantMessage)
			}
		})
	}
}

func TestRateLimitedRoutes_TriagePreflightDoesNotConsumeQuota(t *testing.T) {
	triage := &countingTicketTriageService{}
	handler := middleware.CORS("http://localhost:3000", newRateLimitedMux(t, 1, 1, triage))

	// CORS answers preflights before the router, so they never reach the
	// limiter, however many there are.
	for range 3 {
		req := newClientRequest(http.MethodOptions, triageTarget, "")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)

		rec := serveMux(t, handler, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("preflight status = %d, want %d", rec.Code, http.StatusNoContent)
		}

		assertCORSHeaders(t, rec)
	}

	// The single allowed triage is still available.
	assertTriageCreated(t, serveMux(t, handler, newClientRequest(http.MethodPost, triageTarget, "")))

	rec := serveMux(t, handler, newClientRequest(http.MethodPost, triageTarget, ""))
	assertTooManyRequests(t, rec)
	assertCORSHeaders(t, rec)

	assertAnalyzeCalls(t, triage, 1)
}

func TestRateLimitedRoutes_TriageLimitRunsBeforeHandlerValidation(t *testing.T) {
	// The limiter runs before the handler, so every attempt counts, and once
	// the quota is spent the 429 replaces the handler's own errors.
	tests := []struct {
		name        string
		target      string
		body        string
		serviceErr  error
		wantStatus  int
		wantMessage string
		wantCalls   int
	}{
		{
			name:        "invalid ticket ID",
			target:      "/tickets/not-a-uuid/triage",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
		{
			name:        "non-empty body",
			target:      triageTarget,
			body:        `{}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "request body must be empty",
		},
		{
			// Wrapped like TicketTriageService does.
			name:        "unknown ticket",
			target:      triageTarget,
			serviceErr:  fmt.Errorf("failed to get ticket: %w", repositories.ErrTicketNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
			wantCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triage := &countingTicketTriageService{err: tt.serviceErr}
			mux := newRateLimitedMux(t, 1, 1, triage)

			rec := serveMux(t, mux, newClientRequest(http.MethodPost, tt.target, tt.body))
			assertHandlerError(t, rec, tt.wantStatus, tt.wantMessage)

			// The failed attempt spent the quota.
			assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, tt.target, tt.body)))
			assertTooManyRequests(t, serveMux(t, mux, newClientRequest(http.MethodPost, triageTarget, "")))

			assertAnalyzeCalls(t, triage, tt.wantCalls)
		})
	}
}
