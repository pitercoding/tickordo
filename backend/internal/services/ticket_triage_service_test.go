package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/models"
	"github.com/pitercoding/tickordo/internal/repositories"
)

// fakeTicketAnalyzer stands in for the OpenAI analyzer so no real API call
// is made. It records the ticket it received and returns triage and err.
type fakeTicketAnalyzer struct {
	triage *models.TicketTriage
	err    error

	calls     int
	gotTicket *models.Ticket
}

func (f *fakeTicketAnalyzer) Analyze(ctx context.Context, ticket *models.Ticket) (*models.TicketTriage, error) {
	f.calls++
	f.gotTicket = ticket
	return f.triage, f.err
}

// fakeTicketTriageRepository stands in for *repositories.TicketTriageRepository.
// Every method records how it was called and returns err.
type fakeTicketTriageRepository struct {
	// triages is returned by ListByTicketID.
	triages []*models.TicketTriage
	// triage is returned by GetLatestByTicketID.
	triage *models.TicketTriage
	err    error

	createCalls    int
	created        *models.TicketTriage
	createdID      uuid.UUID
	listCalls      int
	getLatestCalls int
	gotTicketID    uuid.UUID
}

func (f *fakeTicketTriageRepository) Create(ctx context.Context, triage *models.TicketTriage) error {
	f.createCalls++
	f.created = triage
	// Copy the ID at call time to prove it was set before saving.
	f.createdID = triage.ID
	return f.err
}

func (f *fakeTicketTriageRepository) ListByTicketID(ctx context.Context, ticketID uuid.UUID) ([]*models.TicketTriage, error) {
	f.listCalls++
	f.gotTicketID = ticketID
	return f.triages, f.err
}

func (f *fakeTicketTriageRepository) GetLatestByTicketID(ctx context.Context, ticketID uuid.UUID) (*models.TicketTriage, error) {
	f.getLatestCalls++
	f.gotTicketID = ticketID
	return f.triage, f.err
}

func TestTicketTriageServiceAnalyzeTicket_SavesAnalyzerResult(t *testing.T) {
	ticket := &models.Ticket{ID: uuid.New(), Title: "Login fails", Description: "I cannot sign in."}
	analyzerTriage := &models.TicketTriage{TicketID: ticket.ID, Category: "authentication", Priority: "high"}

	tickets := &fakeTicketRepository{ticket: ticket}
	analyzer := &fakeTicketAnalyzer{triage: analyzerTriage}
	triages := &fakeTicketTriageRepository{}
	service := NewTicketTriageService(tickets, triages, analyzer)

	triage, err := service.AnalyzeTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("AnalyzeTicket returned error: %v", err)
	}

	if tickets.getByIDCalls != 1 || tickets.gotID != ticket.ID {
		t.Errorf("ticket GetByID calls = %d with ID %s, want 1 with ID %s", tickets.getByIDCalls, tickets.gotID, ticket.ID)
	}

	// The analyzer must receive the ticket loaded from the repository.
	if analyzer.calls != 1 {
		t.Fatalf("analyzer calls = %d, want 1", analyzer.calls)
	}

	if analyzer.gotTicket != ticket {
		t.Error("analyzer did not receive the ticket loaded from the repository")
	}

	// The analyzer result is saved once, with a newly generated ID.
	if triages.createCalls != 1 {
		t.Fatalf("triage Create calls = %d, want 1", triages.createCalls)
	}

	if triages.created != analyzerTriage {
		t.Error("saved triage is not the analyzer result")
	}

	if triages.createdID == uuid.Nil {
		t.Error("triage ID was not generated before saving")
	}

	// The service returns the triage it saved.
	if triage != analyzerTriage {
		t.Error("returned triage is not the saved triage")
	}

	if triage.ID != triages.createdID {
		t.Errorf("returned triage ID = %s, want saved ID %s", triage.ID, triages.createdID)
	}
}

func TestTicketTriageServiceAnalyzeTicket_StopsOnError(t *testing.T) {
	ticket := &models.Ticket{ID: uuid.New(), Title: "Login fails", Description: "I cannot sign in."}
	databaseErr := errors.New("connection refused")
	analyzerErr := errors.New("openai timeout")

	tests := []struct {
		name        string
		ticketErr   error
		analyzerErr error
		createErr   error
		wantErr     error
		// Expected calls show which steps ran before the failure stopped the flow.
		wantAnalyzerCalls int
		wantCreateCalls   int
	}{
		{
			// No OpenAI call (and no cost) for a ticket that does not exist.
			name:              "ticket not found",
			ticketErr:         repositories.ErrTicketNotFound,
			wantErr:           repositories.ErrTicketNotFound,
			wantAnalyzerCalls: 0,
			wantCreateCalls:   0,
		},
		{
			name:              "ticket repository error",
			ticketErr:         databaseErr,
			wantErr:           databaseErr,
			wantAnalyzerCalls: 0,
			wantCreateCalls:   0,
		},
		{
			// A failed analysis must not leave a triage row behind.
			name:              "analyzer error",
			analyzerErr:       analyzerErr,
			wantErr:           analyzerErr,
			wantAnalyzerCalls: 1,
			wantCreateCalls:   0,
		},
		{
			name:              "triage repository error",
			createErr:         databaseErr,
			wantErr:           databaseErr,
			wantAnalyzerCalls: 1,
			wantCreateCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tickets := &fakeTicketRepository{err: tt.ticketErr}
			if tt.ticketErr == nil {
				tickets.ticket = ticket
			}

			analyzer := &fakeTicketAnalyzer{err: tt.analyzerErr}
			if tt.analyzerErr == nil {
				analyzer.triage = &models.TicketTriage{TicketID: ticket.ID}
			}

			triages := &fakeTicketTriageRepository{err: tt.createErr}
			service := NewTicketTriageService(tickets, triages, analyzer)

			triage, err := service.AnalyzeTicket(context.Background(), ticket.ID)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want it to wrap %v", err, tt.wantErr)
			}

			// A triage that was not saved must not be returned.
			if triage != nil {
				t.Errorf("triage = %+v, want nil", triage)
			}

			if analyzer.calls != tt.wantAnalyzerCalls {
				t.Errorf("analyzer calls = %d, want %d", analyzer.calls, tt.wantAnalyzerCalls)
			}

			if triages.createCalls != tt.wantCreateCalls {
				t.Errorf("triage Create calls = %d, want %d", triages.createCalls, tt.wantCreateCalls)
			}
		})
	}
}

func TestTicketTriageServiceListTriagesByTicketID_ReturnsTriages(t *testing.T) {
	ticketID := uuid.New()
	want := []*models.TicketTriage{
		{ID: uuid.New(), TicketID: ticketID, Category: "billing"},
		{ID: uuid.New(), TicketID: ticketID, Category: "authentication"},
	}

	tickets := &fakeTicketRepository{ticket: &models.Ticket{ID: ticketID}}
	triages := &fakeTicketTriageRepository{triages: want}
	analyzer := &fakeTicketAnalyzer{}
	service := NewTicketTriageService(tickets, triages, analyzer)

	got, err := service.ListTriagesByTicketID(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("ListTriagesByTicketID returned error: %v", err)
	}

	if tickets.getByIDCalls != 1 || tickets.gotID != ticketID {
		t.Errorf("ticket GetByID calls = %d with ID %s, want 1 with ID %s", tickets.getByIDCalls, tickets.gotID, ticketID)
	}

	if triages.listCalls != 1 || triages.gotTicketID != ticketID {
		t.Errorf("triage ListByTicketID calls = %d with ID %s, want 1 with ID %s", triages.listCalls, triages.gotTicketID, ticketID)
	}

	// The service keeps the repository order (newest first).
	if len(got) != len(want) {
		t.Fatalf("got %d triages, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("triage %d is not the one returned by the repository", i)
		}
	}

	// Reading history must never trigger a new analysis.
	if analyzer.calls != 0 {
		t.Errorf("analyzer calls = %d, want 0", analyzer.calls)
	}
}

func TestTicketTriageServiceGetLatestTriageByTicketID_ReturnsTriage(t *testing.T) {
	ticketID := uuid.New()
	want := &models.TicketTriage{ID: uuid.New(), TicketID: ticketID, Category: "billing"}

	tickets := &fakeTicketRepository{ticket: &models.Ticket{ID: ticketID}}
	triages := &fakeTicketTriageRepository{triage: want}
	analyzer := &fakeTicketAnalyzer{}
	service := NewTicketTriageService(tickets, triages, analyzer)

	got, err := service.GetLatestTriageByTicketID(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("GetLatestTriageByTicketID returned error: %v", err)
	}

	if tickets.getByIDCalls != 1 || tickets.gotID != ticketID {
		t.Errorf("ticket GetByID calls = %d with ID %s, want 1 with ID %s", tickets.getByIDCalls, tickets.gotID, ticketID)
	}

	if triages.getLatestCalls != 1 || triages.gotTicketID != ticketID {
		t.Errorf("triage GetLatestByTicketID calls = %d with ID %s, want 1 with ID %s", triages.getLatestCalls, triages.gotTicketID, ticketID)
	}

	if got != want {
		t.Error("returned triage is not the one returned by the repository")
	}

	if analyzer.calls != 0 {
		t.Errorf("analyzer calls = %d, want 0", analyzer.calls)
	}
}

func TestTicketTriageService_ReadMethodsStopOnError(t *testing.T) {
	ticketID := uuid.New()
	databaseErr := errors.New("connection refused")

	list := func(s *TicketTriageService) (any, error) {
		triages, err := s.ListTriagesByTicketID(context.Background(), ticketID)
		if triages != nil {
			return triages, err
		}
		return nil, err
	}
	getLatest := func(s *TicketTriageService) (any, error) {
		triage, err := s.GetLatestTriageByTicketID(context.Background(), ticketID)
		if triage != nil {
			return triage, err
		}
		return nil, err
	}

	tests := []struct {
		name      string
		call      func(s *TicketTriageService) (any, error)
		ticketErr error
		triageErr error
		wantErr   error
		// notWantErr guards errors the handler must tell apart.
		notWantErr error
		// wantTriageCalls counts calls to the triage repository method used.
		wantTriageCalls int
	}{
		{
			// The ticket existence check runs first; history is not read.
			name:            "List/ticket not found",
			call:            list,
			ticketErr:       repositories.ErrTicketNotFound,
			wantErr:         repositories.ErrTicketNotFound,
			wantTriageCalls: 0,
		},
		{
			name:            "List/triage repository error",
			call:            list,
			triageErr:       databaseErr,
			wantErr:         databaseErr,
			wantTriageCalls: 1,
		},
		{
			name:            "GetLatest/ticket not found",
			call:            getLatest,
			ticketErr:       repositories.ErrTicketNotFound,
			wantErr:         repositories.ErrTicketNotFound,
			notWantErr:      repositories.ErrTicketTriageNotFound,
			wantTriageCalls: 0,
		},
		{
			// The ticket exists but has never been triaged: the handler shows
			// a different 404 message, so the two errors must stay distinct.
			name:            "GetLatest/triage not found",
			call:            getLatest,
			triageErr:       repositories.ErrTicketTriageNotFound,
			wantErr:         repositories.ErrTicketTriageNotFound,
			notWantErr:      repositories.ErrTicketNotFound,
			wantTriageCalls: 1,
		},
		{
			name:            "GetLatest/triage repository error",
			call:            getLatest,
			triageErr:       databaseErr,
			wantErr:         databaseErr,
			wantTriageCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tickets := &fakeTicketRepository{err: tt.ticketErr}
			if tt.ticketErr == nil {
				tickets.ticket = &models.Ticket{ID: ticketID}
			}

			triages := &fakeTicketTriageRepository{err: tt.triageErr}
			analyzer := &fakeTicketAnalyzer{}
			service := NewTicketTriageService(tickets, triages, analyzer)

			result, err := tt.call(service)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want it to wrap %v", err, tt.wantErr)
			}

			if tt.notWantErr != nil && errors.Is(err, tt.notWantErr) {
				t.Errorf("error = %v, must not wrap %v", err, tt.notWantErr)
			}

			if result != nil {
				t.Errorf("result = %+v, want nil", result)
			}

			triageCalls := triages.listCalls + triages.getLatestCalls
			if triageCalls != tt.wantTriageCalls {
				t.Errorf("triage repository calls = %d, want %d", triageCalls, tt.wantTriageCalls)
			}

			if triages.createCalls != 0 {
				t.Errorf("triage Create calls = %d, want 0", triages.createCalls)
			}

			if analyzer.calls != 0 {
				t.Errorf("analyzer calls = %d, want 0", analyzer.calls)
			}
		})
	}
}
