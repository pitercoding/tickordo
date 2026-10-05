package services

import (
	"context"

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
		return nil, err
	}

	return ticket, nil
}
