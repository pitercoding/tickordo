package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	ticketHandler *handlers.TicketHandler,
	ticketTriageHandler *handlers.TicketTriageHandler,
) {
	// Health check endpoint.
	mux.HandleFunc("GET /health", handlers.Health)

	// Ticket endpoints.
	mux.HandleFunc("POST /tickets", ticketHandler.Create)
	mux.HandleFunc("GET /tickets", ticketHandler.List)
	mux.HandleFunc("GET /tickets/{id}", ticketHandler.GetByID)
	mux.HandleFunc("PATCH /tickets/{id}", ticketHandler.UpdateStatus)

	// AI ticket triage endpoints.
	mux.HandleFunc("POST /tickets/{id}/triage", ticketTriageHandler.Create)
	mux.HandleFunc("GET /tickets/{id}/triages", ticketTriageHandler.List)
	mux.HandleFunc("GET /tickets/{id}/triage", ticketTriageHandler.GetLatest)
}
