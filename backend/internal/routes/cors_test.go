package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pitercoding/tickordo/internal/middleware"
)

// assertCORSHeaders checks the headers middleware.CORS sets on every response.
func assertCORSHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	headers := []struct {
		name string
		want string
	}{
		{name: "Access-Control-Allow-Origin", want: "http://localhost:3000"},
		{name: "Access-Control-Allow-Methods", want: "GET, POST, PATCH, OPTIONS"},
		{name: "Access-Control-Allow-Headers", want: "Content-Type"},
	}

	for _, h := range headers {
		if got := rec.Header().Get(h.name); got != h.want {
			t.Errorf("%s = %q, want %q", h.name, got, h.want)
		}
	}
}

// The tests below wrap the router in middleware.CORS exactly as main does
// with the server's Handler, using the default CORS_ALLOWED_ORIGIN.

func TestCORSRouter_PreflightStopsBeforeRouter(t *testing.T) {
	// No route is registered for OPTIONS, so a preflight that reached the
	// router would get a 405 with an Allow header. The nil services would
	// panic if a handler ran.
	targets := []string{
		"/tickets",
		"/tickets/" + validTicketID,
		"/tickets/" + validTicketID + "/triage",
		"/dashboard/stats",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, target, nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", "content-type")

			rec := serveMux(t, middleware.CORS("http://localhost:3000", newTestMux()), req)

			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
			}

			if got := rec.Header().Get("Allow"); got != "" {
				t.Errorf("Allow = %q, want empty (preflight reached the router)", got)
			}

			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}

			assertCORSHeaders(t, rec)
		})
	}
}

func TestCORSRouter_RequestReachesRouteWithCORSHeaders(t *testing.T) {
	t.Run("GET /health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")

		rec := serveMux(t, middleware.CORS("http://localhost:3000", newTestMux()), req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		if got, want := rec.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}

		assertCORSHeaders(t, rec)
	})

	t.Run("GET /dashboard/stats error", func(t *testing.T) {
		// Error responses also carry the CORS headers, so the frontend can
		// read the handler's JSON error message.
		req := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)
		req.Header.Set("Origin", "http://localhost:3000")

		rec := serveMux(t, middleware.CORS("http://localhost:3000", newFailingServicesMux()), req)

		assertHandlerError(t, rec, http.StatusInternalServerError, "failed to get dashboard statistics")
		assertCORSHeaders(t, rec)
	})
}
