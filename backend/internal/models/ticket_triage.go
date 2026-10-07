package models

import (
	"time"

	"github.com/google/uuid"
)

type TicketTriage struct {
	ID              uuid.UUID `json:"id"`
	TicketID        uuid.UUID `json:"ticket_id"`
	Category        string    `json:"category"`
	Priority        string    `json:"priority"`
	Sentiment       string    `json:"sentiment"`
	SuggestedTeam   string    `json:"suggested_team"`
	Summary         string    `json:"summary"`
	SuggestedAction string    `json:"suggested_action"`
	Confidence      float64   `json:"confidence"`
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
}
