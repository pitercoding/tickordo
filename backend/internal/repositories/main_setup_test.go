package repositories

import (
	"context"
	"strings"
	"testing"
	"time"
)

// These tests cover the integration test setup itself, so they run without
// TEST_DATABASE_URL and never reach a PostgreSQL server.

func TestCheckMissingTestDatabaseURLAllowsSkipOutsideCI(t *testing.T) {
	if err := checkMissingTestDatabaseURL(""); err != nil {
		t.Fatalf("expected skip to be allowed without CI, got %v", err)
	}
}

func TestCheckMissingTestDatabaseURLFailsOnCI(t *testing.T) {
	for _, ci := range []string{"true", "1"} {
		t.Run("CI="+ci, func(t *testing.T) {
			err := checkMissingTestDatabaseURL(ci)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !strings.Contains(err.Error(), testDatabaseURLEnv) {
				t.Errorf("expected error to mention %s, got %q", testDatabaseURLEnv, err)
			}
		})
	}
}

func TestConnectTestDatabaseRefusesOtherDatabases(t *testing.T) {
	// Port 1 has no PostgreSQL server, so a regression that skipped the name
	// check would surface as a connection error instead of a refusal.
	tests := []struct {
		name        string
		databaseURL string
	}{
		{name: "development database", databaseURL: "postgres://tickordo:tickordo@127.0.0.1:1/tickordo?sslmode=disable"},
		{name: "similar name", databaseURL: "postgres://postgres:postgres@127.0.0.1:1/tickordo_test_copy?sslmode=disable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			pool, err := connectTestDatabase(ctx, tt.databaseURL)
			if pool != nil {
				pool.Close()
				t.Fatal("expected no pool to be returned")
			}

			if err == nil || !strings.Contains(err.Error(), "refusing to run") {
				t.Fatalf("expected a refusal, got %v", err)
			}
		})
	}
}

func TestConnectTestDatabaseRejectsInvalidURL(t *testing.T) {
	pool, err := connectTestDatabase(context.Background(), "postgres://%zz")
	if pool != nil {
		pool.Close()
		t.Fatal("expected no pool to be returned")
	}

	if err == nil || !strings.Contains(err.Error(), "invalid "+testDatabaseURLEnv) {
		t.Fatalf("expected an invalid URL error, got %v", err)
	}
}
