package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pitercoding/tickordo/internal/models"
)

type TicketRepository struct {
	db *pgxpool.Pool
}

func NewTicketRepository(db *pgxpool.Pool) *TicketRepository {
	return &TicketRepository{
		db: db,
	}
}

func (r *TicketRepository) Create(ctx context.Context, ticket *models.Ticket) error {
	query := `
		INSERT INTO tickets (
			id,
			title,
			description,
			status
		)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		ticket.ID,
		ticket.Title,
		ticket.Description,
		ticket.Status,
	).Scan(
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	return err
}

func (r *TicketRepository) List(ctx context.Context) ([]models.Ticket, error) {
	query := `
		SELECT
			id,
			title,
			description,
			status,
			created_at,
			updated_at
		FROM tickets
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]models.Ticket, 0)

	for rows.Next() {
		var ticket models.Ticket

		if err := rows.Scan(
			&ticket.ID,
			&ticket.Title,
			&ticket.Description,
			&ticket.Status,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tickets: %w", err)
	}

	return tickets, nil
}
