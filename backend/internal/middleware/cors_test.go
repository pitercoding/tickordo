package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// These are the exact values CORS sets in cors.go. The tests pass
// wantAllowOrigin as the allowed origin, matching the config default.
const (
	wantAllowOrigin  = "http://localhost:3000"
	wantAllowMethods = "GET, POST, PATCH, OPTIONS"
	wantAllowHeaders = "Content-Type"
)

// assertCORSHeaders checks that rec carries the three headers CORS sets,
// with wantOrigin as the allowed origin.
func assertCORSHeaders(t *testing.T, rec *httptest.ResponseRecorder, wantOrigin string) {
	t.Helper()

	headers := []struct {
		name string
		want string
	}{
		{name: "Access-Control-Allow-Origin", want: wantOrigin},
		{name: "Access-Control-Allow-Methods", want: wantAllowMethods},
		{name: "Access-Control-Allow-Headers", want: wantAllowHeaders},
	}

	for _, h := range headers {
		if got := rec.Header().Get(h.name); got != h.want {
			t.Errorf("%s = %q, want %q", h.name, got, h.want)
		}
	}
}

// recordingHandler is the next handler for CORS. It records whether it was
// called and answers with a status and body CORS itself never writes.
type recordingHandler struct {
	called bool
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("next"))
}

func TestCORS_NonPreflightRequestReachesNextHandler(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/tickets", nil)
			req.Header.Set("Origin", wantAllowOrigin)
			rec := httptest.NewRecorder()
			next := &recordingHandler{}

			CORS(wantAllowOrigin, next).ServeHTTP(rec, req)

			if !next.called {
				t.Fatal("next handler was not called")
			}

			// The status and body come from next, untouched by CORS.
			if rec.Code != http.StatusCreated {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
			}

			if got, want := rec.Body.String(), "next"; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}

			assertCORSHeaders(t, rec, wantAllowOrigin)
		})
	}
}

func TestCORS_PreflightReturnsNoContentWithoutCallingNext(t *testing.T) {
	// A browser preflight for the frontend's PATCH with a JSON body.
	req := httptest.NewRequest(http.MethodOptions, "/tickets/123", nil)
	req.Header.Set("Origin", wantAllowOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()
	next := &recordingHandler{}

	CORS(wantAllowOrigin, next).ServeHTTP(rec, req)

	if next.called {
		t.Error("next handler was called for a preflight request")
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}

	assertCORSHeaders(t, rec, wantAllowOrigin)
}

func TestCORS_HeadersDoNotDependOnRequestOrigin(t *testing.T) {
	// CORS never reads the Origin header: it always allows the configured origin and
	// still calls next. Blocking any other origin is left to the browser,
	// which compares its own origin with Access-Control-Allow-Origin.
	tests := []struct {
		name   string
		origin string
	}{
		{name: "other origin", origin: "http://evil.example"},
		{name: "same host on another port", origin: "http://localhost:5173"},
		{name: "no Origin header", origin: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			next := &recordingHandler{}

			CORS(wantAllowOrigin, next).ServeHTTP(rec, req)

			if !next.called {
				t.Error("next handler was not called")
			}

			assertCORSHeaders(t, rec, wantAllowOrigin)
		})
	}
}

func TestCORS_UsesConfiguredOrigin(t *testing.T) {
	// The allowed origin comes from the caller (CORS_ALLOWED_ORIGIN in main),
	// for normal and preflight requests alike.
	const origin = "https://app.tickordo.example"

	t.Run("GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		next := &recordingHandler{}

		CORS(origin, next).ServeHTTP(rec, req)

		if !next.called {
			t.Error("next handler was not called")
		}

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
		}

		assertCORSHeaders(t, rec, origin)
	})

	t.Run("OPTIONS preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/tickets", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		rec := httptest.NewRecorder()
		next := &recordingHandler{}

		CORS(origin, next).ServeHTTP(rec, req)

		if next.called {
			t.Error("next handler was called for a preflight request")
		}

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}

		assertCORSHeaders(t, rec, origin)
	})
}
