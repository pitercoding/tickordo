package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// validTicketID is a well-formed UUID, used when a test needs the ID check
// to pass so that a later validation step is the one under test.
const validTicketID = "550e8400-e29b-41d4-a716-446655440000"

// invalidTicketIDs are path values that uuid.Parse rejects. uuid.Parse also
// accepts the 32-hex, "{...}" and "urn:uuid:" forms, so none are listed here.
var invalidTicketIDs = []struct {
	name string
	id   string
}{
	{name: "empty", id: ""},
	{name: "plain text", id: "not-a-uuid"},
	{name: "integer", id: "123"},
	{name: "one character short", id: "550e8400-e29b-41d4-a716-44665544000"},
	{name: "non-hex character", id: "550e8400-e29b-41d4-a716-44665544000g"},
	{name: "trailing characters", id: validTicketID + "-extra"},
	{name: "misplaced hyphens", id: "550e8400e-29b-41d4-a716-446655440000"},
}

// serve runs handler against req and returns the recorded response.
//
// The handlers under test are built with a nil service, so a request that
// gets past validation panics as soon as the handler calls the service.
// serve turns that panic into a test failure, which is how these tests check
// that invalid input is rejected before any service or database call.
//
// Any panic fails the test, not only one from the nil service, so the stack
// trace is included to show where the handler actually panicked.
func serve(t *testing.T, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("handler panicked (panic: %v); with a nil service this usually means "+
				"validation let the request reach the service layer\n%s", p, debug.Stack())
		}
	}()

	handler(rec, req)

	return rec
}

// assertErrorResponse checks the status code and that the body is exactly
// the JSON error format written by writeError: {"error": "<message>"}.
func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	raw := rec.Body.String()

	// DisallowUnknownFields makes extra keys fail the test, so the body
	// must match the errorResponse shape and nothing else.
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	var body errorResponse
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("response body is not a JSON error object: %v; body = %q", err, raw)
	}

	if body.Error != wantMessage {
		t.Errorf("error message = %q, want %q", body.Error, wantMessage)
	}
}

// createBody builds a create-ticket JSON body with properly escaped fields.
func createBody(t *testing.T, title, description string) string {
	t.Helper()

	data, err := json.Marshal(createTicketRequest{Title: title, Description: description})
	if err != nil {
		t.Fatalf("failed to marshal create ticket request: %v", err)
	}

	return string(data)
}

// bodyOfSize returns prefix + fill... + suffix, padded with fill so the body
// is exactly size bytes long. It is used to build bodies right at, or just
// over, maxRequestBodyBytes.
func bodyOfSize(t *testing.T, prefix, suffix string, fill byte, size int) io.Reader {
	t.Helper()

	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		t.Fatalf("prefix and suffix (%d bytes) do not fit in %d bytes", len(prefix)+len(suffix), size)
	}

	var b bytes.Buffer

	b.WriteString(prefix)
	b.Write(bytes.Repeat([]byte{fill}, padding))
	b.WriteString(suffix)

	return &b
}

func TestTicketHandlerCreate_RejectsInvalidRequests(t *testing.T) {
	const (
		invalidBody     = "invalid request body"
		requiredFields  = "title and description are required"
		titleTooLong    = "title must be at most 255 characters"
		descriptionLong = "description must be at most 5000 characters"
		nullCharacter   = "title and description must not contain null characters"
	)

	// Some cases pass the title checks on purpose and only break the
	// description length (the last length check). Getting the
	// description error back proves the title was accepted.
	tooLongDescription := strings.Repeat("d", maxDescriptionLength+1)

	tests := []struct {
		name        string
		body        func(t *testing.T) io.Reader
		wantMessage string
	}{
		// Body that cannot be decoded.
		{
			name:        "no body",
			body:        func(t *testing.T) io.Reader { return nil },
			wantMessage: invalidBody,
		},
		{
			name:        "empty body",
			body:        func(t *testing.T) io.Reader { return strings.NewReader("") },
			wantMessage: invalidBody,
		},
		{
			name:        "malformed JSON",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`{"title": "Login fails",`) },
			wantMessage: invalidBody,
		},
		{
			name:        "JSON array instead of object",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`[]`) },
			wantMessage: invalidBody,
		},
		{
			name: "title with wrong type",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(`{"title": 123, "description": "Cannot log in"}`)
			},
			wantMessage: invalidBody,
		},

		// Unknown fields. The known fields are valid, so without the
		// unknown-field check these requests would reach the service.
		{
			name: "unknown field",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(`{"title": "Example", "description": "Example", "unexpected": "value"}`)
			},
			wantMessage: invalidBody,
		},
		{
			name: "misspelled field",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(`{"title": "Example", "descripton": "Example"}`)
			},
			wantMessage: invalidBody,
		},
		{
			name: "status is not accepted on create",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(`{"title": "Example", "description": "Example", "status": "resolved"}`)
			},
			wantMessage: invalidBody,
		},

		// Content after the first JSON value.
		{
			name:        "valid JSON followed by invalid text",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, "Example", "Example") + " lixo") },
			wantMessage: invalidBody,
		},
		{
			name: "two consecutive objects",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Example", "Example") + createBody(t, "Other", "Other"))
			},
			wantMessage: invalidBody,
		},
		{
			name: "two objects separated by whitespace",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Example", "Example") + " \n " + createBody(t, "Other", "Other"))
			},
			wantMessage: invalidBody,
		},
		{
			name:        "object followed by another JSON value",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, "Example", "Example") + " 1") },
			wantMessage: invalidBody,
		},

		// Required fields.
		{
			name:        "JSON null",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`null`) },
			wantMessage: requiredFields,
		},
		{
			name:        "empty object",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`{}`) },
			wantMessage: requiredFields,
		},
		{
			name:        "missing title",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`{"description": "Cannot log in"}`) },
			wantMessage: requiredFields,
		},
		{
			name:        "missing description",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(`{"title": "Login fails"}`) },
			wantMessage: requiredFields,
		},
		{
			name:        "empty title",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, "", "Cannot log in")) },
			wantMessage: requiredFields,
		},
		{
			name:        "whitespace-only title",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, " \t\n ", "Cannot log in")) },
			wantMessage: requiredFields,
		},
		{
			name:        "empty description",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, "Login fails", "")) },
			wantMessage: requiredFields,
		},
		{
			name:        "whitespace-only description",
			body:        func(t *testing.T) io.Reader { return strings.NewReader(createBody(t, "Login fails", "  \n\t")) },
			wantMessage: requiredFields,
		},
		{
			// Trailing whitespace is still a single JSON value: the body is
			// decoded and reaches the required-fields check.
			name: "trailing whitespace is accepted by the decoder",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Login fails", "") + " \n\t\r\n ")
			},
			wantMessage: requiredFields,
		},

		// Title length (counted in characters, after trimming).
		{
			name: "title over 255 characters",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, strings.Repeat("t", maxTitleLength+1), "Cannot log in"))
			},
			wantMessage: titleTooLong,
		},
		{
			name: "title over 255 multi-byte characters",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, strings.Repeat("é", maxTitleLength+1), "Cannot log in"))
			},
			wantMessage: titleTooLong,
		},
		{
			name: "title of exactly 255 characters passes the title check",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, strings.Repeat("t", maxTitleLength), tooLongDescription))
			},
			wantMessage: descriptionLong,
		},
		{
			name: "title of 255 multi-byte characters passes the title check",
			body: func(t *testing.T) io.Reader {
				// 255 runes but 510 bytes: the limit must count characters.
				return strings.NewReader(createBody(t, strings.Repeat("é", maxTitleLength), tooLongDescription))
			},
			wantMessage: descriptionLong,
		},
		{
			name: "surrounding whitespace does not count toward the title limit",
			body: func(t *testing.T) io.Reader {
				title := "   " + strings.Repeat("t", maxTitleLength) + "   "
				return strings.NewReader(createBody(t, title, tooLongDescription))
			},
			wantMessage: descriptionLong,
		},

		// Description length.
		{
			name: "description over 5000 characters",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Login fails", tooLongDescription))
			},
			wantMessage: descriptionLong,
		},
		{
			name: "description over 5000 multi-byte characters",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Login fails", strings.Repeat("ç", maxDescriptionLength+1)))
			},
			wantMessage: descriptionLong,
		},

		// NUL characters, which PostgreSQL text columns reject. The JSON
		// body carries them as the \u0000 escape.
		{
			name: "title with NUL character",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Login\x00fails", "Cannot log in"))
			},
			wantMessage: nullCharacter,
		},
		{
			name: "description with NUL character",
			body: func(t *testing.T) io.Reader {
				return strings.NewReader(createBody(t, "Login fails", "Cannot\x00log in"))
			},
			wantMessage: nullCharacter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTicketHandler(nil)
			req := httptest.NewRequest(http.MethodPost, "/tickets", tt.body(t))

			rec := serve(t, handler.Create, req)

			assertErrorResponse(t, rec, http.StatusBadRequest, tt.wantMessage)
		})
	}
}

func TestTicketHandlerUpdateStatus_RejectsInvalidRequests(t *testing.T) {
	const (
		invalidBody   = "invalid request body"
		invalidStatus = "invalid ticket status"
	)

	tests := []struct {
		name        string
		body        io.Reader
		wantMessage string
	}{
		// Body that cannot be decoded.
		{name: "no body", body: nil, wantMessage: invalidBody},
		{name: "empty body", body: strings.NewReader(""), wantMessage: invalidBody},
		{name: "malformed JSON", body: strings.NewReader(`{"status": "open"`), wantMessage: invalidBody},
		{name: "status with wrong type", body: strings.NewReader(`{"status": 1}`), wantMessage: invalidBody},

		// Unknown fields. "resolved" is a valid status, so without the
		// unknown-field check these requests would reach the service.
		{name: "unknown field", body: strings.NewReader(`{"status": "resolved", "unexpected": "value"}`), wantMessage: invalidBody},
		{name: "title is not accepted on status update", body: strings.NewReader(`{"status": "resolved", "title": "Example"}`), wantMessage: invalidBody},

		// Content after the first JSON value.
		{name: "valid JSON followed by invalid text", body: strings.NewReader(`{"status": "resolved"} lixo`), wantMessage: invalidBody},
		{name: "two consecutive objects", body: strings.NewReader(`{"status": "resolved"}{"status": "open"}`), wantMessage: invalidBody},
		{name: "two objects separated by whitespace", body: strings.NewReader("{\"status\": \"resolved\"}\n{\"status\": \"open\"}"), wantMessage: invalidBody},
		{name: "object followed by another JSON value", body: strings.NewReader(`{"status": "resolved"} "open"`), wantMessage: invalidBody},

		// Trailing whitespace is still a single JSON value: the body is
		// decoded and reaches the status check.
		{name: "trailing whitespace is accepted by the decoder", body: strings.NewReader("{\"status\": \"closed\"} \n\t\r\n "), wantMessage: invalidStatus},

		// Status outside open, in_progress and resolved.
		{name: "JSON null", body: strings.NewReader(`null`), wantMessage: invalidStatus},
		{name: "missing status", body: strings.NewReader(`{}`), wantMessage: invalidStatus},
		{name: "empty status", body: strings.NewReader(`{"status": ""}`), wantMessage: invalidStatus},
		{name: "whitespace-only status", body: strings.NewReader(`{"status": "   "}`), wantMessage: invalidStatus},
		{name: "unknown status", body: strings.NewReader(`{"status": "closed"}`), wantMessage: invalidStatus},
		{name: "uppercase status", body: strings.NewReader(`{"status": "OPEN"}`), wantMessage: invalidStatus},
		{name: "status with space instead of underscore", body: strings.NewReader(`{"status": "in progress"}`), wantMessage: invalidStatus},
		{name: "status with hyphen instead of underscore", body: strings.NewReader(`{"status": "in-progress"}`), wantMessage: invalidStatus},
		{name: "triage priority used as status", body: strings.NewReader(`{"status": "high"}`), wantMessage: invalidStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTicketHandler(nil)
			req := httptest.NewRequest(http.MethodPatch, "/tickets/"+validTicketID, tt.body)
			req.SetPathValue("id", validTicketID)

			rec := serve(t, handler.UpdateStatus, req)

			assertErrorResponse(t, rec, http.StatusBadRequest, tt.wantMessage)
		})
	}
}

func TestTicketHandler_RejectsInvalidTicketID(t *testing.T) {
	endpoints := []struct {
		name    string
		method  string
		handler func(h *TicketHandler) http.HandlerFunc
		body    string
	}{
		{
			name:    "GetByID",
			method:  http.MethodGet,
			handler: func(h *TicketHandler) http.HandlerFunc { return h.GetByID },
		},
		{
			name:    "UpdateStatus",
			method:  http.MethodPatch,
			handler: func(h *TicketHandler) http.HandlerFunc { return h.UpdateStatus },
			// A valid body, so the ID is the only thing wrong.
			body: `{"status": "resolved"}`,
		},
	}

	for _, endpoint := range endpoints {
		for _, tt := range invalidTicketIDs {
			t.Run(endpoint.name+"/"+tt.name, func(t *testing.T) {
				handler := NewTicketHandler(nil)
				req := httptest.NewRequest(endpoint.method, "/tickets/x", strings.NewReader(endpoint.body))
				req.SetPathValue("id", tt.id)

				rec := serve(t, endpoint.handler(handler), req)

				assertErrorResponse(t, rec, http.StatusBadRequest, "invalid ticket ID")
			})
		}
	}
}

func TestTicketHandlerUpdateStatus_ChecksIDBeforeBody(t *testing.T) {
	handler := NewTicketHandler(nil)
	req := httptest.NewRequest(http.MethodPatch, "/tickets/x", strings.NewReader(`{"status": `))
	req.SetPathValue("id", "not-a-uuid")

	rec := serve(t, handler.UpdateStatus, req)

	// Both the ID and the body are invalid; the ID is reported first.
	assertErrorResponse(t, rec, http.StatusBadRequest, "invalid ticket ID")
}

// bodyLimitEndpoints are the handlers that cap their JSON body at
// maxRequestBodyBytes, each with a request builder.
var bodyLimitEndpoints = []struct {
	name       string
	handler    func(h *TicketHandler) http.HandlerFunc
	newRequest func(body io.Reader) *http.Request
	// validBody is a request that passes every check, so with a nil service
	// it would reach the service layer.
	validBody string
	// fieldPrefix opens a JSON string field that bodyOfSize pads and the
	// suffix `"}` closes.
	fieldPrefix string
	// fieldMessage is the validation error that a padded field triggers
	// once the body is decoded.
	fieldMessage string
}{
	{
		name:    "Create",
		handler: func(h *TicketHandler) http.HandlerFunc { return h.Create },
		newRequest: func(body io.Reader) *http.Request {
			return httptest.NewRequest(http.MethodPost, "/tickets", body)
		},
		validBody:    `{"title": "Login fails", "description": "Cannot log in"}`,
		fieldPrefix:  `{"title": "Login fails", "description": "`,
		fieldMessage: "description must be at most 5000 characters",
	},
	{
		name:    "UpdateStatus",
		handler: func(h *TicketHandler) http.HandlerFunc { return h.UpdateStatus },
		newRequest: func(body io.Reader) *http.Request {
			req := httptest.NewRequest(http.MethodPatch, "/tickets/"+validTicketID, body)
			req.SetPathValue("id", validTicketID)
			return req
		},
		validBody:    `{"status": "resolved"}`,
		fieldPrefix:  `{"status": "`,
		fieldMessage: "invalid ticket status",
	},
}

func TestTicketHandler_RejectsOversizedBody(t *testing.T) {
	const tooLarge = "request body must be at most 1 MB"

	for _, endpoint := range bodyLimitEndpoints {
		tests := []struct {
			name string
			body func(t *testing.T) io.Reader
		}{
			{
				// Well-formed JSON one byte over the limit: only the cap can
				// make it fail to decode.
				name: "valid JSON one byte over the limit",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.fieldPrefix, `"}`, 'a', maxRequestBodyBytes+1)
				},
			},
			{
				// A valid request whose trailing whitespace crosses the limit.
				// Without the cap it would reach the service, which panics
				// here (nil service), so a 413 also shows the service is not
				// called.
				name: "valid request padded past the limit with trailing whitespace",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.validBody, "", ' ', maxRequestBodyBytes+1)
				},
			},
			{
				name: "body far over the limit",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.fieldPrefix, `"}`, 'a', 2*maxRequestBodyBytes)
				},
			},
		}

		for _, tt := range tests {
			t.Run(endpoint.name+"/"+tt.name, func(t *testing.T) {
				handler := NewTicketHandler(nil)
				req := endpoint.newRequest(tt.body(t))

				rec := serve(t, endpoint.handler(handler), req)

				assertErrorResponse(t, rec, http.StatusRequestEntityTooLarge, tooLarge)
			})
		}
	}
}

func TestTicketHandler_AcceptsBodyAtSizeLimit(t *testing.T) {
	for _, endpoint := range bodyLimitEndpoints {
		tests := []struct {
			name        string
			body        func(t *testing.T) io.Reader
			wantMessage string
		}{
			{
				// Exactly maxRequestBodyBytes is allowed: the body is decoded
				// and reaches field validation instead of getting a 413.
				name: "valid JSON of exactly the limit",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.fieldPrefix, `"}`, 'a', maxRequestBodyBytes)
				},
				wantMessage: endpoint.fieldMessage,
			},
			{
				// Malformed JSON within the limit is still a 400.
				name: "malformed JSON of exactly the limit",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.fieldPrefix, `"`, 'a', maxRequestBodyBytes)
				},
				wantMessage: "invalid request body",
			},
			{
				// Trailing data within the limit is still a 400.
				name: "trailing data reaching exactly the limit",
				body: func(t *testing.T) io.Reader {
					return bodyOfSize(t, endpoint.validBody, " x", ' ', maxRequestBodyBytes)
				},
				wantMessage: "invalid request body",
			},
		}

		for _, tt := range tests {
			t.Run(endpoint.name+"/"+tt.name, func(t *testing.T) {
				handler := NewTicketHandler(nil)
				req := endpoint.newRequest(tt.body(t))

				rec := serve(t, endpoint.handler(handler), req)

				assertErrorResponse(t, rec, http.StatusBadRequest, tt.wantMessage)
			})
		}
	}
}

// fakeTicketService implements ticketService without a database. Every
// method records the call and its arguments and returns ticket (where the
// method returns one) and err, so a test decides exactly what the handler
// receives from the service layer.
type fakeTicketService struct {
	// ticket is returned by CreateTicket and GetTicketByID.
	ticket *models.Ticket
	// tickets is returned by ListTickets.
	tickets []models.Ticket
	err     error

	calls int
	// gotID is the ticket ID passed to GetTicketByID or UpdateTicketStatus.
	gotID uuid.UUID
	// gotTitle and gotDescription are the values passed to CreateTicket.
	gotTitle       string
	gotDescription string
	// gotStatus is the status passed to UpdateTicketStatus.
	gotStatus string
}

func (f *fakeTicketService) CreateTicket(ctx context.Context, title string, description string) (*models.Ticket, error) {
	f.calls++
	f.gotTitle = title
	f.gotDescription = description
	return f.ticket, f.err
}

func (f *fakeTicketService) ListTickets(ctx context.Context) ([]models.Ticket, error) {
	f.calls++
	return f.tickets, f.err
}

func (f *fakeTicketService) GetTicketByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error) {
	f.calls++
	f.gotID = id
	return f.ticket, f.err
}

func (f *fakeTicketService) UpdateTicketStatus(ctx context.Context, id uuid.UUID, status string) error {
	f.calls++
	f.gotID = id
	f.gotStatus = status
	return f.err
}

func TestTicketHandler_MapsServiceErrors(t *testing.T) {
	// The real service wraps repository errors with %w, so the fakes return
	// wrapped errors too: the handler must find ErrTicketNotFound with
	// errors.Is, not by comparing the error directly.
	notFound := fmt.Errorf("failed to get ticket: %w", repositories.ErrTicketNotFound)
	unexpected := errors.New("connection refused")

	newCreateRequest := func() *http.Request {
		body := strings.NewReader(`{"title": "Login fails", "description": "Cannot log in"}`)
		return httptest.NewRequest(http.MethodPost, "/tickets", body)
	}
	newListRequest := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/tickets", nil)
	}
	newGetRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/tickets/"+validTicketID, nil)
		req.SetPathValue("id", validTicketID)
		return req
	}
	newUpdateRequest := func() *http.Request {
		body := strings.NewReader(`{"status": "resolved"}`)
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+validTicketID, body)
		req.SetPathValue("id", validTicketID)
		return req
	}

	tests := []struct {
		name        string
		handler     func(h *TicketHandler) http.HandlerFunc
		newRequest  func() *http.Request
		serviceErr  error
		wantStatus  int
		wantMessage string
		// wantID is true for the endpoints that pass the path ID to the service.
		wantID bool
	}{
		{
			name:        "Create/unexpected error",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.Create },
			newRequest:  newCreateRequest,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to create ticket",
		},
		{
			name:        "List/unexpected error",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.List },
			newRequest:  newListRequest,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to list tickets",
		},
		{
			name:        "GetByID/ticket not found",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.GetByID },
			newRequest:  newGetRequest,
			serviceErr:  notFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
			wantID:      true,
		},
		{
			name:        "GetByID/unexpected error",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.GetByID },
			newRequest:  newGetRequest,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to get ticket",
			wantID:      true,
		},
		{
			// Same text as ErrTicketNotFound but a different error value:
			// only the sentinel itself may become a 404.
			name:        "GetByID/error with the same text is not a 404",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.GetByID },
			newRequest:  newGetRequest,
			serviceErr:  errors.New("ticket not found"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to get ticket",
			wantID:      true,
		},
		{
			name:        "UpdateStatus/ticket not found",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.UpdateStatus },
			newRequest:  newUpdateRequest,
			serviceErr:  fmt.Errorf("failed to update ticket status: %w", repositories.ErrTicketNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: "ticket not found",
			wantID:      true,
		},
		{
			name:        "UpdateStatus/unexpected error",
			handler:     func(h *TicketHandler) http.HandlerFunc { return h.UpdateStatus },
			newRequest:  newUpdateRequest,
			serviceErr:  unexpected,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "failed to update ticket status",
			wantID:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeTicketService{err: tt.serviceErr}
			handler := NewTicketHandler(service)

			// serve fails the test if the handler panics on the error.
			rec := serve(t, tt.handler(handler), tt.newRequest())

			// The exact message also shows the internal error text is not
			// leaked to the client.
			assertErrorResponse(t, rec, tt.wantStatus, tt.wantMessage)

			if service.calls != 1 {
				t.Errorf("service calls = %d, want 1", service.calls)
			}

			if tt.wantID && service.gotID.String() != validTicketID {
				t.Errorf("service got ticket ID %q, want %q", service.gotID, validTicketID)
			}
		})
	}
}

// assertJSONResponse checks the status code, the JSON Content-Type and that
// the body decodes to exactly want. Comparing generic JSON values checks the
// field names and values the client sees, including null fields. want is a
// map[string]any for an object or a []any for an array.
func assertJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, want any) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	raw := rec.Body.String()

	var body any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v; body = %q", err, raw)
	}

	if !reflect.DeepEqual(body, want) {
		t.Errorf("response body = %v, want %v", body, want)
	}
}

func TestTicketHandlerGetByID_ReturnsTicket(t *testing.T) {
	category := "authentication"
	priority := "high"

	service := &fakeTicketService{
		ticket: &models.Ticket{
			ID:                   uuid.MustParse(validTicketID),
			Title:                "Login fails",
			Description:          "Cannot log in with a valid password",
			Status:               "in_progress",
			LatestTriageCategory: &category,
			LatestTriagePriority: &priority,
			CreatedAt:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:            time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC),
		},
	}
	handler := NewTicketHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/tickets/"+validTicketID, nil)
	req.SetPathValue("id", validTicketID)

	rec := serve(t, handler.GetByID, req)

	assertJSONResponse(t, rec, http.StatusOK, map[string]any{
		"id":                     validTicketID,
		"title":                  "Login fails",
		"description":            "Cannot log in with a valid password",
		"status":                 "in_progress",
		"latest_triage_category": "authentication",
		"latest_triage_priority": "high",
		"created_at":             "2026-01-02T03:04:05Z",
		"updated_at":             "2026-01-03T04:05:06Z",
	})

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}

	if service.gotID.String() != validTicketID {
		t.Errorf("service got ticket ID %q, want %q", service.gotID, validTicketID)
	}
}

func TestTicketHandlerList_ReturnsTickets(t *testing.T) {
	const otherTicketID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

	category := "authentication"
	priority := "high"

	// Like the real repository, the fake returns the newest ticket first.
	// The handler must keep that order and not sort the list itself.
	service := &fakeTicketService{
		tickets: []models.Ticket{
			{
				ID:                   uuid.MustParse(validTicketID),
				Title:                "Login fails",
				Description:          "Cannot log in with a valid password",
				Status:               "in_progress",
				LatestTriageCategory: &category,
				LatestTriagePriority: &priority,
				CreatedAt:            time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC),
				UpdatedAt:            time.Date(2026, 1, 4, 5, 6, 7, 0, time.UTC),
			},
			{
				ID:          uuid.MustParse(otherTicketID),
				Title:       "Invoice missing",
				Description: "The January invoice is not listed",
				Status:      "open",
				CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			},
		},
	}
	handler := NewTicketHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/tickets", nil)

	rec := serve(t, handler.List, req)

	assertJSONResponse(t, rec, http.StatusOK, []any{
		map[string]any{
			"id":                     validTicketID,
			"title":                  "Login fails",
			"description":            "Cannot log in with a valid password",
			"status":                 "in_progress",
			"latest_triage_category": "authentication",
			"latest_triage_priority": "high",
			"created_at":             "2026-01-03T04:05:06Z",
			"updated_at":             "2026-01-04T05:06:07Z",
		},
		map[string]any{
			"id":                     otherTicketID,
			"title":                  "Invoice missing",
			"description":            "The January invoice is not listed",
			"status":                 "open",
			"latest_triage_category": nil,
			"latest_triage_priority": nil,
			"created_at":             "2026-01-02T03:04:05Z",
			"updated_at":             "2026-01-02T03:04:05Z",
		},
	})

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}
}

func TestTicketHandlerList_ReturnsEmptyArray(t *testing.T) {
	// The real repository returns an empty, non-nil slice when there are no
	// tickets. A nil slice would marshal to null, so the handler must turn
	// both into [] for clients that iterate over the list.
	tests := []struct {
		name    string
		tickets []models.Ticket
	}{
		{name: "empty slice", tickets: []models.Ticket{}},
		{name: "nil slice", tickets: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeTicketService{tickets: tt.tickets}
			handler := NewTicketHandler(service)

			req := httptest.NewRequest(http.MethodGet, "/tickets", nil)

			rec := serve(t, handler.List, req)

			assertJSONResponse(t, rec, http.StatusOK, []any{})

			// []any{} and nil decode differently, but check the raw body too
			// so the contract is explicit: an empty array, never null.
			if got := rec.Body.String(); got != "[]" {
				t.Errorf("response body = %q, want %q", got, "[]")
			}

			if service.calls != 1 {
				t.Errorf("service calls = %d, want 1", service.calls)
			}
		})
	}
}

func TestTicketHandlerCreate_ReturnsCreatedTicket(t *testing.T) {
	// Like the real service, the fake returns a new open ticket with no
	// triage yet; the database would fill in the timestamps.
	service := &fakeTicketService{
		ticket: &models.Ticket{
			ID:          uuid.MustParse(validTicketID),
			Title:       "Login fails",
			Description: "Cannot log in",
			Status:      "open",
			CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	handler := NewTicketHandler(service)

	// Surrounding whitespace is trimmed before the values reach the service.
	body := strings.NewReader(createBody(t, "  Login fails \n", "\tCannot log in  "))
	req := httptest.NewRequest(http.MethodPost, "/tickets", body)

	rec := serve(t, handler.Create, req)

	assertJSONResponse(t, rec, http.StatusCreated, map[string]any{
		"id":                     validTicketID,
		"title":                  "Login fails",
		"description":            "Cannot log in",
		"status":                 "open",
		"latest_triage_category": nil,
		"latest_triage_priority": nil,
		"created_at":             "2026-01-02T03:04:05Z",
		"updated_at":             "2026-01-02T03:04:05Z",
	})

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}

	if service.gotTitle != "Login fails" {
		t.Errorf("service got title %q, want %q", service.gotTitle, "Login fails")
	}

	if service.gotDescription != "Cannot log in" {
		t.Errorf("service got description %q, want %q", service.gotDescription, "Cannot log in")
	}
}

func TestTicketHandlerUpdateStatus_ReturnsNoContent(t *testing.T) {
	for _, status := range []string{"open", "in_progress", "resolved"} {
		t.Run(status, func(t *testing.T) {
			service := &fakeTicketService{}
			handler := NewTicketHandler(service)

			body := strings.NewReader(`{"status": "` + status + `"}`)
			req := httptest.NewRequest(http.MethodPatch, "/tickets/"+validTicketID, body)
			req.SetPathValue("id", validTicketID)

			rec := serve(t, handler.UpdateStatus, req)

			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
			}

			if rec.Body.Len() != 0 {
				t.Errorf("response body = %q, want empty", rec.Body.String())
			}

			if service.calls != 1 {
				t.Errorf("service calls = %d, want 1", service.calls)
			}

			if service.gotID.String() != validTicketID {
				t.Errorf("service got ticket ID %q, want %q", service.gotID, validTicketID)
			}

			if service.gotStatus != status {
				t.Errorf("service got status %q, want %q", service.gotStatus, status)
			}
		})
	}
}
