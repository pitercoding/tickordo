package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// fakeTicketRepository stands in for *repositories.TicketRepository so the
// services can be tested without PostgreSQL. Every method records how it was
// called and returns err, so a test decides exactly what the service receives
// from the repository layer.
type fakeTicketRepository struct {
	// ticket is returned by GetByID.
	ticket *models.Ticket
	// tickets is returned by List.
	tickets []models.Ticket
	err     error

	createCalls       int
	listCalls         int
	created           *models.Ticket
	getByIDCalls      int
	updateStatusCalls int
	gotID             uuid.UUID
	gotStatus         string
}

func (f *fakeTicketRepository) Create(ctx context.Context, ticket *models.Ticket) error {
	f.createCalls++
	f.created = ticket

	if f.err != nil {
		return f.err
	}

	// The real repository fills the timestamps from RETURNING.
	ticket.CreatedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ticket.UpdatedAt = ticket.CreatedAt

	return nil
}

func (f *fakeTicketRepository) List(ctx context.Context) ([]models.Ticket, error) {
	f.listCalls++
	return f.tickets, f.err
}

func (f *fakeTicketRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error) {
	f.getByIDCalls++
	f.gotID = id
	return f.ticket, f.err
}

func (f *fakeTicketRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	f.updateStatusCalls++
	f.gotID = id
	f.gotStatus = status
	return f.err
}

func TestTicketServiceCreateTicket_CreatesOpenTicket(t *testing.T) {
	repository := &fakeTicketRepository{}
	service := NewTicketService(repository)

	ticket, err := service.CreateTicket(context.Background(), "Login fails", "I cannot sign in.")
	if err != nil {
		t.Fatalf("CreateTicket returned error: %v", err)
	}

	if repository.createCalls != 1 {
		t.Fatalf("repository Create calls = %d, want 1", repository.createCalls)
	}

	// The service returns the same ticket it saved, including the timestamps
	// the repository filled in.
	if ticket != repository.created {
		t.Error("returned ticket is not the ticket passed to the repository")
	}

	if ticket.CreatedAt.IsZero() || ticket.UpdatedAt.IsZero() {
		t.Error("returned ticket is missing the timestamps set by the repository")
	}

	// Business rule: every new ticket gets its own ID and starts open.
	if ticket.ID == uuid.Nil {
		t.Error("ticket ID was not generated")
	}

	if ticket.Status != "open" {
		t.Errorf("ticket status = %q, want %q", ticket.Status, "open")
	}

	// Title and description are stored as received; the handler already
	// trimmed and validated them.
	if ticket.Title != "Login fails" {
		t.Errorf("ticket title = %q, want %q", ticket.Title, "Login fails")
	}

	if ticket.Description != "I cannot sign in." {
		t.Errorf("ticket description = %q, want %q", ticket.Description, "I cannot sign in.")
	}
}

func TestTicketServiceCreateTicket_GeneratesDistinctIDs(t *testing.T) {
	service := NewTicketService(&fakeTicketRepository{})

	first, err := service.CreateTicket(context.Background(), "First", "First ticket.")
	if err != nil {
		t.Fatalf("first CreateTicket returned error: %v", err)
	}

	second, err := service.CreateTicket(context.Background(), "Second", "Second ticket.")
	if err != nil {
		t.Fatalf("second CreateTicket returned error: %v", err)
	}

	if first.ID == second.ID {
		t.Errorf("both tickets got ID %s, want distinct IDs", first.ID)
	}
}

func TestTicketServiceCreateTicket_ReturnsRepositoryError(t *testing.T) {
	repositoryErr := errors.New("connection refused")
	repository := &fakeTicketRepository{err: repositoryErr}
	service := NewTicketService(repository)

	ticket, err := service.CreateTicket(context.Background(), "Login fails", "I cannot sign in.")

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("error = %v, want it to wrap %v", err, repositoryErr)
	}

	// A ticket that was not saved must not be returned as if it existed.
	if ticket != nil {
		t.Errorf("ticket = %+v, want nil", ticket)
	}

	if repository.createCalls != 1 {
		t.Errorf("repository Create calls = %d, want 1", repository.createCalls)
	}
}

func TestTicketService_PreservesTicketNotFound(t *testing.T) {
	// The handlers answer 404 only when errors.Is finds ErrTicketNotFound, so
	// the service must wrap the repository error instead of replacing it.
	ticketID := uuid.New()

	tests := []struct {
		name string
		call func(s *TicketService) error
		// check verifies the arguments the repository received.
		check func(t *testing.T, r *fakeTicketRepository)
	}{
		{
			name: "GetTicketByID",
			call: func(s *TicketService) error {
				ticket, err := s.GetTicketByID(context.Background(), ticketID)
				if ticket != nil {
					t.Errorf("ticket = %+v, want nil", ticket)
				}
				return err
			},
			check: func(t *testing.T, r *fakeTicketRepository) {
				if r.getByIDCalls != 1 {
					t.Errorf("repository GetByID calls = %d, want 1", r.getByIDCalls)
				}
			},
		},
		{
			name: "UpdateTicketStatus",
			call: func(s *TicketService) error {
				return s.UpdateTicketStatus(context.Background(), ticketID, "resolved")
			},
			check: func(t *testing.T, r *fakeTicketRepository) {
				if r.updateStatusCalls != 1 {
					t.Errorf("repository UpdateStatus calls = %d, want 1", r.updateStatusCalls)
				}

				if r.gotStatus != "resolved" {
					t.Errorf("repository got status %q, want %q", r.gotStatus, "resolved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeTicketRepository{err: repositories.ErrTicketNotFound}
			service := NewTicketService(repository)

			err := tt.call(service)

			if !errors.Is(err, repositories.ErrTicketNotFound) {
				t.Fatalf("error = %v, want it to wrap %v", err, repositories.ErrTicketNotFound)
			}

			if repository.gotID != ticketID {
				t.Errorf("repository got ID %s, want %s", repository.gotID, ticketID)
			}

			tt.check(t, repository)
		})
	}
}

func TestTicketServiceListTickets_ReturnsRepositoryTickets(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	want := []models.Ticket{
		{ID: uuid.New(), Title: "Newest", Description: "Second ticket.", Status: "open", CreatedAt: createdAt.Add(time.Hour), UpdatedAt: createdAt.Add(time.Hour)},
		{ID: uuid.New(), Title: "Oldest", Description: "First ticket.", Status: "resolved", CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	repository := &fakeTicketRepository{tickets: want}
	service := NewTicketService(repository)

	tickets, err := service.ListTickets(context.Background())
	if err != nil {
		t.Fatalf("ListTickets returned error: %v", err)
	}

	if repository.listCalls != 1 {
		t.Fatalf("repository List calls = %d, want 1", repository.listCalls)
	}

	// The repository already orders the tickets, so the service must return
	// them unchanged and in the same order.
	if len(tickets) != len(want) {
		t.Fatalf("got %d tickets, want %d", len(tickets), len(want))
	}

	for i := range want {
		if tickets[i] != want[i] {
			t.Errorf("tickets[%d] = %+v, want %+v", i, tickets[i], want[i])
		}
	}
}

func TestTicketServiceListTickets_KeepsEmptySlice(t *testing.T) {
	// The repository returns an empty, non-nil slice when there are no
	// tickets, so the API encodes [] instead of null. The service must not
	// turn it into nil.
	repository := &fakeTicketRepository{tickets: []models.Ticket{}}
	service := NewTicketService(repository)

	tickets, err := service.ListTickets(context.Background())
	if err != nil {
		t.Fatalf("ListTickets returned error: %v", err)
	}

	if tickets == nil {
		t.Fatal("tickets = nil, want an empty slice")
	}

	if len(tickets) != 0 {
		t.Errorf("got %d tickets, want 0", len(tickets))
	}
}

func TestTicketServiceListTickets_ReturnsRepositoryError(t *testing.T) {
	repositoryErr := errors.New("connection refused")
	repository := &fakeTicketRepository{
		// Partial results must be discarded when the repository fails.
		tickets: []models.Ticket{{ID: uuid.New(), Title: "Partial"}},
		err:     repositoryErr,
	}
	service := NewTicketService(repository)

	tickets, err := service.ListTickets(context.Background())

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("error = %v, want it to wrap %v", err, repositoryErr)
	}

	if !strings.HasPrefix(err.Error(), "failed to list tickets: ") {
		t.Errorf("error = %q, want prefix %q", err.Error(), "failed to list tickets: ")
	}

	if tickets != nil {
		t.Errorf("tickets = %+v, want nil", tickets)
	}

	if repository.listCalls != 1 {
		t.Errorf("repository List calls = %d, want 1", repository.listCalls)
	}
}
