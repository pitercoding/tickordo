package handlers

import (
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

			if err := decodeJSONBody(strings.NewReader(tt.body), &request); err == nil {
				t.Errorf("decodeJSONBody(%q) error = nil, want an error", tt.body)
			}
		})
	}
}
