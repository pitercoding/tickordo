package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

type TicketService struct {
	repository *repositories.TicketRepository
}

func NewTicketService(repository *repositories.TicketRepository) *TicketService {
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
