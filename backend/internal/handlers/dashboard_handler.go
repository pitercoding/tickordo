package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/pitercoding/tickordo/internal/services"
)

type DashboardHandler struct {
	service *services.DashboardService
}

func NewDashboardHandler(
	service *services.DashboardService,
) *DashboardHandler {
	return &DashboardHandler{
		service: service,
	}
}

func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Load the dashboard counters.
	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		log.Printf("failed to get dashboard stats: %v", err)
		writeError(w, "failed to get dashboard statistics", http.StatusInternalServerError)
		return
	}

	// Encode before writing headers so encoding errors can still return 500.
	response, err := json.Marshal(stats)
	if err != nil {
		log.Printf("failed to marshal dashboard stats response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Return the dashboard counters as JSON.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write dashboard stats response: %v", err)
	}
}
