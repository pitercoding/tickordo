package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

// RateLimits holds the middlewares that limit individual routes. Each one
// must wrap its own limiter, so the routes never share a quota. Both fields
// are required.
type RateLimits struct {
	// CreateTicket limits POST /tickets.
	CreateTicket func(http.Handler) http.Handler

	// TriageTicket limits POST /tickets/{id}/triage.
	TriageTicket func(http.Handler) http.Handler
}

func RegisterRoutes(
	mux *http.ServeMux,
	ticketHandler *handlers.TicketHandler,
	ticketTriageHandler *handlers.TicketTriageHandler,
	dashboardHandler *handlers.DashboardHandler,
	rateLimits RateLimits,
) {
	// Health check endpoint.
	mux.HandleFunc("GET /health", handlers.Health)

	// Ticket endpoints.
	// Only ticket creation is rate limited here; reads are not.
	mux.Handle("POST /tickets", rateLimits.CreateTicket(http.HandlerFunc(ticketHandler.Create)))
	mux.HandleFunc("GET /tickets", ticketHandler.List)
	mux.HandleFunc("GET /tickets/{id}", ticketHandler.GetByID)
	mux.HandleFunc("PATCH /tickets/{id}", ticketHandler.UpdateStatus)

	// AI ticket triage endpoints.
	// Only starting a triage is rate limited, before the handler validates
	// anything, so every attempt counts, including invalid IDs and unknown
	// tickets.
	mux.Handle("POST /tickets/{id}/triage", rateLimits.TriageTicket(http.HandlerFunc(ticketTriageHandler.Create)))
	mux.HandleFunc("GET /tickets/{id}/triages", ticketTriageHandler.List)
	mux.HandleFunc("GET /tickets/{id}/triage", ticketTriageHandler.GetLatest)

	// Dashboard endpoint.
	mux.HandleFunc("GET /dashboard/stats", dashboardHandler.GetStats)
}
