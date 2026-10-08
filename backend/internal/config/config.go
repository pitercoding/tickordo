package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port         string
	DatabaseURL  string
	OpenAIAPIKey string
}

// Load reads the configuration from environment variables and returns an
// error when a required variable is missing.
func Load() (Config, error) {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	cfg := Config{
		Port:         port,
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
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
