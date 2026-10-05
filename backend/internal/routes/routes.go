package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	ticketHandler *handlers.TicketHandler,
) {
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/tickets", ticketHandler.Create)
}
