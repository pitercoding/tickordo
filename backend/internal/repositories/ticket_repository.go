package repositories

import (
	"context"

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
