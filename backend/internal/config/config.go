package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port              string
	DatabaseURL       string
	OpenAIAPIKey      string
	OpenAIModel       string
	CORSAllowedOrigin string
}

// Load reads the configuration from environment variables and returns an
// error when a required variable is missing.
func Load() (Config, error) {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	// Default to the local frontend so development works without extra setup.
	corsAllowedOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")

	if corsAllowedOrigin == "" {
		corsAllowedOrigin = "http://localhost:3000"
	}

	// Default to the model the analyzer has used so far, so existing
	// deployments keep their behavior when OPENAI_MODEL is not set.
	openAIModel := os.Getenv("OPENAI_MODEL")

	if openAIModel == "" {
		openAIModel = "gpt-5.6"
	}

	cfg := Config{
		Port:              port,
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:       openAIModel,
		CORSAllowedOrigin: corsAllowedOrigin,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("missing required environment variable: DATABASE_URL")
	}

	// Triage depends on OpenAI, so the API key is required at startup.
	if cfg.OpenAIAPIKey == "" {
		return Config{}, fmt.Errorf("missing required environment variable: OPENAI_API_KEY")
	}

	return cfg, nil
}
