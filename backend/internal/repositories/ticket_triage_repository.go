package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pitercoding/tickordo/internal/models"
)

type TicketTriageRepository struct {
	db *pgxpool.Pool
}

func NewTicketTriageRepository(db *pgxpool.Pool) *TicketTriageRepository {
	return &TicketTriageRepository{
		db: db,
	}
}

func (r *TicketTriageRepository) Create(
	ctx context.Context,
	triage *models.TicketTriage,
) error {
	query := `
		INSERT INTO ticket_triages (
			id,
			ticket_id,
			category,
			priority,
			sentiment,
			suggested_team,
			summary,
			suggested_action,
			confidence,
			model
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		triage.ID,
		triage.TicketID,
		triage.Category,
		triage.Priority,
		triage.Sentiment,
		triage.SuggestedTeam,
		triage.Summary,
		triage.SuggestedAction,
		triage.Confidence,
		triage.Model,
	).Scan(&triage.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create ticket triage: %w", err)
	}

	return nil
}
