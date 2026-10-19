package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// retryServer is a fake OpenAI API that records the SDK retry count header
// of every request it receives.
type retryServer struct {
	mu          sync.Mutex
	retryCounts []string
}

func (s *retryServer) attempts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.retryCounts)
}

// failFirstTransport fails the first round trip without touching the
// network, as a dropped connection would, and sends the rest to next.
type failFirstTransport struct {
	next   http.RoundTripper
	failed atomic.Bool
}

func (f *failFirstTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.failed.CompareAndSwap(false, true) {
		if req.Body != nil {
			req.Body.Close()
		}

		return nil, errors.New("simulated connection reset")
	}

	return f.next.RoundTrip(req)
}

// newRetryTestAnalyzer returns an analyzer built with NewOpenAIClient, so it
// uses the production retry policy, talking only to a local fake API.
// wrap, when not nil, wraps the local-only transport.
func newRetryTestAnalyzer(
	t *testing.T,
	handler http.HandlerFunc,
	wrap func(http.RoundTripper) http.RoundTripper,
) (*OpenAIAnalyzer, *retryServer) {
	t.Helper()

	rs := &retryServer{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.retryCounts = append(rs.retryCounts, r.Header.Get("X-Stainless-Retry-Count"))
		rs.mu.Unlock()

		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	var transport http.RoundTripper = localOnlyTransport{t: t, host: srv.Listener.Addr().String()}
	if wrap != nil {
		transport = wrap(transport)
	}

	client := NewOpenAIClient(
		testAPIKey,
		// Explicit options override OPENAI_BASE_URL from the environment,
		// so a real endpoint is never used.
		option.WithBaseURL(srv.URL),
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)

	return NewOpenAIAnalyzer(client, testModel, ""), rs
}

// respondError answers with an OpenAI-style JSON error and the given headers.
func respondError(status int, headers map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(`{"error":{"message":"test error","type":"test_error","code":null,"param":null}}`))
	}
}

// assertAPIError checks that err wraps an *openai.Error with status.
func assertAPIError(t *testing.T, err error, status int) {
	t.Helper()

	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want an *openai.Error", err)
	}

	if apiErr.StatusCode != status {
		t.Errorf("status code = %d, want %d", apiErr.StatusCode, status)
	}
}

func assertAttempts(t *testing.T, rs *retryServer, want []string) {
	t.Helper()

	if got := rs.attempts(); !slices.Equal(got, want) {
		t.Errorf("X-Stainless-Retry-Count per request = %v, want %v", got, want)
	}
}

func TestOpenAIClient_RetriesPersistentServerErrorOnce(t *testing.T) {
	analyzer, rs := newRetryTestAnalyzer(t, respondError(http.StatusInternalServerError, nil), nil)

	got, err := analyzer.Analyze(context.Background(), testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	assertAPIError(t, err, http.StatusInternalServerError)
	assertAttempts(t, rs, []string{"0", "1"})
}

func TestOpenAIClient_RecoversFromTransientServerError(t *testing.T) {
	var calls atomic.Int32

	valid := respondWith(responseBody(t, "completed", outputText(validTriageJSON(t, validTriageResponse()))))
	unavailable := respondError(http.StatusServiceUnavailable, nil)

	analyzer, rs := newRetryTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			unavailable(w, r)
			return
		}

		valid(w, r)
	}, nil)

	if _, err := analyzer.Analyze(context.Background(), testTicket()); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	assertAttempts(t, rs, []string{"0", "1"})
}

func TestOpenAIClient_RetriesRateLimitWithShortRetryAfterOnce(t *testing.T) {
	analyzer, rs := newRetryTestAnalyzer(t, respondError(http.StatusTooManyRequests, map[string]string{
		"Retry-After-Ms": "10",
	}), nil)

	got, err := analyzer.Analyze(context.Background(), testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	assertAPIError(t, err, http.StatusTooManyRequests)
	assertAttempts(t, rs, []string{"0", "1"})
}

func TestOpenAIClient_DoesNotWaitForRetryAfterAboveMaxDelay(t *testing.T) {
	analyzer, rs := newRetryTestAnalyzer(t, respondError(http.StatusTooManyRequests, map[string]string{
		"Retry-After": "30",
	}), nil)

	start := time.Now()
	got, err := analyzer.Analyze(context.Background(), testTicket())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want the original 429 instead of a deadline error", err)
	}

	assertAPIError(t, err, http.StatusTooManyRequests)
	assertAttempts(t, rs, []string{"0"})

	// Generous bound: the call must not wait for Retry-After or the timeout.
	if elapsed > 5*time.Second {
		t.Errorf("Analyze() took %s, want it to return without waiting", elapsed)
	}
}

func TestOpenAIClient_DoesNotRetryClientErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			analyzer, rs := newRetryTestAnalyzer(t, respondError(status, nil), nil)

			got, err := analyzer.Analyze(context.Background(), testTicket())
			if err == nil {
				t.Fatalf("Analyze() = %+v, want an error", got)
			}

			assertAPIError(t, err, status)
			assertAttempts(t, rs, []string{"0"})
		})
	}
}

func TestOpenAIClient_DoesNotRetryIncompleteResponse(t *testing.T) {
	body := []byte(`{"id":"resp_test","object":"response","model":"` + testModel + `",` +
		`"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`)

	analyzer, rs := newRetryTestAnalyzer(t, respondWith(body), nil)

	got, err := analyzer.Analyze(context.Background(), testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	assertAttempts(t, rs, []string{"0"})
}

func TestOpenAIClient_RetriesTransportFailureOnce(t *testing.T) {
	transport := &failFirstTransport{}

	analyzer, rs := newRetryTestAnalyzer(
		t,
		respondWith(responseBody(t, "completed", outputText(validTriageJSON(t, validTriageResponse())))),
		func(next http.RoundTripper) http.RoundTripper {
			transport.next = next
			return transport
		},
	)

	if _, err := analyzer.Analyze(context.Background(), testTicket()); err != nil {
		t.Fatalf("Analyze() returned unexpected error: %v", err)
	}

	if !transport.failed.Load() {
		t.Error("the first round trip was not failed by the test transport")
	}

	// The failed attempt never reached the server, so only the retry did.
	assertAttempts(t, rs, []string{"1"})
}

func TestOpenAIClient_StopsAtContextDeadline(t *testing.T) {
	analyzer, rs := newRetryTestAnalyzer(t, func(w http.ResponseWriter, r *http.Request) {
		// The server notices a closed connection only after the body is
		// read, so drain it before holding the request until the client
		// gives up; the fallback only keeps a broken test from hanging.
		io.Copy(io.Discard, r.Body)

		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	got, err := analyzer.Analyze(ctx, testTicket())
	if err == nil {
		t.Fatalf("Analyze() = %+v, want an error", got)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}

	assertAttempts(t, rs, []string{"0"})
}

func TestNewOpenAIClient_ExtraOptionsCannotDisableRetries(t *testing.T) {
	rs := &retryServer{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.retryCounts = append(rs.retryCounts, r.Header.Get("X-Stainless-Retry-Count"))
		rs.mu.Unlock()

		respondError(http.StatusInternalServerError, nil)(w, r)
	}))
	t.Cleanup(srv.Close)

	client := NewOpenAIClient(
		testAPIKey,
		option.WithBaseURL(srv.URL),
		option.WithHTTPClient(&http.Client{
			Transport: localOnlyTransport{t: t, host: srv.Listener.Addr().String()},
		}),
		option.WithMaxRetries(0),
	)

	if _, err := NewOpenAIAnalyzer(client, testModel, "").Analyze(context.Background(), testTicket()); err == nil {
		t.Fatal("Analyze() returned no error, want one")
	}

	assertAttempts(t, rs, []string{"0", "1"})
}
