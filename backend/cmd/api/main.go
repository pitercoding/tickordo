package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/pitercoding/tickordo/internal/config"
	"github.com/pitercoding/tickordo/internal/database"
	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/repositories"
	"github.com/pitercoding/tickordo/internal/routes"
	"github.com/pitercoding/tickordo/internal/services"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		fmt.Printf("warning: .env file not loaded: %v\n", err)
	}

	cfg := config.Load()

	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Println("Database connection established")

	ticketRepository := repositories.NewTicketRepository(db)
	ticketService := services.NewTicketService(ticketRepository)
	ticketHandler := handlers.NewTicketHandler(ticketService)

	mux := http.NewServeMux()

	routes.RegisterRoutes(mux, ticketHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	fmt.Printf("Tickordo API running on http://localhost:%s\n", cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}
