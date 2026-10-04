package main

import (
	"fmt"
	"net/http"

	"github.com/pitercoding/tickordo/internal/config"
	"github.com/pitercoding/tickordo/internal/routes"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()

	routes.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	fmt.Printf("Tickordo API running on http://localhost:%s\n", cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}
