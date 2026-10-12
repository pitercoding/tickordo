package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
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

// assertReachesService runs handler against req and checks that it gets
// past validation and calls the service.
//
// It is the counterpart of serve: the handler is built with a nil service,
// so calling the service panics with a nil pointer dereference. That panic
// is expected here; returning normally means validation rejected the request.
func assertReachesService(t *testing.T, handler http.HandlerFunc, req *http.Request) {
	t.Helper()

	rec := httptest.NewRecorder()

	defer func() {
		p := recover()
		if p == nil {
			t.Fatalf("handler returned status %d (body %q), want it to call the service",
				rec.Code, rec.Body.String())
		}

		// Only a nil pointer dereference shows the nil service was called;
		// any other panic is a real failure.
		var runtimeErr runtime.Error
		if err, ok := p.(error); !ok || !errors.As(err, &runtimeErr) ||
			!strings.Contains(runtimeErr.Error(), "nil pointer dereference") {
			t.Fatalf("handler panicked with %v, want a nil pointer dereference from the nil service", p)
		}
	}()

	handler(rec, req)
}

func TestTicketTriageHandlerCreate_AcceptsEmptyBody(t *testing.T) {
	tests := []struct {
		name   string
		body   io.Reader
		header map[string]string
	}{
		{name: "no body", body: nil},
		{name: "empty body", body: strings.NewReader("")},
		{
			// What the frontend sends: no body, but the apiClient always
			// sets this header.
			name:   "no body with JSON Content-Type",
			body:   nil,
			header: map[string]string{"Content-Type": "application/json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTicketTriageHandler(nil)
			req := httptest.NewRequest(http.MethodPost, "/tickets/"+validTicketID+"/triage", tt.body)
			req.SetPathValue("id", validTicketID)
			for key, value := range tt.header {
				req.Header.Set(key, value)
			}

			assertReachesService(t, handler.Create, req)
		})
	}
}

func TestTicketTriageHandlerCreate_RejectsNonEmptyBody(t *testing.T) {
	tests := []struct {
		name string
		body io.Reader
	}{
		{name: "empty JSON object", body: strings.NewReader(`{}`)},
		{name: "JSON object with a field", body: strings.NewReader(`{"model": "gpt-5"}`)},
		{name: "JSON null", body: strings.NewReader(`null`)},
		{name: "malformed JSON", body: strings.NewReader(`{"model": `)},
		{name: "plain text", body: strings.NewReader("analyze")},
		{name: "whitespace only", body: strings.NewReader(" \n")},
		{name: "single byte", body: strings.NewReader("x")},
		{
			// Only the first byte is read, so a large body is rejected
			// without being consumed.
			name: "body larger than 1 MB",
			body: bytes.NewReader(bytes.Repeat([]byte("a"), 2*maxRequestBodyBytes)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTicketTriageHandler(nil)
			req := httptest.NewRequest(http.MethodPost, "/tickets/"+validTicketID+"/triage", tt.body)
			req.SetPathValue("id", validTicketID)
			req.Header.Set("Content-Type", "application/json")

			// serve fails the test if the nil service is reached, so this
			// also checks that no triage (and no OpenAI call) is attempted.
			rec := serve(t, handler.Create, req)

			assertErrorResponse(t, rec, http.StatusBadRequest, "request body must be empty")
		})
	}
}

func TestTicketTriageHandlerCreate_ChecksIDBeforeBody(t *testing.T) {
	handler := NewTicketTriageHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/tickets/x/triage", strings.NewReader(`{"model": "gpt-5"}`))
	req.SetPathValue("id", "not-a-uuid")

	rec := serve(t, handler.Create, req)

	// Both the ID and the body are invalid; the ID is reported first.
	assertErrorResponse(t, rec, http.StatusBadRequest, "invalid ticket ID")
}

// fakeTicketTriageService implements ticketTriageService without a database
// or OpenAI. Every method records the call and returns err, so a test
// decides exactly which error the handler receives from the service layer.
type fakeTicketTriageService struct {
	err error

	calls       int
	gotTicketID uuid.UUID
}

func (f *fakeTicketTriageService) AnalyzeTicket(ctx context.Context, ticketID uuid.UUID) (*models.TicketTriage, error) {
	f.calls++
	f.gotTicketID = ticketID
	return nil, f.err
}

func (f *fakeTicketTriageService) ListTriagesByTicketID(ctx context.Context, ticketID uuid.UUID) ([]*models.TicketTriage, error) {
	f.calls++
	f.gotTicketID = ticketID
	return nil, f.err
}

func (f *fakeTicketTriageService) GetLatestTriageByTicketID(ctx context.Context, ticketID uuid.UUID) (*models.TicketTriage, error) {
	f.calls++
	f.gotTicketID = ticketID
	return nil, f.err
}

func TestTicketTriageHandler_MapsServiceErrors(t *testing.T) {
	// The real service wraps repository errors with %w, so the fakes return
	// wrapped errors too.
	ticketNotFound := fmt.Errorf("failed to get ticket: %w", repositories.ErrTicketNotFound)
	unexpected := errors.New("connection refused")

	create := func(h *TicketTriageHandler) http.HandlerFunc { return h.Create }
	list := func(h *TicketTriageHandler) http.HandlerFunc { return h.List }
	getLatest := func(h *TicketTriageHandler) http.HandlerFunc { return h.GetLatest }

	tests := []struct {
		name        string
		method      string
		handler     func(h *TicketTriageHandler) http.HandlerFunc
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "Create/ticket not found",
			method:      http.MethodPost,
			handler:     create,
			serviceErr:  ticketNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
		},
		{
			// What the service returns when the OpenAI call fails.
			name:        "Create/analyzer error",
			method:      http.MethodPost,
			handler:     create,
			serviceErr:  fmt.Errorf("failed to analyze ticket: %w", errors.New("openai timeout")),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to triage ticket",
		},
		{
			name:        "Create/unexpected error",
			method:      http.MethodPost,
			handler:     create,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to triage ticket",
		},
		{
			name:        "List/ticket not found",
			method:      http.MethodGet,
			handler:     list,
			serviceErr:  ticketNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
		},
		{
			name:        "List/unexpected error",
			method:      http.MethodGet,
			handler:     list,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve ticket triage history",
		},
		{
			name:        "GetLatest/ticket not found",
			method:      http.MethodGet,
			handler:     getLatest,
			serviceErr:  ticketNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
		},
		{
			// The ticket exists but has never been triaged.
			name:        "GetLatest/triage not found",
			method:      http.MethodGet,
			handler:     getLatest,
			serviceErr:  fmt.Errorf("failed to get latest ticket triage: %w", repositories.ErrTicketTriageNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket triage not found",
		},
		{
			name:        "GetLatest/unexpected error",
			method:      http.MethodGet,
			handler:     getLatest,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to retrieve latest ticket triage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeTicketTriageService{err: tt.serviceErr}
			handler := NewTicketTriageHandler(service)

			// No body: Create requires an empty one, List and GetLatest ignore it.
			req := httptest.NewRequest(tt.method, "/tickets/"+validTicketID+"/triage", nil)
			req.SetPathValue("id", validTicketID)

			// serve fails the test if the handler panics on the error.
			rec := serve(t, tt.handler(handler), req)

			// The exact message also shows the internal error text is not
			// leaked to the client.
			assertErrorResponse(t, rec, tt.wantStatus, tt.wantMessage)

			if service.calls != 1 {
				t.Errorf("service calls = %d, want 1", service.calls)
			}

			if service.gotTicketID.String() != validTicketID {
				t.Errorf("service got ticket ID %q, want %q", service.gotTicketID, validTicketID)
			}
		})
	}
}
