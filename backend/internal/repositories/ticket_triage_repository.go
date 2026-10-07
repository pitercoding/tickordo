package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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

func (r *TicketTriageRepository) ListByTicketID(
	ctx context.Context,
	ticketID uuid.UUID,
) ([]*models.TicketTriage, error) {
	query := `
		SELECT
			id,
			ticket_id,
			category,
			priority,
			sentiment,
			suggested_team,
			summary,
			suggested_action,
			confidence,
			model,
			created_at
		FROM ticket_triages
		WHERE ticket_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ticket triages: %w", err)
	}
	defer rows.Close()

	triages := make([]*models.TicketTriage, 0)

	for rows.Next() {
		triage := &models.TicketTriage{}

		if err := rows.Scan(
			&triage.ID,
			&triage.TicketID,
			&triage.Category,
			&triage.Priority,
			&triage.Sentiment,
			&triage.SuggestedTeam,
			&triage.Summary,
			&triage.SuggestedAction,
			&triage.Confidence,
			&triage.Model,
			&triage.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket triage: %w", err)
		}

		triages = append(triages, triage)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate ticket triages: %w", err)
	}

	return triages, nil
}
