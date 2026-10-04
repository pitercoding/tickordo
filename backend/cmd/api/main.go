package main

import (
	"fmt"
	"net/http"

	"github.com/pitercoding/tickordo/internal/routes"
)

func main() {
	const address = ":8080"

	mux := http.NewServeMux()

	routes.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    address,
		Handler: mux,
	}

	fmt.Printf("Tickordo API running on http://localhost%s\n", address)

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}
