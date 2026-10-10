package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pitercoding/tickordo/internal/ai"
	"github.com/pitercoding/tickordo/internal/config"
	"github.com/pitercoding/tickordo/internal/database"
	"github.com/pitercoding/tickordo/internal/handlers"
	"github.com/pitercoding/tickordo/internal/middleware"
	"github.com/pitercoding/tickordo/internal/repositories"
	"github.com/pitercoding/tickordo/internal/routes"
	"github.com/pitercoding/tickordo/internal/services"
)

const (
	// connectTimeout limits how long startup waits for PostgreSQL.
	connectTimeout = 10 * time.Second

	// shutdownTimeout is longer than the OpenAI analyze timeout so an
	// in-flight triage can complete before the process exits.
	shutdownTimeout = 70 * time.Second
)

func main() {
	// Load environment variables from the .env file.
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("warning: .env file not loaded: %v", err)
	}

	// Load the application configuration and fail fast when it is incomplete.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// Bound the startup connection so an unreachable database cannot hang the app.
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), connectTimeout)
	defer cancelConnect()

	// Connect to the PostgreSQL database.
	db, err := database.Connect(connectCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connection established")

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

	// Initialize the dashboard repository.
	dashboardRepository := repositories.NewDashboardRepository(db)

	// Initialize the dashboard service.
	dashboardService := services.NewDashboardService(dashboardRepository)

	// Initialize the dashboard HTTP handler.
	dashboardHandler := handlers.NewDashboardHandler(dashboardService)

	// Create the HTTP router.
	mux := http.NewServeMux()

	// Register all application routes.
	routes.RegisterRoutes(mux, ticketHandler, ticketTriageHandler, dashboardHandler)

	// Configure the HTTP server with timeouts so slow clients cannot hold
	// connections open forever. WriteTimeout must stay above the OpenAI
	// analyze timeout, otherwise triage responses would be cut off.
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           middleware.CORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Stop the server gracefully on Ctrl+C or SIGTERM (e.g. docker stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the HTTP server in the background so main can wait for a signal.
	serverErr := make(chan error, 1)

	go func() {
		log.Printf("Tickordo API running on http://localhost:%s", cfg.Port)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		// ListenAndServe only returns here on failure (e.g. port already in use).
		log.Fatalf("server failed: %v", err)
	case <-ctx.Done():
		log.Println("shutting down server...")
	}

	// Give in-flight requests (including OpenAI triage calls) time to finish.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
		return
	}

	log.Println("server stopped")
}
