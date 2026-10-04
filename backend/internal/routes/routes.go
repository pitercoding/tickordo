package routes

import (
	"net/http"

	"github.com/pitercoding/tickordo/internal/handlers"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", handlers.Health)
}
