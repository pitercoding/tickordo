package ai

import (
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	// maxRetries limits the SDK to one extra attempt, so a single triage
	// sends at most two HTTP requests to OpenAI.
	maxRetries = 1

	// maxRetryDelay caps the SDK wait before a retry. A longer Retry-After
	// stops retrying and returns the original error instead of waiting.
	maxRetryDelay = 5 * time.Second
)

type OpenAIClient struct {
	Client openai.Client
}

// NewOpenAIClient returns a client with the triage retry policy. Extra opts
// are applied before the retry options, so they cannot override the policy.
func NewOpenAIClient(apiKey string, opts ...option.RequestOption) *OpenAIClient {
	options := append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	options = append(options,
		option.WithMaxRetries(maxRetries),
		option.WithMaxRetryDelay(maxRetryDelay),
	)

	client := openai.NewClient(options...)

	return &OpenAIClient{
		Client: client,
	}
}
