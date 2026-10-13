package repositories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testDatabaseURLEnv = "TEST_DATABASE_URL"
	testDatabaseName   = "tickordo_test"
	ciEnv              = "CI"
)

// testDB is the shared pool used by integration tests. It is nil when
// TEST_DATABASE_URL is not set, in which case integration tests are skipped.
var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	databaseURL := os.Getenv(testDatabaseURLEnv)
	if databaseURL == "" {
		if err := checkMissingTestDatabaseURL(os.Getenv(ciEnv)); err != nil {
			fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
			return 1
		}

		fmt.Printf("%s is not set: skipping repository integration tests\n", testDatabaseURLEnv)
		return m.Run()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := connectTestDatabase(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
		return 1
	}
	defer pool.Close()

	if err := resetSchema(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
		return 1
	}

	testDB = pool

	return m.Run()
}

// checkMissingTestDatabaseURL decides what happens when TEST_DATABASE_URL is
// not set. Locally the integration tests are skipped, but on CI (any non-empty
// CI value) a missing database must fail the run instead of passing silently.
func checkMissingTestDatabaseURL(ci string) error {
	if ci == "" {
		return nil
	}

	return fmt.Errorf(
		"refusing to skip repository integration tests: %s is not set but %s=%q",
		testDatabaseURLEnv,
		ciEnv,
		ci,
	)
}

// connectTestDatabase parses the URL, refuses any database other than
// tickordo_test and confirms the server-side database name before returning.
func connectTestDatabase(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", testDatabaseURLEnv, err)
	}

	if config.ConnConfig.Database != testDatabaseName {
		return nil, fmt.Errorf(
			"refusing to run: %s must point to database %q, got %q",
			testDatabaseURLEnv,
			testDatabaseName,
			config.ConnConfig.Database,
		)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	var currentDatabase string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDatabase); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	if currentDatabase != testDatabaseName {
		pool.Close()
		return nil, fmt.Errorf(
			"refusing to run: connected to database %q, expected %q",
			currentDatabase,
			testDatabaseName,
		)
	}

	return pool, nil
}

// resetSchema runs every down migration in reverse order and then every up
// migration in ascending order, inside a single transaction.
func resetSchema(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir := filepath.Join("..", "..", "..", "migrations")

	upFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to list up migrations: %w", err)
	}

	downFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
	if err != nil {
		return fmt.Errorf("failed to list down migrations: %w", err)
	}

	if len(upFiles) == 0 {
		return fmt.Errorf("no up migrations found in %s", migrationsDir)
	}

	slices.Sort(upFiles)
	slices.Sort(downFiles)
	slices.Reverse(downFiles)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, file := range append(downFiles, upFiles...) {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filepath.Base(file), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}

	return nil
}

// requireTestDB skips the calling test when no test database is configured.
func requireTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testDB == nil {
		t.Skipf("%s is not set: skipping integration test", testDatabaseURLEnv)
	}

	return testDB
}
