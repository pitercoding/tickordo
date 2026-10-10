package services

import (
	"context"
	"fmt"

	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

type DashboardService struct {
	repository *repositories.DashboardRepository
}

func NewDashboardService(
	repository *repositories.DashboardRepository,
) *DashboardService {
	return &DashboardService{
		repository: repository,
	}
}

func (s *DashboardService) GetStats(
	ctx context.Context,
) (*models.DashboardStats, error) {
	// Load the aggregated dashboard counters.
	stats, err := s.repository.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard stats: %w", err)
	}

	return stats, nil
}
