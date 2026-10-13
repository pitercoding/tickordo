package repositories

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pitercoding/tickordo/internal/models"
)

func TestTicketRepositoryGetByIDReturnsNotFoundForUnknownID(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)

	ticket, err := repo.GetByID(context.Background(), uuid.New())

	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}

	if ticket != nil {
		t.Fatalf("expected nil ticket, got %+v", ticket)
	}
}

func TestTicketRepositoryGetByIDReturnsTicketWithoutTriage(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	created := &models.Ticket{
		ID:          uuid.New(),
		Title:       "Printer not working",
		Description: "The office printer shows a paper jam error.",
		Status:      "open",
	}

	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), "DELETE FROM tickets WHERE id = $1", created.ID); err != nil {
			t.Errorf("failed to delete test ticket: %v", err)
		}
	})

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, got.ID)
	}

	if got.Title != created.Title {
		t.Errorf("expected title %q, got %q", created.Title, got.Title)
	}

	if got.Description != created.Description {
		t.Errorf("expected description %q, got %q", created.Description, got.Description)
	}

	if got.Status != created.Status {
		t.Errorf("expected status %q, got %q", created.Status, got.Status)
	}

	if got.LatestTriageCategory != nil {
		t.Errorf("expected nil LatestTriageCategory, got %q", *got.LatestTriageCategory)
	}

	if got.LatestTriagePriority != nil {
		t.Errorf("expected nil LatestTriagePriority, got %q", *got.LatestTriagePriority)
	}

	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected non-zero timestamps, got created_at=%v updated_at=%v", got.CreatedAt, got.UpdatedAt)
	}

	// Both values come from PostgreSQL (microsecond precision), so they must
	// match exactly what Create returned via RETURNING.
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("expected created_at %v, got %v", created.CreatedAt, got.CreatedAt)
	}

	if !got.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("expected updated_at %v, got %v", created.UpdatedAt, got.UpdatedAt)
	}

	// Both columns default to NOW(), which is fixed for the inserting transaction.
	if !got.CreatedAt.Equal(got.UpdatedAt) {
		t.Errorf("expected created_at and updated_at to be equal, got %v and %v", got.CreatedAt, got.UpdatedAt)
	}
}

// createTestTicket inserts an open ticket through the repository and deletes
// it when the test finishes, even if the test fails.
func createTestTicket(t *testing.T, repo *TicketRepository, title string) *models.Ticket {
	t.Helper()

	ticket := &models.Ticket{
		ID:          uuid.New(),
		Title:       title,
		Description: "Description for " + title,
		Status:      "open",
	}

	if err := repo.Create(context.Background(), ticket); err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	t.Cleanup(func() {
		if _, err := repo.db.Exec(context.Background(), "DELETE FROM tickets WHERE id = $1", ticket.ID); err != nil {
			t.Errorf("failed to delete test ticket: %v", err)
		}
	})

	return ticket
}

// databaseNow returns PostgreSQL's NOW() in its own transaction, so it can be
// used as a bound for timestamps written by later or earlier statements.
func databaseNow(t *testing.T, repo *TicketRepository) time.Time {
	t.Helper()

	var now time.Time
	if err := repo.db.QueryRow(context.Background(), "SELECT NOW()").Scan(&now); err != nil {
		t.Fatalf("failed to read database time: %v", err)
	}

	return now
}

func TestTicketRepositoryUpdateStatusUpdatesExistingTicket(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	created := createTestTicket(t, repo, "VPN disconnects")

	// Move both timestamps to a fixed past instant so the update is observable
	// without sleeping or relying on clock resolution.
	past := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(ctx, "UPDATE tickets SET created_at = $1, updated_at = $1 WHERE id = $2", past, created.ID); err != nil {
		t.Fatalf("failed to backdate ticket: %v", err)
	}

	before := databaseNow(t, repo)

	if err := repo.UpdateStatus(ctx, created.ID, "in_progress"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	after := databaseNow(t, repo)

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to reload ticket: %v", err)
	}

	if got.Status != "in_progress" {
		t.Errorf("expected status %q, got %q", "in_progress", got.Status)
	}

	// updated_at is set to NOW() of the UPDATE's transaction, which starts
	// between the two databaseNow calls.
	if got.UpdatedAt.Before(before) || got.UpdatedAt.After(after) {
		t.Errorf("expected updated_at between %v and %v, got %v", before, after, got.UpdatedAt)
	}

	if !got.CreatedAt.Equal(past) {
		t.Errorf("expected created_at %v to be unchanged, got %v", past, got.CreatedAt)
	}

	if got.Title != created.Title {
		t.Errorf("expected title %q, got %q", created.Title, got.Title)
	}

	if got.Description != created.Description {
		t.Errorf("expected description %q, got %q", created.Description, got.Description)
	}
}

func TestTicketRepositoryUpdateStatusReturnsNotFoundForUnknownID(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), "resolved")

	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestTicketRepositoryUpdateStatusOnlyAffectsTargetTicket(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	target := createTestTicket(t, repo, "Email bounce")
	other := createTestTicket(t, repo, "Monitor flickering")

	if err := repo.UpdateStatus(ctx, target.ID, "resolved"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := repo.GetByID(ctx, other.ID)
	if err != nil {
		t.Fatalf("failed to reload other ticket: %v", err)
	}

	if got.Status != other.Status {
		t.Errorf("expected other ticket status %q, got %q", other.Status, got.Status)
	}

	if !got.UpdatedAt.Equal(other.UpdatedAt) {
		t.Errorf("expected other ticket updated_at %v, got %v", other.UpdatedAt, got.UpdatedAt)
	}
}

func TestTicketRepositoryUpdateStatusSucceedsWithSameStatus(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	created := createTestTicket(t, repo, "Keyboard missing keys")

	// PostgreSQL reports matched rows, not changed rows, so re-applying the
	// current status must not be mistaken for a missing ticket.
	if err := repo.UpdateStatus(ctx, created.ID, created.Status); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to reload ticket: %v", err)
	}

	if got.Status != created.Status {
		t.Errorf("expected status %q, got %q", created.Status, got.Status)
	}
}

func TestTicketRepositoryUpdateStatusReturnsDatabaseError(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	created := createTestTicket(t, repo, "Laptop overheating")

	// status is VARCHAR(50); the repository does not validate it, so the
	// database rejects the value.
	err := repo.UpdateStatus(ctx, created.ID, strings.Repeat("x", 51))

	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected a database error, got ErrTicketNotFound")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to reload ticket: %v", err)
	}

	if got.Status != created.Status {
		t.Errorf("expected status %q to be unchanged, got %q", created.Status, got.Status)
	}

	if !got.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("expected updated_at %v to be unchanged, got %v", created.UpdatedAt, got.UpdatedAt)
	}
}

func TestTicketRepositoryUpdateStatusRejectsUnknownStatus(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	created := createTestTicket(t, repo, "Monitor flickering")

	// "closed" fits VARCHAR(50), so only the tickets_status_check constraint
	// (migration 003) can reject it.
	err := repo.UpdateStatus(ctx, created.ID, "closed")

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a PostgreSQL error, got %v", err)
	}

	// 23514 is check_violation.
	if pgErr.Code != "23514" {
		t.Errorf("expected SQLSTATE 23514, got %s", pgErr.Code)
	}

	if pgErr.ConstraintName != "tickets_status_check" {
		t.Errorf("expected constraint tickets_status_check, got %q", pgErr.ConstraintName)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to reload ticket: %v", err)
	}

	if got.Status != created.Status {
		t.Errorf("expected status %q to be unchanged, got %q", created.Status, got.Status)
	}
}

func TestTicketRepositoryListReturnsEmptySliceWithoutTickets(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	repo := NewTicketRepository(db)

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// A nil slice would be encoded as JSON null instead of [].
	if got == nil {
		t.Fatal("expected an empty slice, got nil")
	}

	if len(got) != 0 {
		t.Fatalf("expected no tickets, got %d", len(got))
	}
}

func TestTicketRepositoryListUsesLatestTriageBreakingCreatedAtTiesByID(t *testing.T) {
	db := requireTestDB(t)
	ticketRepo := NewTicketRepository(db)
	triageRepo := NewTicketTriageRepository(db)

	at := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	lowID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	highID := uuid.MustParse("bfffffff-ffff-ffff-ffff-ffffffffffff")
	// The older triage has the greatest id, so ordering by id alone would
	// wrongly pick it over the tied triages.
	olderID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	triages := map[uuid.UUID]struct {
		category  string
		priority  string
		createdAt time.Time
	}{
		lowID:   {category: "billing", priority: "low", createdAt: at},
		highID:  {category: "technical", priority: "critical", createdAt: at},
		olderID: {category: "account", priority: "medium", createdAt: at.Add(-time.Microsecond)},
	}

	// Each case inserts the same triages in a different order, so insertion
	// order cannot decide the result.
	tests := []struct {
		name string
		ids  []uuid.UUID
	}{
		{name: "greatest tied id inserted first", ids: []uuid.UUID{highID, lowID, olderID}},
		{name: "greatest tied id inserted last", ids: []uuid.UUID{olderID, lowID, highID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ticket := createTestTicket(t, ticketRepo, "Listed with tied triages: "+tt.name)

			for _, id := range tt.ids {
				triage := triages[id]
				createTestTriage(t, triageRepo, id, ticket.ID, triage.priority, triage.createdAt)

				if _, err := db.Exec(ctx, "UPDATE ticket_triages SET category = $1 WHERE id = $2", triage.category, id); err != nil {
					t.Fatalf("failed to set triage category: %v", err)
				}
			}

			tickets, err := ticketRepo.List(ctx)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			var got *models.Ticket
			for i := range tickets {
				if tickets[i].ID == ticket.ID {
					got = &tickets[i]
					break
				}
			}

			if got == nil {
				t.Fatalf("expected ticket %s in the list", ticket.ID)
			}

			want := triages[highID]

			if got.LatestTriageCategory == nil || *got.LatestTriageCategory != want.category {
				t.Errorf("expected latest category %q, got %v", want.category, derefString(got.LatestTriageCategory))
			}

			if got.LatestTriagePriority == nil || *got.LatestTriagePriority != want.priority {
				t.Errorf("expected latest priority %q, got %v", want.priority, derefString(got.LatestTriagePriority))
			}
		})
	}
}

// derefString formats an optional string for failure messages.
func derefString(s *string) string {
	if s == nil {
		return "<nil>"
	}

	return strconv.Quote(*s)
}
