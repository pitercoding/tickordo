package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	ticketHandler *handlers.TicketHandler,
) {
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("POST /tickets", ticketHandler.Create)
	mux.HandleFunc("GET /tickets", ticketHandler.List)
}
