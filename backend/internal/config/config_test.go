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

// unsetEnv removes key for the duration of the test. t.Setenv runs first so
// the original value is restored afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()

	t.Setenv(key, "")
	os.Unsetenv(key)
}

func TestLoad_AllVariablesSet(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://app.tickordo.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Port:              "9090",
		DatabaseURL:       "postgres://test:test@localhost:5432/test",
		OpenAIAPIKey:      "test-key",
		CORSAllowedOrigin: "https://app.tickordo.example",
	}

	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Port(t *testing.T) {
	tests := []struct {
		name  string
		unset bool
		value string
		want  string
	}{
		{name: "unset uses default", unset: true, want: "8080"},
		{name: "empty uses default", value: "", want: "8080"},
		{name: "custom port", value: "9090", want: "9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			t.Setenv("PORT", tt.value)
			if tt.unset {
				os.Unsetenv("PORT")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Port != tt.want {
				t.Errorf("Port = %q, want %q", cfg.Port, tt.want)
			}
		})
	}
}

func TestLoad_MissingRequiredVariable(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		unset   bool
		wantErr string
	}{
		{name: "DATABASE_URL unset", key: "DATABASE_URL", unset: true, wantErr: "missing required environment variable: DATABASE_URL"},
		{name: "DATABASE_URL empty", key: "DATABASE_URL", wantErr: "missing required environment variable: DATABASE_URL"},
		{name: "OPENAI_API_KEY unset", key: "OPENAI_API_KEY", unset: true, wantErr: "missing required environment variable: OPENAI_API_KEY"},
		{name: "OPENAI_API_KEY empty", key: "OPENAI_API_KEY", wantErr: "missing required environment variable: OPENAI_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)

			if tt.unset {
				unsetEnv(t, tt.key)
			} else {
				t.Setenv(tt.key, "")
			}

			cfg, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want %q", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Errorf("Load() error = %q, want %q", err.Error(), tt.wantErr)
			}

			// A failed load must not return a partially filled config.
			if cfg != (Config{}) {
				t.Errorf("Load() config = %+v, want zero value", cfg)
			}
		})
	}
}

func TestLoad_ChecksDatabaseURLFirst(t *testing.T) {
	// With both required variables missing, the error names DATABASE_URL.
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "OPENAI_API_KEY")

	_, err := Load()

	want := "missing required environment variable: DATABASE_URL"
	if err == nil || err.Error() != want {
		t.Errorf("Load() error = %v, want %q", err, want)
	}
}
