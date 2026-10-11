package ai

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// validTriageResponse returns a response that passes validation, so each
// test case can break exactly one field and check that it is rejected.
func validTriageResponse() triageResponse {
	return triageResponse{
		Category:        "billing",
		Priority:        "high",
		Sentiment:       "frustrated",
		SuggestedTeam:   "Billing Support",
		Summary:         "Customer was charged twice for the same invoice.",
		SuggestedAction: "Refund the duplicate charge and confirm with the customer.",
		Confidence:      0.87,
	}
}

func TestTriageResponseValidate_ValidResponse(t *testing.T) {
	if err := validTriageResponse().validate(); err != nil {
		t.Fatalf("validate() returned unexpected error: %v", err)
	}
}

func TestTriageResponseValidate_AcceptsAllowedPriorities(t *testing.T) {
	for _, priority := range []string{"low", "medium", "high", "critical"} {
		t.Run(priority, func(t *testing.T) {
			r := validTriageResponse()
			r.Priority = priority

			if err := r.validate(); err != nil {
				t.Fatalf("validate() rejected priority %q: %v", priority, err)
			}
		})
	}
}

func TestTriageResponseValidate_AcceptsAllowedSentiments(t *testing.T) {
	for _, sentiment := range []string{"positive", "neutral", "negative", "frustrated"} {
		t.Run(sentiment, func(t *testing.T) {
			r := validTriageResponse()
			r.Sentiment = sentiment

			if err := r.validate(); err != nil {
				t.Fatalf("validate() rejected sentiment %q: %v", sentiment, err)
			}
		})
	}
}

func TestTriageResponseValidate_AcceptsConfidenceBounds(t *testing.T) {
	for _, confidence := range []float64{0, 1} {
		r := validTriageResponse()
		r.Confidence = confidence

		if err := r.validate(); err != nil {
			t.Errorf("validate() rejected confidence %v: %v", confidence, err)
		}
	}
}

func TestTriageResponseValidate_RejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *triageResponse)
		wantErr string
	}{
		{
			name:    "unknown priority",
			mutate:  func(r *triageResponse) { r.Priority = "urgent" },
			wantErr: "unexpected priority",
		},
		{
			name:    "empty priority",
			mutate:  func(r *triageResponse) { r.Priority = "" },
			wantErr: "unexpected priority",
		},
		{
			name:    "priority with different casing",
			mutate:  func(r *triageResponse) { r.Priority = "High" },
			wantErr: "unexpected priority",
		},
		{
			name:    "unknown sentiment",
			mutate:  func(r *triageResponse) { r.Sentiment = "angry" },
			wantErr: "unexpected sentiment",
		},
		{
			name:    "empty sentiment",
			mutate:  func(r *triageResponse) { r.Sentiment = "" },
			wantErr: "unexpected sentiment",
		},
		{
			name:    "empty category",
			mutate:  func(r *triageResponse) { r.Category = "" },
			wantErr: "are required",
		},
		{
			name:    "whitespace-only category",
			mutate:  func(r *triageResponse) { r.Category = "   " },
			wantErr: "are required",
		},
		{
			name:    "empty suggested_team",
			mutate:  func(r *triageResponse) { r.SuggestedTeam = "" },
			wantErr: "are required",
		},
		{
			name:    "whitespace-only suggested_team",
			mutate:  func(r *triageResponse) { r.SuggestedTeam = "\t\n " },
			wantErr: "are required",
		},
		{
			name:    "empty summary",
			mutate:  func(r *triageResponse) { r.Summary = "" },
			wantErr: "are required",
		},
		{
			name:    "whitespace-only summary",
			mutate:  func(r *triageResponse) { r.Summary = "  \n  " },
			wantErr: "are required",
		},
		{
			name:    "empty suggested_action",
			mutate:  func(r *triageResponse) { r.SuggestedAction = "" },
			wantErr: "are required",
		},
		{
			name:    "whitespace-only suggested_action",
			mutate:  func(r *triageResponse) { r.SuggestedAction = " \t " },
			wantErr: "are required",
		},
		{
			name:    "confidence below zero",
			mutate:  func(r *triageResponse) { r.Confidence = -0.01 },
			wantErr: "out of range",
		},
		{
			name:    "confidence above one",
			mutate:  func(r *triageResponse) { r.Confidence = 1.01 },
			wantErr: "out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validTriageResponse()
			tt.mutate(&r)

			err := r.validate()
			if err == nil {
				t.Fatalf("validate() returned nil, want error containing %q", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "shorter than limit is unchanged",
			s:    "billing",
			max:  10,
			want: "billing",
		},
		{
			name: "exactly at limit is unchanged",
			s:    "billing",
			max:  7,
			want: "billing",
		},
		{
			name: "longer than limit is cut to max runes",
			s:    "technical support",
			max:  9,
			want: "technical",
		},
		{
			name: "leading and trailing spaces are trimmed",
			s:    "  billing \n",
			max:  10,
			want: "billing",
		},
		{
			name: "spaces are trimmed before measuring the limit",
			s:    "   billing   ",
			max:  7,
			want: "billing",
		},
		{
			name: "whitespace-only string becomes empty",
			s:    " \t\n ",
			max:  5,
			want: "",
		},
		{
			name: "accented text at limit is unchanged",
			s:    "Atenção",
			max:  7,
			want: "Atenção",
		},
		{
			name: "accented text is cut on rune boundaries",
			s:    "Configuração de autenticação",
			max:  13,
			want: "Configuração ",
		},
		{
			name: "cut right after a multi-byte rune",
			s:    "ação rápida",
			max:  2,
			want: "aç",
		},
		{
			name: "non-latin and emoji runes are kept whole",
			s:    "日本語🎫サポート",
			max:  4,
			want: "日本語🎫",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.max)

			if got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}

			if !utf8.ValidString(got) {
				t.Fatalf("truncate(%q, %d) = %q, which is not valid UTF-8", tt.s, tt.max, got)
			}

			if n := utf8.RuneCountInString(got); n > tt.max {
				t.Fatalf("truncate(%q, %d) has %d runes, want at most %d", tt.s, tt.max, n, tt.max)
			}
		})
	}
}
