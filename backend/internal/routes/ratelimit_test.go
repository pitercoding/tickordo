package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/middleware"
	"github.com/pitercoding/tickordo/internal/ratelimit"
)

// newRateLimitedMux builds the router like main, with a ticket creation
// limiter of limit 1, and the failing services so every handler answers
// without a database or OpenAI.
func newRateLimitedMux(t *testing.T) *http.ServeMux {
	t.Helper()

	limiter, err := ratelimit.New(1, time.Hour)
	if err != nil {
		t.Fatalf("ratelimit.New returned error: %v", err)
	}

	mux := http.NewServeMux()

	RegisterRoutes(
		mux,
		handlers.NewTicketHandler(failingTicketService{}),
		handlers.NewTicketTriageHandler(failingTicketTriageService{}),
		handlers.NewDashboardHandler(failingDashboardService{}),
		middleware.RateLimit(limiter, nil),
	)

	return mux
}

// newClientRequest builds a request from a fixed client address, so every
// request in a test shares one rate limit key.
func newClientRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Origin", "http://localhost:3000")

	return req
}

func TestRateLimitedRoutes_PostTicketsIsLimited(t *testing.T) {
	handler := middleware.CORS("http://localhost:3000", newRateLimitedMux(t))

	// Within the limit, the request reaches Create's own validation.
	rec := serveMux(t, handler, newClientRequest(http.MethodPost, "/tickets", `{}`))
	assertHandlerError(t, rec, http.StatusBadRequest, "title and description are required")

	// Over the limit, Create is not reached: its validation error would
	// otherwise be returned again.
	rec = serveMux(t, handler, newClientRequest(http.MethodPost, "/tickets", `{}`))
	assertHandlerError(t, rec, http.StatusTooManyRequests, "too many requests, please try again later")
	assertCORSHeaders(t, rec)

	if got, want := rec.Header().Get("Retry-After"), "3600"; got != want {
		t.Errorf("Retry-After = %q, want %q", got, want)
	}
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
			name:        "POST /tickets/{id}/triage",
			method:      http.MethodPost,
			target:      "/tickets/" + validTicketID + "/triage",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to triage ticket",
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
			mux := newRateLimitedMux(t)

			// Exhaust the ticket creation limit for this client first.
			for range 2 {
				serveMux(t, mux, newClientRequest(http.MethodPost, "/tickets", `{}`))
			}

			// Several requests, more than the limit, all reach the handler.
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
