package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/pitercoding/tickordo/internal/ai"
	"github.com/pitercoding/tickordo/internal/config"
	"github.com/pitercoding/tickordo/internal/database"
	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/repositories"
	"github.com/pitercoding/tickordo/internal/routes"
	"github.com/pitercoding/tickordo/internal/services"
)

func main() {
	// Load environment variables from the .env file.
	if err := godotenv.Load("../.env"); err != nil {
		fmt.Printf("warning: .env file not loaded: %v\n", err)
	}

	// Load the application configuration.
	cfg := config.Load()

	// Fail fast when the OpenAI API key is missing, since triage depends on it.
	if cfg.OpenAIAPIKey == "" {
		fmt.Println("missing required environment variable: OPENAI_API_KEY")
		return
	}

	// Create a context for the application startup process.
	ctx := context.Background()

	// Connect to the PostgreSQL database.
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Println("Database connection established")

	// Initialize the ticket repository.
	ticketRepository := repositories.NewTicketRepository(db)

	// Initialize the ticket service.
	ticketService := services.NewTicketService(ticketRepository)

	// Initialize the ticket HTTP handler.
	ticketHandler := handlers.NewTicketHandler(ticketService)

	// Initialize the OpenAI client using the API key from the environment.
	openAIClient := ai.NewOpenAIClient(cfg.OpenAIAPIKey)

	// Initialize the AI analyzer responsible for analyzing support tickets.
	ticketAnalyzer := ai.NewOpenAIAnalyzer(openAIClient, "gpt-5.6")

	// Initialize the ticket triage repository.
	ticketTriageRepository := repositories.NewTicketTriageRepository(db)

	// Initialize the ticket triage service.
	ticketTriageService := services.NewTicketTriageService(
		ticketRepository,
		ticketTriageRepository,
		ticketAnalyzer,
	)

	// Initialize the ticket triage HTTP handler.
	ticketTriageHandler := handlers.NewTicketTriageHandler(ticketTriageService)

	// Create the HTTP router.
	mux := http.NewServeMux()

	// Register all application routes.
	routes.RegisterRoutes(mux, ticketHandler, ticketTriageHandler)

	// Configure the HTTP server.
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// Start the HTTP server.
	fmt.Printf("Tickordo API running on http://localhost:%s\n", cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}
