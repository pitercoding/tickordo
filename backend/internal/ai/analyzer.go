package ai

import (
	"context"

	"github.com/pitercoding/tickordo/internal/models"
)

type TicketAnalyzer interface {
	Analyze(ctx context.Context, ticket *models.Ticket) (*models.TicketTriage, error)
}
