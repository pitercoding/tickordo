package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
