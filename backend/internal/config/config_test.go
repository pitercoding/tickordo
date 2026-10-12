package config

import (
	"os"
	"testing"
)

// setRequiredEnv sets the variables Load requires, so each test only has to
// set the variable it is about. t.Setenv restores them after the test.
func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("OPENAI_API_KEY", "test-key")
}

func TestLoad_CORSAllowedOrigin(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{name: "unset uses default", unset: true, want: "http://localhost:3000"},
		{name: "empty uses default", value: "", want: "http://localhost:3000"},
		{name: "custom origin", value: "https://app.tickordo.example", want: "https://app.tickordo.example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			// t.Setenv first so the original value is restored after the test,
			// even when the variable is then removed.
			t.Setenv("CORS_ALLOWED_ORIGIN", tt.value)
			if tt.unset {
				os.Unsetenv("CORS_ALLOWED_ORIGIN")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.CORSAllowedOrigin != tt.want {
				t.Errorf("CORSAllowedOrigin = %q, want %q", cfg.CORSAllowedOrigin, tt.want)
			}
		})
	}
}
