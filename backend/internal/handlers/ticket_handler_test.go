package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
func serve(t *testing.T, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("request reached the service layer (panic: %v); validation should have rejected it", p)
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

// oversizedBody returns a syntactically valid JSON object for the given field
// that is larger than maxRequestBodyBytes. Without the size cap it would
// decode fine, so a decode error can only come from the cap.
func oversizedBody(field string) io.Reader {
	var b bytes.Buffer

	b.WriteString(`{"` + field + `":"`)
	b.WriteString(strings.Repeat("a", maxRequestBodyBytes))
	b.WriteString(`"}`)

	return &b
}

func TestTicketHandlerCreate_RejectsInvalidRequests(t *testing.T) {
	const (
		invalidBody     = "invalid request body"
		requiredFields  = "title and description are required"
		titleTooLong    = "title must be at most 255 characters"
		descriptionLong = "description must be at most 5000 characters"
	)

	// Some cases pass the title checks on purpose and only break the
	// description length (the last check before the service). Getting the
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
		{
			name:        "body larger than 1 MB",
			body:        func(t *testing.T) io.Reader { return oversizedBody("title") },
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
		{name: "body larger than 1 MB", body: oversizedBody("status"), wantMessage: invalidBody},

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
