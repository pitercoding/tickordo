package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/pitercoding/tickordo/internal/models"
)

const (
	testAPIKey = "test-key"
	testModel  = "test-model"
)

// localOnlyTransport sends requests only to the test server. Any other host
// fails the test, so a misconfigured client can never reach the real API.
type localOnlyTransport struct {
	t    *testing.T
	host string
}

func (l localOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != l.host {
		l.t.Errorf("request sent to %q, want only the test server %q", req.URL.Host, l.host)
		return nil, fmt.Errorf("blocked request to non-test host %q", req.URL.Host)
	}

	return http.DefaultTransport.RoundTrip(req)
}

// testServer is a fake OpenAI API. It counts the requests it receives so
// tests can check that exactly one (or no) call was made.
type testServer struct {
	requests atomic.Int32
}

// newTestAnalyzer starts a fake OpenAI API that answers with handler and
// returns an analyzer whose real SDK client talks only to it.
func newTestAnalyzer(t *testing.T, handler http.HandlerFunc) (*OpenAIAnalyzer, *testServer) {
	t.Helper()

	ts := &testServer{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.requests.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	client := &OpenAIClient{
		Client: openai.NewClient(
			// Explicit options override OPENAI_BASE_URL and OPENAI_API_KEY
			// from the environment, so a real key is never used.
			option.WithBaseURL(srv.URL),
			option.WithAPIKey(testAPIKey),
			option.WithHTTPClient(&http.Client{
				Transport: localOnlyTransport{t: t, host: srv.Listener.Addr().String()},
			}),
			// The SDK retries 5xx responses with backoff by default; one
			// attempt keeps error tests fast and the request count exact.
			option.WithMaxRetries(0),
		),
	}

	return NewOpenAIAnalyzer(client, testModel), ts
}

func testTicket() *models.Ticket {
	return &models.Ticket{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Title:       "Charged twice",
		Description: "I was charged twice for the same invoice this month.",
		Status:      "open",
	}
}

// validTriageJSON is the structured output text the model would return.
func validTriageJSON(t *testing.T, r triageResponse) string {
	t.Helper()

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal triage response: %v", err)
	}

	return string(data)
}

// responseBody builds a Responses API body with the given status and
// message content items.
func responseBody(t *testing.T, status string, content ...map[string]any) []byte {
	t.Helper()

	output := []map[string]any{}
	if len(content) > 0 {
		output = append(output, map[string]any{
			"type":    "message",
			"id":      "msg_test",
			"role":    "assistant",
			"status":  "completed",
			"content": content,
		})
	}

	data, err := json.Marshal(map[string]any{
		"id":     "resp_test",
		"object": "response",
		"model":  testModel,
		"status": status,
		"output": output,
	})
	if err != nil {
		t.Fatalf("failed to marshal response body: %v", err)
	}

	return data
}

func outputText(text string) map[string]any {
	return map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
}

// respondWith answers every request with a 200 and the given body.
func respondWith(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}

func TestOpenAIAnalyzerAnalyze_MapsValidResponse(t *testing.T) {
	want := validTriageResponse()
	analyzer, ts := newTestAnalyzer(t, respondWith(
		responseBody(t, "completed", outputText(validTriageJSON(t, want))),
	))
	ticket := testTicket()

	got, err := analyzer.Analyze(context.Background(), ticket)
	if err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	if n := ts.requests.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}

	// ID and CreatedAt are left empty: the service and the database set them.
	wantTriage := models.TicketTriage{
		TicketID:        ticket.ID,
		Category:        want.Category,
		Priority:        want.Priority,
		Sentiment:       want.Sentiment,
		SuggestedTeam:   want.SuggestedTeam,
		Summary:         want.Summary,
		SuggestedAction: want.SuggestedAction,
		Confidence:      want.Confidence,
		Model:           testModel,
	}

	if *got != wantTriage {
		t.Errorf("Analyze() = %+v, want %+v", *got, wantTriage)
	}
}

func TestOpenAIAnalyzerAnalyze_SendsStructuredOutputRequest(t *testing.T) {
	var (
		method, path, auth string
		body               map[string]any
	)

	analyzer, _ := newTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}

		// Decoding into a map makes the checks independent of key order.
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}

		respondWith(responseBody(t, "completed", outputText(validTriageJSON(t, validTriageResponse()))))(w, r)
	})
	ticket := testTicket()

	if _, err := analyzer.Analyze(context.Background(), ticket); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	if method != http.MethodPost {
		t.Errorf("method = %q, want POST", method)
	}

	if path != "/responses" {
		t.Errorf("path = %q, want /responses", path)
	}

	if auth != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q, want the test key", auth)
	}

	if body["model"] != testModel {
		t.Errorf("model = %v, want %q", body["model"], testModel)
	}

	input, _ := body["input"].(string)
	if !strings.Contains(input, ticket.Title) || !strings.Contains(input, ticket.Description) {
		t.Errorf("input = %q, want it to contain the ticket title and description", input)
	}

	if instructions, _ := body["instructions"].(string); strings.TrimSpace(instructions) == "" {
		t.Error("instructions are missing")
	}

	// JSON numbers decode as float64 into a map.
	if body["max_output_tokens"] != float64(4096) {
		t.Errorf("max_output_tokens = %v, want 4096", body["max_output_tokens"])
	}

	format := jsonObject(t, body, "text", "format")

	if format["type"] != "json_schema" {
		t.Errorf("text.format.type = %v, want json_schema", format["type"])
	}

	if format["name"] != "ticket_triage" {
		t.Errorf("text.format.name = %v, want ticket_triage", format["name"])
	}

	if format["strict"] != true {
		t.Errorf("text.format.strict = %v, want true", format["strict"])
	}

	schema := jsonObject(t, format, "schema")

	if schema["additionalProperties"] != false {
		t.Errorf("schema.additionalProperties = %v, want false", schema["additionalProperties"])
	}

	wantFields := []string{
		"category", "priority", "sentiment", "suggested_team",
		"summary", "suggested_action", "confidence",
	}

	if got := sortedStrings(t, schema["required"]); !slices.Equal(got, sorted(wantFields)) {
		t.Errorf("schema.required = %v, want %v", got, sorted(wantFields))
	}

	properties := jsonObject(t, schema, "properties")

	for _, field := range wantFields {
		if _, ok := properties[field]; !ok {
			t.Errorf("schema.properties is missing %q", field)
		}
	}

	enums := map[string][]string{
		"priority":  {"low", "medium", "high", "critical"},
		"sentiment": {"positive", "neutral", "negative", "frustrated"},
	}

	for field, want := range enums {
		got := sortedStrings(t, jsonObject(t, properties, field)["enum"])
		if !slices.Equal(got, sorted(want)) {
			t.Errorf("schema.properties.%s.enum = %v, want %v", field, got, sorted(want))
		}
	}
}

func TestOpenAIAnalyzerAnalyze_RejectsUnusableResponses(t *testing.T) {
	validText := validTriageJSON(t, validTriageResponse())

	invalidPriority := validTriageResponse()
	invalidPriority.Priority = "urgent"

	invalidConfidence := validTriageResponse()
	invalidConfidence.Confidence = 1.5

	tests := []struct {
		name string
		body []byte
	}{
		// Status: only "completed" responses carry complete JSON.
		{name: "incomplete status", body: responseBody(t, "incomplete", outputText(validText))},
		{name: "failed status", body: responseBody(t, "failed", outputText(validText))},
		// Empty output or refusal: no output_text to decode.
		{name: "no output items", body: responseBody(t, "completed")},
		{name: "empty output text", body: responseBody(t, "completed", outputText(""))},
		{
			name: "refusal",
			body: responseBody(t, "completed", map[string]any{
				"type":    "refusal",
				"refusal": "I can't help with that.",
			}),
		},
		// Output text that is not a JSON triage.
		{name: "plain text output", body: responseBody(t, "completed", outputText("billing, high priority"))},
		{name: "truncated JSON output", body: responseBody(t, "completed", outputText(validText[:len(validText)/2]))},
		// Decoded JSON that validate() rejects; validate itself is covered
		// in detail by TestTriageResponseValidate_RejectsInvalidResponses.
		{name: "invalid priority", body: responseBody(t, "completed", outputText(validTriageJSON(t, invalidPriority)))},
		{name: "confidence out of range", body: responseBody(t, "completed", outputText(validTriageJSON(t, invalidConfidence)))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, _ := newTestAnalyzer(t, respondWith(tt.body))

			got, err := analyzer.Analyze(context.Background(), testTicket())
			if err == nil {
				t.Fatalf("Analyze() = %+v, want an error", got)
			}

			if got != nil {
				t.Errorf("Analyze() returned %+v alongside the error, want nil", got)
			}
		})
	}
}

func TestOpenAIAnalyzerAnalyze_RejectsResponseCutByMaxOutputTokens(t *testing.T) {
	validText := validTriageJSON(t, validTriageResponse())

	// The token limit can stop generation mid-JSON or before any output.
	tests := []struct {
		name    string
		content []map[string]any
	}{
		{name: "truncated JSON output", content: []map[string]any{outputText(validText[:len(validText)/2])}},
		{name: "no output items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal(responseBody(t, "incomplete", tt.content...), &body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			body["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}

			data, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("failed to marshal response body: %v", err)
			}

			analyzer, _ := newTestAnalyzer(t, respondWith(data))

			got, err := analyzer.Analyze(context.Background(), testTicket())
			if err == nil {
				t.Fatalf("Analyze() = %+v, want an error", got)
			}

			if got != nil {
				t.Errorf("Analyze() returned %+v alongside the error, want nil", got)
			}

			if !strings.Contains(err.Error(), "max_output_tokens") {
				t.Errorf("error = %v, want it to mention the max_output_tokens reason", err)
			}
		})
	}
}

func TestOpenAIAnalyzerAnalyze_TruncatesLongFields(t *testing.T) {
	tests := []struct {
		name     string
		category string
		team     string
		wantCat  string
		wantTeam string
	}{
		{
			name:     "exactly at the limits is unchanged",
			category: strings.Repeat("c", 50),
			team:     strings.Repeat("t", 100),
			wantCat:  strings.Repeat("c", 50),
			wantTeam: strings.Repeat("t", 100),
		},
		{
			name:     "one over the limits is cut",
			category: strings.Repeat("c", 50) + "X",
			team:     strings.Repeat("t", 100) + "X",
			wantCat:  strings.Repeat("c", 50),
			wantTeam: strings.Repeat("t", 100),
		},
		{
			name:     "multi-byte runes are counted and cut whole",
			category: strings.Repeat("ç", 60),
			team:     strings.Repeat("🎫", 120),
			wantCat:  strings.Repeat("ç", 50),
			wantTeam: strings.Repeat("🎫", 100),
		},
		{
			name:     "surrounding spaces are trimmed before the limit",
			category: "  " + strings.Repeat("c", 50) + "  ",
			team:     "\n" + strings.Repeat("t", 100) + "\t",
			wantCat:  strings.Repeat("c", 50),
			wantTeam: strings.Repeat("t", 100),
		},
		{
			name:     "short values are only trimmed",
			category: " billing ",
			team:     " Billing Support ",
			wantCat:  "billing",
			wantTeam: "Billing Support",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validTriageResponse()
			r.Category = tt.category
			r.SuggestedTeam = tt.team

			analyzer, _ := newTestAnalyzer(t, respondWith(
				responseBody(t, "completed", outputText(validTriageJSON(t, r))),
			))

			got, err := analyzer.Analyze(context.Background(), testTicket())
			if err != nil {
				t.Fatalf("Analyze() returned unexpected error: %v", err)
			}

			if got.Category != tt.wantCat {
				t.Errorf("Category = %q (%d runes), want %q (%d runes)",
					got.Category, utf8.RuneCountInString(got.Category),
					tt.wantCat, utf8.RuneCountInString(tt.wantCat))
			}

			if got.SuggestedTeam != tt.wantTeam {
				t.Errorf("SuggestedTeam = %q (%d runes), want %q (%d runes)",
					got.SuggestedTeam, utf8.RuneCountInString(got.SuggestedTeam),
					tt.wantTeam, utf8.RuneCountInString(tt.wantTeam))
			}

			// Summary and SuggestedAction have no limit and are not truncated.
			if got.Summary != r.Summary || got.SuggestedAction != r.SuggestedAction {
				t.Errorf("Summary/SuggestedAction changed: got %q / %q", got.Summary, got.SuggestedAction)
			}
		})
	}
}

func TestOpenAIAnalyzerAnalyze_ReturnsAPIErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "400 bad request", status: http.StatusBadRequest},
		{name: "401 unauthorized", status: http.StatusUnauthorized},
		{name: "500 internal server error", status: http.StatusInternalServerError},
		{name: "503 service unavailable", status: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, ts := newTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"error": {"message": "simulated error", "type": "test_error"}}`))
			})

			got, err := analyzer.Analyze(context.Background(), testTicket())
			if err == nil {
				t.Fatalf("Analyze() = %+v, want an error", got)
			}

			// Analyze wraps the SDK error with %w, so the status stays reachable.
			var apiErr *openai.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error %v does not wrap *openai.Error", err)
			}

			if apiErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
			}

			if n := ts.requests.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}

func TestOpenAIAnalyzerAnalyze_StopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// release unblocks the handler when the test ends. Cleanups run in
	// reverse order, so this one runs before the server's Close, which
	// waits for every handler to return.
	release := make(chan struct{})

	// The handler cancels the caller's context while the request is in
	// flight, then waits for the client to disconnect, like a slow API.
	analyzer, _ := newTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
		// net/http only notices a client disconnect after the request body
		// has been read, so drain it before waiting on r.Context().
		io.Copy(io.Discard, r.Body)
		cancel()

		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })

	got, err := analyzer.Analyze(ctx, testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want it to wrap context.Canceled", err)
	}
}

func TestOpenAIAnalyzerAnalyze_RejectsNilTicket(t *testing.T) {
	analyzer, ts := newTestAnalyzer(t, respondWith(nil))

	if _, err := analyzer.Analyze(context.Background(), nil); err == nil {
		t.Fatal("Analyze(nil) returned nil error, want an error")
	}

	if n := ts.requests.Load(); n != 0 {
		t.Errorf("server received %d requests, want none for a nil ticket", n)
	}
}

// jsonObject follows keys through nested JSON objects, failing the test when
// a key is missing or is not an object.
func jsonObject(t *testing.T, obj map[string]any, keys ...string) map[string]any {
	t.Helper()

	for _, key := range keys {
		next, ok := obj[key].(map[string]any)
		if !ok {
			t.Fatalf("%q is %T, want a JSON object", key, obj[key])
		}

		obj = next
	}

	return obj
}

// sortedStrings converts a decoded JSON array of strings to a sorted slice,
// so the comparison does not depend on array order.
func sortedStrings(t *testing.T, value any) []string {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value is %T, want a JSON array", value)
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("array item is %T, want a string", item)
		}

		out = append(out, s)
	}

	return sorted(out)
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)

	return out
}
