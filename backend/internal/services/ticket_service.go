package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// ticketRepository lists the repository methods TicketService calls.
// Production passes *repositories.TicketRepository; tests pass a fake so
// they can run without a database.
type ticketRepository interface {
	Create(ctx context.Context, ticket *models.Ticket) error
	List(ctx context.Context) ([]models.Ticket, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// The real repository must keep satisfying ticketRepository.
var _ ticketRepository = (*repositories.TicketRepository)(nil)

type TicketService struct {
	repository ticketRepository
}

func NewTicketService(repository ticketRepository) *TicketService {
	return &TicketService{
		repository: repository,
	}
}

func (s *TicketService) CreateTicket(
	ctx context.Context,
	title string,
	description string,
) (*models.Ticket, error) {
	ticket := &models.Ticket{
		ID:          uuid.New(),
		Title:       title,
		Description: description,
		Status:      "open",
	}

	if err := s.repository.Create(ctx, ticket); err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	return ticket, nil
}

func (s *TicketService) ListTickets(
	ctx context.Context,
) ([]models.Ticket, error) {
	tickets, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return tickets, nil
}

func (s *TicketService) GetTicketByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.Ticket, error) {
	ticket, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	return ticket, nil
}

func (s *TicketService) UpdateTicketStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
) error {
	if err := s.repository.UpdateStatus(ctx, id, status); err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	return nil
}
