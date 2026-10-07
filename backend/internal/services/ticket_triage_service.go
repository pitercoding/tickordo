package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/ai"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

type TicketTriageService struct {
	ticketRepository *repositories.TicketRepository
	triageRepository *repositories.TicketTriageRepository
	analyzer         ai.TicketAnalyzer
}

func NewTicketTriageService(
	ticketRepository *repositories.TicketRepository,
	triageRepository *repositories.TicketTriageRepository,
	analyzer ai.TicketAnalyzer,
) *TicketTriageService {
	return &TicketTriageService{
		ticketRepository: ticketRepository,
		triageRepository: triageRepository,
		analyzer:         analyzer,
	}
}

func (s *TicketTriageService) AnalyzeTicket(
	ctx context.Context,
	ticketID uuid.UUID,
) (*models.TicketTriage, error) {
	// Load the ticket that will be analyzed.
	ticket, err := s.ticketRepository.GetByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	// Send the ticket to the AI analyzer.
	triage, err := s.analyzer.Analyze(ctx, ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze ticket: %w", err)
	}

	// Generate the ID for the new triage result.
	triage.ID = uuid.New()

	// Save the AI analysis in PostgreSQL.
	if err := s.triageRepository.Create(ctx, triage); err != nil {
		return nil, fmt.Errorf("failed to save ticket triage: %w", err)
	}

	return triage, nil
}

func (s *TicketTriageService) ListTriagesByTicketID(
	ctx context.Context,
	ticketID uuid.UUID,
) ([]*models.TicketTriage, error) {
	// Verify that the ticket exists.
	if _, err := s.ticketRepository.GetByID(ctx, ticketID); err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	// Load the ticket's triage history.
	triages, err := s.triageRepository.ListByTicketID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ticket triages: %w", err)
	}

	return triages, nil
}

func (s *TicketTriageService) GetLatestTriageByTicketID(
	ctx context.Context,
	ticketID uuid.UUID,
) (*models.TicketTriage, error) {
	// Verify that the ticket exists.
	if _, err := s.ticketRepository.GetByID(ctx, ticketID); err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	// Load the most recent triage for the ticket.
	triage, err := s.triageRepository.GetLatestByTicketID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest ticket triage: %w", err)
	}

	return triage, nil
}
