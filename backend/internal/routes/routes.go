package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	ticketHandler *handlers.TicketHandler,
	ticketTriageHandler *handlers.TicketTriageHandler,
	dashboardHandler *handlers.DashboardHandler,
	createTicketRateLimit func(http.Handler) http.Handler,
) {
	// Health check endpoint.
	mux.HandleFunc("GET /health", handlers.Health)

	// Ticket endpoints.
	// Only ticket creation is rate limited here; reads are not.
	mux.Handle("POST /tickets", createTicketRateLimit(http.HandlerFunc(ticketHandler.Create)))
	mux.HandleFunc("GET /tickets", ticketHandler.List)
	mux.HandleFunc("GET /tickets/{id}", ticketHandler.GetByID)
	mux.HandleFunc("PATCH /tickets/{id}", ticketHandler.UpdateStatus)

	// AI ticket triage endpoints.
	mux.HandleFunc("POST /tickets/{id}/triage", ticketTriageHandler.Create)
	mux.HandleFunc("GET /tickets/{id}/triages", ticketTriageHandler.List)
	mux.HandleFunc("GET /tickets/{id}/triage", ticketTriageHandler.GetLatest)

	// Dashboard endpoint.
	mux.HandleFunc("GET /dashboard/stats", dashboardHandler.GetStats)
}
