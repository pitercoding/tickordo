package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pitercoding/tickordo/internal/models"
)

// fakeDashboardService implements dashboardService without a database.
// GetStats records the call and returns stats and err, so a test decides
// exactly what the handler receives from the service layer.
type fakeDashboardService struct {
	stats *models.DashboardStats
	err   error

	calls int
}

func (f *fakeDashboardService) GetStats(ctx context.Context) (*models.DashboardStats, error) {
	f.calls++
	return f.stats, f.err
}

func TestDashboardHandlerGetStats_ReturnsStats(t *testing.T) {
	// Each counter has a different value, so a field mapped to the wrong
	// JSON key would fail the test.
	service := &fakeDashboardService{
		stats: &models.DashboardStats{
			OpenTickets:  7,
			HighPriority: 3,
			AIAnalyzed:   12,
		},
	}
	handler := NewDashboardHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)

	rec := serve(t, handler.GetStats, req)

	// encoding/json decodes every JSON number into float64.
	assertJSONResponse(t, rec, http.StatusOK, map[string]any{
		"open_tickets":  float64(7),
		"high_priority": float64(3),
		"ai_analyzed":   float64(12),
	})

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}
}

func TestDashboardHandlerGetStats_ReturnsZeroCounters(t *testing.T) {
	// An empty database gives all-zero counters. The fields have no
	// omitempty, so the client must still receive every key with 0.
	service := &fakeDashboardService{stats: &models.DashboardStats{}}
	handler := NewDashboardHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)

	rec := serve(t, handler.GetStats, req)

	assertJSONResponse(t, rec, http.StatusOK, map[string]any{
		"open_tickets":  float64(0),
		"high_priority": float64(0),
		"ai_analyzed":   float64(0),
	})

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}
}

func TestDashboardHandlerGetStats_MapsServiceError(t *testing.T) {
	// The real service wraps repository errors with %w, so the fake does too.
	service := &fakeDashboardService{
		err: fmt.Errorf("failed to get dashboard stats: %w", errors.New("connection refused")),
	}
	handler := NewDashboardHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)

	// serve fails the test if the handler panics on the error.
	rec := serve(t, handler.GetStats, req)

	// The exact message also shows the internal error text is not leaked
	// to the client.
	assertErrorResponse(t, rec, http.StatusInternalServerError, "failed to get dashboard statistics")

	if service.calls != 1 {
		t.Errorf("service calls = %d, want 1", service.calls)
	}
}
