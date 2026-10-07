package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/pitercoding/tickordo/internal/models"
)

const (
	// analyzeTimeout limits how long a single OpenAI triage call may take.
	analyzeTimeout = 60 * time.Second

	// maxCategoryLength matches the VARCHAR(50) limit of ticket_triages.category.
	maxCategoryLength = 50

	// maxSuggestedTeamLength matches the VARCHAR(100) limit of ticket_triages.suggested_team.
	maxSuggestedTeamLength = 100
)

type OpenAIAnalyzer struct {
	client *OpenAIClient
	model  string
}

type triageResponse struct {
	Category        string  `json:"category"`
	Priority        string  `json:"priority"`
	Sentiment       string  `json:"sentiment"`
	SuggestedTeam   string  `json:"suggested_team"`
	Summary         string  `json:"summary"`
	SuggestedAction string  `json:"suggested_action"`
	Confidence      float64 `json:"confidence"`
}

func NewOpenAIAnalyzer(client *OpenAIClient, model string) *OpenAIAnalyzer {
	return &OpenAIAnalyzer{
		client: client,
		model:  model,
	}
}

func (a *OpenAIAnalyzer) Analyze(
	ctx context.Context,
	ticket *models.Ticket,
) (*models.TicketTriage, error) {
	if ticket == nil {
		return nil, fmt.Errorf("ticket cannot be nil")
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{
				"type":        "string",
				"description": "The main category of the support ticket, as a short label of at most 50 characters.",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high", "critical"},
				"description": "The urgency of the support ticket.",
			},
			"sentiment": map[string]any{
				"type":        "string",
				"enum":        []string{"positive", "neutral", "negative", "frustrated"},
				"description": "The customer's emotional sentiment.",
			},
			"suggested_team": map[string]any{
				"type":        "string",
				"description": "The support team that should handle the ticket, at most 100 characters.",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "A concise summary of the customer's issue.",
			},
			"suggested_action": map[string]any{
				"type":        "string",
				"description": "The recommended next action for the support agent.",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "The model's confidence in the overall triage, from 0 to 1.",
			},
		},
		"required": []string{
			"category",
			"priority",
			"sentiment",
			"suggested_team",
			"summary",
			"suggested_action",
			"confidence",
		},
		"additionalProperties": false,
	}

	input := fmt.Sprintf(
		"Analyze the following customer support ticket.\n\nTitle: %s\n\nDescription: %s",
		ticket.Title,
		ticket.Description,
	)

	params := responses.ResponseNewParams{
		Model: openai.ResponsesModel(a.model),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input),
		},
		Instructions: openai.String(
			"You are a support ticket triage assistant. " +
				"Analyze the ticket and classify it accurately. " +
				"Return only the requested structured data.",
		),
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:        "ticket_triage",
					Description: openai.String("Structured support ticket triage result."),
					Schema:      schema,
					Strict:      openai.Bool(true),
				},
			},
		},
	}

	// Bound the OpenAI call so a slow response cannot hang the request.
	ctx, cancel := context.WithTimeout(ctx, analyzeTimeout)
	defer cancel()

	response, err := a.client.Client.Responses.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze ticket with OpenAI: %w", err)
	}

	// An incomplete response (e.g. token limit reached) has truncated JSON.
	if response.Status != responses.ResponseStatusCompleted {
		return nil, fmt.Errorf("OpenAI triage response not completed: status %q", response.Status)
	}

	// A refusal produces no output text instead of the structured data.
	outputText := response.OutputText()
	if outputText == "" {
		return nil, fmt.Errorf("OpenAI triage response has no output text")
	}

	var result triageResponse

	if err := json.Unmarshal([]byte(outputText), &result); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI triage response: %w", err)
	}

	if err := result.validate(); err != nil {
		return nil, fmt.Errorf("invalid OpenAI triage response: %w", err)
	}

	return &models.TicketTriage{
		TicketID:        ticket.ID,
		Category:        truncate(result.Category, maxCategoryLength),
		Priority:        result.Priority,
		Sentiment:       result.Sentiment,
		SuggestedTeam:   truncate(result.SuggestedTeam, maxSuggestedTeamLength),
		Summary:         result.Summary,
		SuggestedAction: result.SuggestedAction,
		Confidence:      result.Confidence,
		Model:           a.model,
	}, nil
}

// validate checks the fields that the JSON schema cannot fully guarantee.
func (r triageResponse) validate() error {
	switch r.Priority {
	case "low", "medium", "high", "critical":
	default:
		return fmt.Errorf("unexpected priority %q", r.Priority)
	}

	switch r.Sentiment {
	case "positive", "neutral", "negative", "frustrated":
	default:
		return fmt.Errorf("unexpected sentiment %q", r.Sentiment)
	}

	if strings.TrimSpace(r.Category) == "" ||
		strings.TrimSpace(r.SuggestedTeam) == "" ||
		strings.TrimSpace(r.Summary) == "" ||
		strings.TrimSpace(r.SuggestedAction) == "" {
		return fmt.Errorf("category, suggested_team, summary and suggested_action are required")
	}

	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("confidence %v is out of range [0, 1]", r.Confidence)
	}

	return nil
}

// truncate shortens s to at most max runes so it fits its VARCHAR column.
func truncate(s string, max int) string {
	s = strings.TrimSpace(s)

	if utf8.RuneCountInString(s) <= max {
		return s
	}

	return string([]rune(s)[:max])
}
