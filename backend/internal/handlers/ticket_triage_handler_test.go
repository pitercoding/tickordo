package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTicketTriageHandler_RejectsInvalidTicketID(t *testing.T) {
	endpoints := []struct {
		name    string
		method  string
		handler func(h *TicketTriageHandler) http.HandlerFunc
	}{
		{
			// The service would call OpenAI, so this also guards against
			// paid API calls for malformed IDs.
			name:    "Create",
			method:  http.MethodPost,
			handler: func(h *TicketTriageHandler) http.HandlerFunc { return h.Create },
		},
		{
			name:    "List",
			method:  http.MethodGet,
			handler: func(h *TicketTriageHandler) http.HandlerFunc { return h.List },
		},
		{
			name:    "GetLatest",
			method:  http.MethodGet,
			handler: func(h *TicketTriageHandler) http.HandlerFunc { return h.GetLatest },
		},
	}

	for _, endpoint := range endpoints {
		for _, tt := range invalidTicketIDs {
			t.Run(endpoint.name+"/"+tt.name, func(t *testing.T) {
				handler := NewTicketTriageHandler(nil)
				req := httptest.NewRequest(endpoint.method, "/tickets/x/triage", nil)
				req.SetPathValue("id", tt.id)

				rec := serve(t, endpoint.handler(handler), req)

				assertErrorResponse(t, rec, http.StatusBadRequest, "invalid ticket ID")
			})
		}
	}
}
