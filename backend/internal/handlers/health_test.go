package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	rec := serve(t, Health, req)

	// Health never calls WriteHeader, so the 200 comes from the first write.
	assertJSONResponse(t, rec, http.StatusOK, map[string]any{
		"status": "ok",
	})

	// json.Encoder ends its output with a newline, so this is the exact
	// body a client receives.
	if got, want := rec.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
