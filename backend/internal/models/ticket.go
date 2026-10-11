package models

import (
	"time"

	"github.com/google/uuid"
)

type Ticket struct {
	ID                   uuid.UUID `json:"id"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	Status               string    `json:"status"`
	LatestTriageCategory *string   `json:"latest_triage_category"`
	LatestTriagePriority *string   `json:"latest_triage_priority"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
