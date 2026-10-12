package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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
