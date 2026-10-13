package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pitercoding/tickordo/internal/models"
)

// clearDashboardTables removes every ticket and triage before and after the
// test. GetStats aggregates the whole tables, so exact assertions require a
// known starting point. requireTestDB guarantees this only runs against
// tickordo_test.
func clearDashboardTables(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	clearTables := func() error {
		// ticket_triages references tickets without ON DELETE CASCADE.
		if _, err := db.Exec(context.Background(), "DELETE FROM ticket_triages"); err != nil {
			return err
		}

		_, err := db.Exec(context.Background(), "DELETE FROM tickets")
		return err
	}

	if err := clearTables(); err != nil {
		t.Fatalf("failed to clear dashboard tables: %v", err)
	}

	t.Cleanup(func() {
		if err := clearTables(); err != nil {
			t.Errorf("failed to clear dashboard tables: %v", err)
		}
	})
}

// createTestTicketWithStatus creates a ticket through the repository and moves
// it to the given status when it differs from the default "open".
func createTestTicketWithStatus(t *testing.T, repo *TicketRepository, title, status string) *models.Ticket {
	t.Helper()

	ticket := createTestTicket(t, repo, title)

	if status != ticket.Status {
		if err := repo.UpdateStatus(context.Background(), ticket.ID, status); err != nil {
			t.Fatalf("failed to set ticket status to %q: %v", status, err)
		}

		ticket.Status = status
	}

	return ticket
}

// createTestTriage inserts a triage through the repository with a fixed
// created_at, so "latest triage" is deterministic, and deletes it when the
// test finishes. Cleanups run in reverse order, so the triage is removed
// before the ticket it references.
func createTestTriage(
	t *testing.T,
	repo *TicketTriageRepository,
	id uuid.UUID,
	ticketID uuid.UUID,
	priority string,
	createdAt time.Time,
) *models.TicketTriage {
	t.Helper()

	ctx := context.Background()

	triage := &models.TicketTriage{
		ID:              id,
		TicketID:        ticketID,
		Category:        "technical",
		Priority:        priority,
		Sentiment:       "neutral",
		SuggestedTeam:   "support",
		Summary:         "Test summary",
		SuggestedAction: "Test action",
		Confidence:      0.9,
		Model:           "test-model",
	}

	if err := repo.Create(ctx, triage); err != nil {
		t.Fatalf("failed to create triage: %v", err)
	}

	t.Cleanup(func() {
		if _, err := repo.db.Exec(context.Background(), "DELETE FROM ticket_triages WHERE id = $1", triage.ID); err != nil {
			t.Errorf("failed to delete test triage: %v", err)
		}
	})

	if _, err := repo.db.Exec(ctx, "UPDATE ticket_triages SET created_at = $1 WHERE id = $2", createdAt, triage.ID); err != nil {
		t.Fatalf("failed to set triage created_at: %v", err)
	}

	triage.CreatedAt = createdAt

	return triage
}

func assertDashboardStats(t *testing.T, got *models.DashboardStats, want models.DashboardStats) {
	t.Helper()

	if got == nil {
		t.Fatal("expected stats, got nil")
	}

	if got.OpenTickets != want.OpenTickets {
		t.Errorf("expected open_tickets %d, got %d", want.OpenTickets, got.OpenTickets)
	}

	if got.HighPriority != want.HighPriority {
		t.Errorf("expected high_priority %d, got %d", want.HighPriority, got.HighPriority)
	}

	if got.AIAnalyzed != want.AIAnalyzed {
		t.Errorf("expected ai_analyzed %d, got %d", want.AIAnalyzed, got.AIAnalyzed)
	}
}

func TestDashboardRepositoryGetStatsReturnsZerosWithoutTickets(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	repo := NewDashboardRepository(db)

	got, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDashboardStats(t, got, models.DashboardStats{})
}

func TestDashboardRepositoryGetStatsCountsOnlyOpenTickets(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	ticketRepo := NewTicketRepository(db)
	repo := NewDashboardRepository(db)

	createTestTicketWithStatus(t, ticketRepo, "Open one", "open")
	createTestTicketWithStatus(t, ticketRepo, "Open two", "open")
	createTestTicketWithStatus(t, ticketRepo, "In progress", "in_progress")
	createTestTicketWithStatus(t, ticketRepo, "Resolved", "resolved")

	got, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Without triages, nothing is high priority or AI analyzed.
	assertDashboardStats(t, got, models.DashboardStats{OpenTickets: 2})
}

func TestDashboardRepositoryGetStatsCountsPriorityAndAnalyzedTickets(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	ticketRepo := NewTicketRepository(db)
	triageRepo := NewTicketTriageRepository(db)
	repo := NewDashboardRepository(db)

	at := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)

	openCritical := createTestTicketWithStatus(t, ticketRepo, "Open critical", "open")
	inProgressHigh := createTestTicketWithStatus(t, ticketRepo, "In progress high", "in_progress")
	resolvedHigh := createTestTicketWithStatus(t, ticketRepo, "Resolved high", "resolved")
	resolvedCritical := createTestTicketWithStatus(t, ticketRepo, "Resolved critical", "resolved")
	openMedium := createTestTicketWithStatus(t, ticketRepo, "Open medium", "open")
	openLow := createTestTicketWithStatus(t, ticketRepo, "Open low", "open")
	createTestTicketWithStatus(t, ticketRepo, "Open without triage", "open")
	createTestTicketWithStatus(t, ticketRepo, "In progress without triage", "in_progress")

	createTestTriage(t, triageRepo, uuid.New(), openCritical.ID, "critical", at)
	createTestTriage(t, triageRepo, uuid.New(), inProgressHigh.ID, "high", at)
	createTestTriage(t, triageRepo, uuid.New(), resolvedHigh.ID, "high", at)
	createTestTriage(t, triageRepo, uuid.New(), resolvedCritical.ID, "critical", at)
	createTestTriage(t, triageRepo, uuid.New(), openMedium.ID, "medium", at)
	createTestTriage(t, triageRepo, uuid.New(), openLow.ID, "low", at)

	got, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDashboardStats(t, got, models.DashboardStats{
		// Open critical, open medium, open low and open without triage.
		OpenTickets: 4,
		// Only unresolved tickets with high or critical latest triage.
		HighPriority: 2,
		// Every ticket with a triage, including resolved ones.
		AIAnalyzed: 6,
	})
}

func TestDashboardRepositoryGetStatsUsesLatestTriageWithoutDuplicates(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	ticketRepo := NewTicketRepository(db)
	triageRepo := NewTicketTriageRepository(db)
	repo := NewDashboardRepository(db)

	older := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	// Two high-priority triages must still count the ticket once.
	escalated := createTestTicketWithStatus(t, ticketRepo, "Escalated", "open")
	createTestTriage(t, triageRepo, uuid.New(), escalated.ID, "high", older)
	createTestTriage(t, triageRepo, uuid.New(), escalated.ID, "critical", newer)

	// An older critical triage is ignored once a newer one lowers the priority.
	downgraded := createTestTicketWithStatus(t, ticketRepo, "Downgraded", "open")
	createTestTriage(t, triageRepo, uuid.New(), downgraded.ID, "critical", older)
	createTestTriage(t, triageRepo, uuid.New(), downgraded.ID, "low", newer)

	// A newer high triage counts even though an older one was low. The newer
	// triage is inserted first, so insertion order cannot decide the result.
	upgraded := createTestTicketWithStatus(t, ticketRepo, "Upgraded", "in_progress")
	createTestTriage(t, triageRepo, uuid.New(), upgraded.ID, "high", newer)
	createTestTriage(t, triageRepo, uuid.New(), upgraded.ID, "low", older)

	// Several non-urgent triages count once for ai_analyzed.
	reanalyzed := createTestTicketWithStatus(t, ticketRepo, "Reanalyzed", "open")
	createTestTriage(t, triageRepo, uuid.New(), reanalyzed.ID, "medium", older)
	createTestTriage(t, triageRepo, uuid.New(), reanalyzed.ID, "medium", newer)
	createTestTriage(t, triageRepo, uuid.New(), reanalyzed.ID, "medium", newer.Add(time.Hour))

	got, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDashboardStats(t, got, models.DashboardStats{
		// Triages never change the open count: escalated, downgraded, reanalyzed.
		OpenTickets: 3,
		// Escalated and upgraded.
		HighPriority: 2,
		AIAnalyzed:   4,
	})
}

func TestDashboardRepositoryGetStatsBreaksCreatedAtTiesByID(t *testing.T) {
	db := requireTestDB(t)
	clearDashboardTables(t, db)
	ticketRepo := NewTicketRepository(db)
	triageRepo := NewTicketTriageRepository(db)
	repo := NewDashboardRepository(db)

	at := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	lowID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	highID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	// With equal created_at, the triage with the greatest id is the latest.
	countedTicket := createTestTicketWithStatus(t, ticketRepo, "Tie resolves to critical", "open")
	createTestTriage(t, triageRepo, highID, countedTicket.ID, "critical", at)
	createTestTriage(t, triageRepo, lowID, countedTicket.ID, "low", at)

	ignoredTicket := createTestTicketWithStatus(t, ticketRepo, "Tie resolves to low", "open")
	createTestTriage(t, triageRepo, uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffffe"), ignoredTicket.ID, "low", at)
	createTestTriage(t, triageRepo, uuid.MustParse("00000000-0000-0000-0000-000000000002"), ignoredTicket.ID, "high", at)

	got, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertDashboardStats(t, got, models.DashboardStats{
		OpenTickets:  2,
		HighPriority: 1,
		AIAnalyzed:   2,
	})
}
