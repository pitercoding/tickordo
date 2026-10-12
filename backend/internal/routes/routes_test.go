package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pitercoding/tickordo/internal/handlers"
)

// newTestMux builds the router exactly as main does, but with nil services:
// the health routes never reach the service layer, so no database or OpenAI
// client is needed.
func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()

	RegisterRoutes(
		mux,
		handlers.NewTicketHandler(nil),
		handlers.NewTicketTriageHandler(nil),
		handlers.NewDashboardHandler(nil),
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
