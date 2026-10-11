package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pitercoding/tickordo/internal/models"
)

// ErrTicketNotFound is returned when no ticket matches the given ID.
var ErrTicketNotFound = errors.New("ticket not found")

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

	if err != nil {
		return fmt.Errorf("failed to create ticket: %w", err)
	}

	return nil
}

func (r *TicketRepository) List(ctx context.Context) ([]models.Ticket, error) {
	query := `
		SELECT
			t.id,
			t.title,
			t.description,
			t.status,
			latest_triage.category,
			latest_triage.priority,
			t.created_at,
			t.updated_at
		FROM tickets AS t
		LEFT JOIN LATERAL (
			SELECT
				category,
				priority
			FROM ticket_triages
			WHERE ticket_id = t.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) AS latest_triage ON TRUE
		ORDER BY t.created_at DESC
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
			&ticket.LatestTriageCategory,
			&ticket.LatestTriagePriority,
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

func (r *TicketRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.Ticket, error) {
	query := `
		SELECT
			t.id,
			t.title,
			t.description,
			t.status,
			latest_triage.category,
			latest_triage.priority,
			t.created_at,
			t.updated_at
		FROM tickets AS t
		LEFT JOIN LATERAL (
			SELECT
				category,
				priority
			FROM ticket_triages
			WHERE ticket_id = t.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) AS latest_triage ON TRUE
		WHERE t.id = $1
	`

	var ticket models.Ticket

	err := r.db.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&ticket.ID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.LatestTriageCategory,
		&ticket.LatestTriagePriority,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTicketNotFound
		}

		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	return &ticket, nil
}

func (r *TicketRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
) error {
	query := `
		UPDATE tickets
		SET
			status = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.db.Exec(
		ctx,
		query,
		status,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTicketNotFound
	}

	return nil
}
