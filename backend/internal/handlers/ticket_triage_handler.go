package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/repositories"
	"github.com/pitercoding/tickordo/internal/services"
)

type TicketTriageHandler struct {
	service *services.TicketTriageService
}

func NewTicketTriageHandler(
	service *services.TicketTriageService,
) *TicketTriageHandler {
	return &TicketTriageHandler{
		service: service,
	}
}

func (h *TicketTriageHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Extract the ticket ID from the URL.
	ticketID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Analyze the ticket and save the AI triage result.
	triage, err := h.service.AnalyzeTicket(r.Context(), ticketID)
	if err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			writeError(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to triage ticket: %v", err)
		writeError(w, "failed to triage ticket", http.StatusInternalServerError)
		return
	}

	// Encode before writing headers so encoding errors can still return 500.
	response, err := json.Marshal(triage)
	if err != nil {
		log.Printf("failed to marshal ticket triage response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Return the AI triage result as JSON.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write ticket triage response: %v", err)
	}
}

func (h *TicketTriageHandler) List(w http.ResponseWriter, r *http.Request) {
	// Extract the ticket ID from the URL.
	ticketID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Load the ticket's triage history.
	triages, err := h.service.ListTriagesByTicketID(r.Context(), ticketID)
	if err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			writeError(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to list ticket triages: %v", err)
		writeError(w, "failed to retrieve ticket triage history", http.StatusInternalServerError)
		return
	}

	// Encode before writing headers so encoding errors can still return 500.
	response, err := json.Marshal(triages)
	if err != nil {
		log.Printf("failed to marshal ticket triages response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Return the triage history as JSON.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write ticket triages response: %v", err)
	}
}

func (h *TicketTriageHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	// Extract the ticket ID from the URL.
	ticketID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}

	// Load the most recent triage for the ticket.
	triage, err := h.service.GetLatestTriageByTicketID(r.Context(), ticketID)
	if err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			writeError(w, "ticket not found", http.StatusNotFound)
			return
		}

		if errors.Is(err, repositories.ErrTicketTriageNotFound) {
			writeError(w, "ticket triage not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to get latest ticket triage: %v", err)
		writeError(w, "failed to retrieve latest ticket triage", http.StatusInternalServerError)
		return
	}

	// Convert the response to JSON before writing the HTTP headers.
	response, err := json.Marshal(triage)
	if err != nil {
		log.Printf("failed to marshal ticket triage: %v", err)
		writeError(w, "failed to retrieve latest ticket triage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write ticket triage response: %v", err)
	}
}
