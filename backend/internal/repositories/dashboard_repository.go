package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pitercoding/tickordo/internal/models"
)

type DashboardRepository struct {
	db *pgxpool.Pool
}

func NewDashboardRepository(db *pgxpool.Pool) *DashboardRepository {
	return &DashboardRepository{
		db: db,
	}
}

func (r *DashboardRepository) GetStats(
	ctx context.Context,
) (*models.DashboardStats, error) {
	// Compute every counter in a single round trip:
	//   - open_tickets: tickets waiting for work.
	//   - high_priority: unresolved tickets whose latest triage is high or critical.
	//   - ai_analyzed: tickets with at least one triage.
	query := `
		SELECT
			(
				SELECT COUNT(*)
				FROM tickets
				WHERE status = 'open'
			) AS open_tickets,
			(
				SELECT COUNT(*)
				FROM (
					SELECT DISTINCT ON (ticket_id)
						ticket_id,
						priority
					FROM ticket_triages
					ORDER BY ticket_id, created_at DESC, id DESC
				) AS latest_triages
				JOIN tickets ON tickets.id = latest_triages.ticket_id
				WHERE latest_triages.priority IN ('high', 'critical')
					AND tickets.status <> 'resolved'
			) AS high_priority,
			(
				SELECT COUNT(DISTINCT ticket_id)
				FROM ticket_triages
			) AS ai_analyzed
	`

	stats := &models.DashboardStats{}

	err := r.db.QueryRow(ctx, query).Scan(
		&stats.OpenTickets,
		&stats.HighPriority,
		&stats.AIAnalyzed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard stats: %w", err)
	}

	return stats, nil
}
