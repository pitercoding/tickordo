package handlers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBody_AcceptsSingleJSONValue(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "compact object", body: `{"status":"open"}`},
		{name: "trailing space", body: `{"status":"open"} `},
		{name: "trailing newlines and tabs", body: "{\"status\":\"open\"}\n\t\r\n  "},
		{name: "leading and trailing whitespace", body: "\n  {\"status\":\"open\"}\n"},
		{name: "whitespace inside the object", body: "{\n  \"status\" : \"open\"\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request updateTicketStatusRequest

			if err := decodeJSONBody(strings.NewReader(tt.body), &request); err != nil {
				t.Fatalf("decodeJSONBody(%q) error = %v, want nil", tt.body, err)
			}

			if request.Status != "open" {
				t.Errorf("Status = %q, want %q", request.Status, "open")
			}
		})
	}
}

func TestDecodeJSONBody_RejectsInvalidBodies(t *testing.T) {
	const valid = `{"status":"open"}`

	tests := []struct {
		name string
		body string
	}{
		// Not a single complete JSON value.
		{name: "empty body", body: ""},
		{name: "whitespace only", body: " \n\t "},
		{name: "malformed JSON", body: `{"status":"open"`},

		// Unknown fields.
		{name: "unknown field", body: `{"status":"open","unexpected":"value"}`},
		{name: "only an unknown field", body: `{"unexpected":"value"}`},
		{name: "misspelled field", body: `{"stauts":"open"}`},

		// Content after the first value.
		{name: "trailing invalid text", body: valid + " lixo"},
		{name: "two consecutive objects", body: valid + valid},
		{name: "two objects separated by a space", body: valid + " " + valid},
		{name: "two objects separated by a newline", body: valid + "\n" + valid},
		{name: "object followed by a number", body: valid + " 1"},
		{name: "object followed by a string", body: valid + ` "extra"`},
		{name: "object followed by null", body: valid + " null"},
		{name: "object followed by an array", body: valid + " []"},
		// decoder.More() reports false for these, so a More()-based check
		// would wrongly accept them.
		{name: "object followed by a closing bracket", body: valid + "]"},
		{name: "object followed by a closing brace", body: valid + "}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request updateTicketStatusRequest

			err := decodeJSONBody(limitedBody(tt.body), &request)
			if err == nil {
				t.Fatalf("decodeJSONBody(%q) error = nil, want an error", tt.body)
			}

			// These bodies are far below the cap, so the handlers must
			// answer 400, not 413.
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				t.Errorf("decodeJSONBody(%q) error = %v, want a non-size error", tt.body, err)
			}
		})
	}
}

// limitedBody wraps body in http.MaxBytesReader with maxRequestBodyBytes,
// the same cap the ticket handlers apply.
func limitedBody(body string) io.Reader {
	return http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(strings.NewReader(body)), maxRequestBodyBytes)
}

func TestDecodeJSONBody_AcceptsValidBodyWithinLimit(t *testing.T) {
	const valid = `{"status":"open"}`

	tests := []struct {
		name string
		body string
	}{
		{name: "small body", body: valid},
		{name: "body of exactly the limit", body: valid + strings.Repeat(" ", maxRequestBodyBytes-len(valid))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request updateTicketStatusRequest

			if err := decodeJSONBody(limitedBody(tt.body), &request); err != nil {
				t.Fatalf("decodeJSONBody error = %v, want nil", err)
			}

			if request.Status != "open" {
				t.Errorf("Status = %q, want %q", request.Status, "open")
			}
		})
	}
}

func TestDecodeJSONBody_ReturnsMaxBytesErrorOverLimit(t *testing.T) {
	const valid = `{"status":"open"}`

	tests := []struct {
		name string
		body string
	}{
		{
			// The limit is crossed inside the first value.
			name: "value one byte over the limit",
			body: `{"status":"` + strings.Repeat("a", maxRequestBodyBytes-len(`{"status":""}`)+1) + `"}`,
		},
		{
			// The value fits, but the trailing whitespace crosses the limit.
			// This must not be reported as trailing data.
			name: "trailing whitespace one byte over the limit",
			body: valid + strings.Repeat(" ", maxRequestBodyBytes-len(valid)+1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.body) != maxRequestBodyBytes+1 {
				t.Fatalf("body is %d bytes, want %d", len(tt.body), maxRequestBodyBytes+1)
			}

			var request updateTicketStatusRequest

			err := decodeJSONBody(limitedBody(tt.body), &request)

			var maxBytesErr *http.MaxBytesError
			if !errors.As(err, &maxBytesErr) {
				t.Fatalf("decodeJSONBody error = %v, want *http.MaxBytesError", err)
			}

			if maxBytesErr.Limit != maxRequestBodyBytes {
				t.Errorf("Limit = %d, want %d", maxBytesErr.Limit, maxRequestBodyBytes)
			}
		})
	}
}
