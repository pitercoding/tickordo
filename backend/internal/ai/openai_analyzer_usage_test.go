package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

const (
	usageLogPrefix = "openai usage: "

	// testServedModel differs from testModel so the tests can tell that the
	// log uses the model returned by the API, not the configured one.
	testServedModel = "test-model-2026-01-01"
)

// captureLog redirects the global logger to a buffer until the test ends.
// Tests that use it must not call t.Parallel.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })

	return &buf
}

// responseBodyWithUsage builds a Responses API body like responseBody, with
// the given usage object and testServedModel as the model.
func responseBodyWithUsage(t *testing.T, status string, usage map[string]any, content ...map[string]any) []byte {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(responseBody(t, status, content...), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	body["model"] = testServedModel
	body["usage"] = usage

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal response body: %v", err)
	}

	return data
}

func fullUsage() map[string]any {
	return map[string]any{
		"input_tokens": 1234,
		"input_tokens_details": map[string]any{
			"cached_tokens":      200,
			"cache_write_tokens": 30,
		},
		"output_tokens": 456,
		"output_tokens_details": map[string]any{
			"reasoning_tokens": 300,
		},
		"total_tokens": 1690,
	}
}

// usageLogFields finds the single usage line in the log and returns its
// key=value fields, with quoted values unquoted.
func usageLogFields(t *testing.T, logs string) map[string]string {
	t.Helper()

	var lines []string
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, usageLogPrefix) {
			lines = append(lines, line)
		}
	}

	if len(lines) != 1 {
		t.Fatalf("found %d usage log lines, want 1; log:\n%s", len(lines), logs)
	}

	_, rest, _ := strings.Cut(lines[0], usageLogPrefix)
	fields := map[string]string{}

	for _, pair := range strings.Fields(rest) {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			t.Fatalf("usage log field %q is not key=value", pair)
		}

		if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				t.Fatalf("usage log field %q has an invalid quoted value: %v", pair, err)
			}

			value = unquoted
		}

		fields[key] = value
	}

	return fields
}

func checkUsageFields(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()

	for key, wantValue := range want {
		if gotValue, ok := got[key]; !ok {
			t.Errorf("usage log is missing %q", key)
		} else if gotValue != wantValue {
			t.Errorf("usage log %s = %q, want %q", key, gotValue, wantValue)
		}
	}
}

var usageMetrics = []string{
	"input_tokens",
	"cached_input_tokens",
	"cache_write_tokens",
	"output_tokens",
	"reasoning_tokens",
	"total_tokens",
}

func TestOpenAIAnalyzerAnalyze_LogsFullUsage(t *testing.T) {
	logs := captureLog(t)
	analyzer, _ := newTestAnalyzer(t, respondWith(responseBodyWithUsage(
		t, "completed", fullUsage(), outputText(validTriageJSON(t, validTriageResponse())),
	)))
	ticket := testTicket()

	if _, err := analyzer.Analyze(context.Background(), ticket); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	checkUsageFields(t, usageLogFields(t, logs.String()), map[string]string{
		"ticket_id":           ticket.ID.String(),
		"response_id":         "resp_test",
		"model":               testServedModel,
		"status":              "completed",
		"input_tokens":        "1234",
		"cached_input_tokens": "200",
		"cache_write_tokens":  "30",
		// Reasoning tokens are part of output_tokens, not added to it.
		"output_tokens":    "456",
		"reasoning_tokens": "300",
		"total_tokens":     "1690",
	})
}

func TestOpenAIAnalyzerAnalyze_UsageLogOmitsContent(t *testing.T) {
	logs := captureLog(t)
	triage := validTriageResponse()
	analyzer, _ := newTestAnalyzer(t, respondWith(responseBodyWithUsage(
		t, "completed", fullUsage(), outputText(validTriageJSON(t, triage)),
	)))
	ticket := testTicket()

	if _, err := analyzer.Analyze(context.Background(), ticket); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	got := logs.String()

	if !strings.Contains(got, usageLogPrefix) {
		t.Fatalf("no usage log line; log:\n%s", got)
	}

	forbidden := map[string]string{
		"title":            ticket.Title,
		"description":      ticket.Description,
		"summary":          triage.Summary,
		"suggested action": triage.SuggestedAction,
		"suggested team":   triage.SuggestedTeam,
		"API key":          testAPIKey,
	}

	for name, value := range forbidden {
		if strings.Contains(got, value) {
			t.Errorf("usage log contains the %s %q; log:\n%s", name, value, got)
		}
	}
}

func TestOpenAIAnalyzerAnalyze_LogsMissingUsageAsUnknown(t *testing.T) {
	logs := captureLog(t)
	want := validTriageResponse()

	// responseBody sends no usage object at all.
	analyzer, _ := newTestAnalyzer(t, respondWith(
		responseBody(t, "completed", outputText(validTriageJSON(t, want))),
	))

	got, err := analyzer.Analyze(context.Background(), testTicket())
	if err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	if got.Category != want.Category || got.Priority != want.Priority {
		t.Errorf("Analyze() = %+v, want the decoded triage", got)
	}

	wantFields := map[string]string{}
	for _, metric := range usageMetrics {
		wantFields[metric] = "unknown"
	}

	checkUsageFields(t, usageLogFields(t, logs.String()), wantFields)
}

func TestOpenAIAnalyzerAnalyze_LogsPartialUsage(t *testing.T) {
	logs := captureLog(t)

	// cache_write_tokens and output_tokens_details are absent, total_tokens
	// is null, and the explicit zeros must stay zeros.
	usage := map[string]any{
		"input_tokens": 0,
		"input_tokens_details": map[string]any{
			"cached_tokens": 0,
		},
		"output_tokens": 42,
		"total_tokens":  nil,
	}

	analyzer, _ := newTestAnalyzer(t, respondWith(responseBodyWithUsage(
		t, "completed", usage, outputText(validTriageJSON(t, validTriageResponse())),
	)))

	if _, err := analyzer.Analyze(context.Background(), testTicket()); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	checkUsageFields(t, usageLogFields(t, logs.String()), map[string]string{
		"input_tokens":        "0",
		"cached_input_tokens": "0",
		"cache_write_tokens":  "unknown",
		"output_tokens":       "42",
		"reasoning_tokens":    "unknown",
		"total_tokens":        "unknown",
	})
}

func TestOpenAIAnalyzerAnalyze_LogsUsageOfIncompleteResponse(t *testing.T) {
	logs := captureLog(t)
	validText := validTriageJSON(t, validTriageResponse())

	analyzer, _ := newTestAnalyzer(t, respondWith(responseBodyWithUsage(
		t, "incomplete", fullUsage(), outputText(validText[:len(validText)/2]),
	)))

	got, err := analyzer.Analyze(context.Background(), testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	if !strings.Contains(err.Error(), "not completed") {
		t.Errorf("error = %v, want the not completed error", err)
	}

	checkUsageFields(t, usageLogFields(t, logs.String()), map[string]string{
		"status":        "incomplete",
		"output_tokens": "456",
	})
}

func TestOpenAIAnalyzerAnalyze_NoUsageLogOnAPIError(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			logs := captureLog(t)
			analyzer, _ := newTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				w.Write([]byte(`{"error": {"message": "simulated error", "type": "test_error"}}`))
			})

			if _, err := analyzer.Analyze(context.Background(), testTicket()); err == nil {
				t.Fatal("Analyze() returned nil error, want an error")
			}

			if strings.Contains(logs.String(), usageLogPrefix) {
				t.Errorf("usage was logged for a failed request; log:\n%s", logs.String())
			}
		})
	}
}
