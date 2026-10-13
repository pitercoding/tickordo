package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pitercoding/tickordo/internal/models"
)

func TestTicketTriageRepositoryGetLatestByTicketIDPrefersNewestCreatedAt(t *testing.T) {
	db := requireTestDB(t)
	ticketRepo := NewTicketRepository(db)
	repo := NewTicketTriageRepository(db)

	older := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Microsecond)

	// The newest triage has the smallest id, so the id tiebreaker must not
	// override created_at when the timestamps differ.
	newestID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	ticket := createTestTicket(t, ticketRepo, "Newest created_at wins")
	createTestTriage(t, repo, uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), ticket.ID, "low", older)
	createTestTriage(t, repo, newestID, ticket.ID, "high", newer)

	got, err := repo.GetLatestByTicketID(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != newestID {
		t.Errorf("expected latest triage %s, got %s", newestID, got.ID)
	}

	if !got.CreatedAt.Equal(newer) {
		t.Errorf("expected created_at %v, got %v", newer, got.CreatedAt)
	}
}

func TestTicketTriageRepositoryGetLatestByTicketIDBreaksCreatedAtTiesByID(t *testing.T) {
	db := requireTestDB(t)
	ticketRepo := NewTicketRepository(db)
	repo := NewTicketTriageRepository(db)

	at := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	lowID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	middleID := uuid.MustParse("7fffffff-ffff-ffff-ffff-ffffffffffff")
	highID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	// Each case inserts the same tied triages in a different order, so
	// insertion order cannot decide the result.
	tests := []struct {
		name string
		ids  []uuid.UUID
	}{
		{name: "greatest id inserted first", ids: []uuid.UUID{highID, middleID, lowID}},
		{name: "greatest id inserted last", ids: []uuid.UUID{lowID, middleID, highID}},
		{name: "greatest id inserted in the middle", ids: []uuid.UUID{middleID, highID, lowID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ticket := createTestTicket(t, ticketRepo, "Tied triages: "+tt.name)

			// Only the expected winner is critical, so the ticket's own
			// latest-triage lookup can be checked against it.
			for _, id := range tt.ids {
				priority := "low"
				if id == highID {
					priority = "critical"
				}

				createTestTriage(t, repo, id, ticket.ID, priority, at)
			}

			got, err := repo.GetLatestByTicketID(ctx, ticket.ID)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got.ID != highID {
				t.Errorf("expected latest triage %s, got %s", highID, got.ID)
			}

			if !got.CreatedAt.Equal(at) {
				t.Errorf("expected created_at %v, got %v", at, got.CreatedAt)
			}

			// TicketRepository.GetByID must agree on which triage is the latest.
			gotTicket, err := ticketRepo.GetByID(ctx, ticket.ID)
			if err != nil {
				t.Fatalf("expected no error from GetByID, got %v", err)
			}

			if gotTicket.LatestTriagePriority == nil {
				t.Fatalf("expected GetByID latest priority %q, got nil", got.Priority)
			}

			if *gotTicket.LatestTriagePriority != got.Priority {
				t.Errorf("expected GetByID latest priority %q, got %q", got.Priority, *gotTicket.LatestTriagePriority)
			}
		})
	}
}

func TestTicketTriageRepositoryListByTicketIDBreaksCreatedAtTiesByID(t *testing.T) {
	db := requireTestDB(t)
	ticketRepo := NewTicketRepository(db)
	repo := NewTicketTriageRepository(db)

	at := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	lowID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	middleID := uuid.MustParse("7fffffff-ffff-ffff-ffff-ffffffffffff")
	highID := uuid.MustParse("bfffffff-ffff-ffff-ffff-ffffffffffff")
	// The older triage has the greatest id, so the id tiebreaker must not
	// override created_at when the timestamps differ.
	olderID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	ticket := createTestTicket(t, ticketRepo, "Tied triage history")

	// Insert in an order that matches neither the expected result nor its
	// reverse, so insertion order cannot decide the result.
	createTestTriage(t, repo, lowID, ticket.ID, "low", at)
	createTestTriage(t, repo, olderID, ticket.ID, "low", at.Add(-time.Microsecond))
	createTestTriage(t, repo, highID, ticket.ID, "critical", at)
	createTestTriage(t, repo, middleID, ticket.ID, "low", at)

	got, err := repo.ListByTicketID(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []uuid.UUID{highID, middleID, lowID, olderID}

	if len(got) != len(want) {
		t.Fatalf("expected %d triages, got %d", len(want), len(got))
	}

	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("triage %d: expected %s, got %s", i, id, got[i].ID)
		}
	}

	// The first history entry must be the triage GetLatestByTicketID returns.
	latest, err := repo.GetLatestByTicketID(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("expected no error from GetLatestByTicketID, got %v", err)
	}

	if got[0].ID != latest.ID {
		t.Errorf("expected first history entry %s to be the latest triage %s", got[0].ID, latest.ID)
	}
}

func TestTicketTriageRepositoryCreateRejectsUnknownTicket(t *testing.T) {
	db := requireTestDB(t)
	repo := NewTicketTriageRepository(db)
	ctx := context.Background()

	// The repository does not check the ticket itself, so only the
	// ticket_triages.ticket_id foreign key can reject this row.
	triage := &models.TicketTriage{
		ID:              uuid.New(),
		TicketID:        uuid.New(),
		Category:        "technical",
		Priority:        "low",
		Sentiment:       "neutral",
		SuggestedTeam:   "support",
		Summary:         "Test summary",
		SuggestedAction: "Test action",
		Confidence:      0.9,
		Model:           "test-model",
	}

	err := repo.Create(ctx, triage)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a PostgreSQL error, got %v", err)
	}

	// 23503 is foreign_key_violation.
	if pgErr.Code != "23503" {
		t.Errorf("expected SQLSTATE 23503, got %s", pgErr.Code)
	}

	if pgErr.ConstraintName != "ticket_triages_ticket_id_fkey" {
		t.Errorf("expected constraint ticket_triages_ticket_id_fkey, got %q", pgErr.ConstraintName)
	}

	var count int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM ticket_triages WHERE id = $1", triage.ID).Scan(&count); err != nil {
		t.Fatalf("failed to count triages: %v", err)
	}

	if count != 0 {
		t.Errorf("expected no triage to be saved, got %d", count)
	}
}
