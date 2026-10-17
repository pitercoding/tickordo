package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/models"
)

// validTicketID is a well-formed UUID, used when a request must get past the
// handler's ID check.
const validTicketID = "550e8400-e29b-41d4-a716-446655440000"

// noRateLimit leaves a route unlimited, for tests about routing only.
func noRateLimit(next http.Handler) http.Handler {
	return next
}

// noRateLimits leaves every rate limited route unlimited.
var noRateLimits = RateLimits{
	CreateTicket: noRateLimit,
	TriageTicket: noRateLimit,
}

// newTestMux builds the router exactly as main does, but with nil services:
// the requests sent to it are rejected before the service layer, so no
// database or OpenAI client is needed.
func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()

	RegisterRoutes(
		mux,
		handlers.NewTicketHandler(nil),
		handlers.NewTicketTriageHandler(nil),
		handlers.NewDashboardHandler(nil),
		noRateLimits,
	)

	return mux
}

func TestRegisterRoutes_GetHealthReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	newTestMux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	// json.Encoder ends its output with a newline, so this is the exact
	// body a client receives.
	if got, want := rec.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRegisterRoutes_PostHealthReturnsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	newTestMux().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	// ServeMux sets Allow only when the path matches a pattern registered for
	// other methods, so this proves the 405 comes from the "GET /health"
	// route and not from a missing route. GET patterns also match HEAD.
	if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Errorf("Allow = %q, want %q", got, want)
	}
}

// errFakeService is returned by every method of the failing fakes below.
var errFakeService = errors.New("fake service error")

// failingTicketService, failingTicketTriageService and failingDashboardService
// satisfy the handlers' service interfaces without a database or OpenAI.
// Every method fails, so each handler answers with its own 500 message, which
// tells exactly which handler a route reached.
type failingTicketService struct{}

func (failingTicketService) CreateTicket(context.Context, string, string) (*models.Ticket, error) {
	return nil, errFakeService
}

func (failingTicketService) ListTickets(context.Context) ([]models.Ticket, error) {
	return nil, errFakeService
}

func (failingTicketService) GetTicketByID(context.Context, uuid.UUID) (*models.Ticket, error) {
	return nil, errFakeService
}

func (failingTicketService) UpdateTicketStatus(context.Context, uuid.UUID, string) error {
	return errFakeService
}

type failingTicketTriageService struct{}

func (failingTicketTriageService) AnalyzeTicket(context.Context, uuid.UUID) (*models.TicketTriage, error) {
	return nil, errFakeService
}

func (failingTicketTriageService) ListTriagesByTicketID(context.Context, uuid.UUID) ([]*models.TicketTriage, error) {
	return nil, errFakeService
}

func (failingTicketTriageService) GetLatestTriageByTicketID(context.Context, uuid.UUID) (*models.TicketTriage, error) {
	return nil, errFakeService
}

type failingDashboardService struct{}

func (failingDashboardService) GetStats(context.Context) (*models.DashboardStats, error) {
	return nil, errFakeService
}

// newFailingServicesMux builds the router like newTestMux, but with services
// that always fail, for routes whose handler calls the service before any
// validation.
func newFailingServicesMux() *http.ServeMux {
	mux := http.NewServeMux()

	RegisterRoutes(
		mux,
		handlers.NewTicketHandler(failingTicketService{}),
		handlers.NewTicketTriageHandler(failingTicketTriageService{}),
		handlers.NewDashboardHandler(failingDashboardService{}),
		noRateLimits,
	)

	return mux
}

// serveMux sends req through mux (or a handler wrapping it) and returns the
// recorded response. A panic (for example a nil service reached because a
// route points to the wrong handler) fails the test instead of aborting the
// whole test binary.
func serveMux(t *testing.T, mux http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("handler panicked (panic: %v); with a nil service this usually means "+
				"the request reached the service layer\n%s", p, debug.Stack())
		}
	}()

	mux.ServeHTTP(rec, req)

	return rec
}

// routeCase is a request sent through the router and the JSON error the
// handler it reaches must answer with.
type routeCase struct {
	name        string
	method      string
	target      string
	body        string
	wantStatus  int
	wantMessage string
}

// assertHandlerError checks that rec holds the JSON error written by the
// handlers' writeError. ServeMux writes its own 404 and 405 as plain text, so
// a JSON body shows the request reached a handler.
func assertHandlerError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	// json.Encoder ends its output with a newline.
	if got, want := rec.Body.String(), "{\"error\":\""+wantMessage+"\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRegisterRoutes_RoutesReachHandlerValidation(t *testing.T) {
	tests := []routeCase{
		{
			// Only Create checks for a title and description.
			name:        "POST /tickets",
			method:      http.MethodPost,
			target:      "/tickets",
			body:        `{}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "title and description are required",
		},
		{
			name:        "GET /tickets/{id}",
			method:      http.MethodGet,
			target:      "/tickets/not-a-uuid",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
		{
			name:        "PATCH /tickets/{id} with invalid ID",
			method:      http.MethodPatch,
			target:      "/tickets/not-a-uuid",
			body:        `{"status": "resolved"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
		{
			// Only UpdateStatus checks the status value.
			name:        "PATCH /tickets/{id} with invalid status",
			method:      http.MethodPatch,
			target:      "/tickets/" + validTicketID,
			body:        `{"status": "closed"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket status",
		},
		{
			name:        "POST /tickets/{id}/triage with invalid ID",
			method:      http.MethodPost,
			target:      "/tickets/not-a-uuid/triage",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
		{
			// Only the triage Create rejects a non-empty body, before any
			// OpenAI call.
			name:        "POST /tickets/{id}/triage with body",
			method:      http.MethodPost,
			target:      "/tickets/" + validTicketID + "/triage",
			body:        `{}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "request body must be empty",
		},
		{
			name:        "GET /tickets/{id}/triages",
			method:      http.MethodGet,
			target:      "/tickets/not-a-uuid/triages",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
		{
			name:        "GET /tickets/{id}/triage",
			method:      http.MethodGet,
			target:      "/tickets/not-a-uuid/triage",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid ticket ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))

			rec := serveMux(t, newTestMux(), req)

			assertHandlerError(t, rec, tt.wantStatus, tt.wantMessage)
		})
	}
}

func TestRegisterRoutes_RoutesReachExpectedHandler(t *testing.T) {
	// GET /tickets and GET /dashboard/stats have no validation step, and the
	// GET routes with an ID share the "invalid ticket ID" message. With
	// failing services every handler returns a 500 message of its own, so
	// a route wired to the wrong handler fails here.
	tests := []routeCase{
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
			name:        "GET /tickets/{id}/triages",
			method:      http.MethodGet,
			target:      "/tickets/" + validTicketID + "/triages",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve ticket triage history",
		},
		{
			name:        "GET /tickets/{id}/triage",
			method:      http.MethodGet,
			target:      "/tickets/" + validTicketID + "/triage",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve latest ticket triage",
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
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))

			rec := serveMux(t, newFailingServicesMux(), req)

			assertHandlerError(t, rec, tt.wantStatus, tt.wantMessage)
		})
	}
}

func TestRegisterRoutes_WrongMethodReturnsMethodNotAllowed(t *testing.T) {
	// Allow lists the methods registered for the path (GET also matches
	// HEAD), so it shows which patterns ServeMux matched.
	tests := []struct {
		name      string
		method    string
		target    string
		wantAllow string
	}{
		{name: "DELETE /tickets", method: http.MethodDelete, target: "/tickets", wantAllow: "GET, HEAD, POST"},
		{name: "DELETE /tickets/{id}", method: http.MethodDelete, target: "/tickets/" + validTicketID, wantAllow: "GET, HEAD, PATCH"},
		{name: "PUT /tickets/{id}/triage", method: http.MethodPut, target: "/tickets/" + validTicketID + "/triage", wantAllow: "GET, HEAD, POST"},
		{name: "POST /tickets/{id}/triages", method: http.MethodPost, target: "/tickets/" + validTicketID + "/triages", wantAllow: "GET, HEAD"},
		{name: "POST /dashboard/stats", method: http.MethodPost, target: "/dashboard/stats", wantAllow: "GET, HEAD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)

			rec := serveMux(t, newTestMux(), req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}

			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("Allow = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

func TestRegisterRoutes_UnknownPathReturnsNotFound(t *testing.T) {
	// ServeMux answers unregistered paths with a plain-text 404, unlike the
	// JSON "ticket not found" a handler returns for a missing resource.
	tests := []struct {
		name   string
		target string
	}{
		{name: "unknown ticket subresource", target: "/tickets/" + validTicketID + "/comments"},
		{name: "extra segment after triage", target: "/tickets/" + validTicketID + "/triage/latest"},
		{name: "dashboard without stats", target: "/dashboard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)

			rec := serveMux(t, newTestMux(), req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}

			if got, want := rec.Header().Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
				t.Errorf("Content-Type = %q, want %q", got, want)
			}

			if got, want := rec.Body.String(), "404 page not found\n"; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
		})
	}
}
