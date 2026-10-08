package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/pitercoding/tickordo/internal/repositories"
	"github.com/pitercoding/tickordo/internal/services"
)

const (
	// maxTitleLength matches the VARCHAR(255) limit of tickets.title.
	maxTitleLength = 255

	// maxDescriptionLength is an application rule (the column is TEXT): it
	// keeps tickets readable and bounds what is sent to OpenAI for triage.
	maxDescriptionLength = 5000

	// maxRequestBodyBytes caps JSON request bodies so a client cannot send
	// an arbitrarily large payload (1 MB).
	maxRequestBodyBytes = 1 << 20
)

type TicketHandler struct {
	service *services.TicketService
}

type createTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateTicketStatusRequest struct {
	Status string `json:"status"`
}

func NewTicketHandler(service *services.TicketService) *TicketHandler {
	return &TicketHandler{
		service: service,
	}
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createTicketRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)

	if request.Title == "" || request.Description == "" {
		writeError(w, "title and description are required", http.StatusBadRequest)
		return
	}

	if utf8.RuneCountInString(request.Title) > maxTitleLength {
		writeError(w, "title must be at most 255 characters", http.StatusBadRequest)
		return
	}

	if utf8.RuneCountInString(request.Description) > maxDescriptionLength {
		writeError(w, "description must be at most 5000 characters", http.StatusBadRequest)
		return
	}

	ticket, err := h.service.CreateTicket(
		r.Context(),
		request.Title,
		request.Description,
	)
	if err != nil {
		log.Printf("failed to create ticket: %v", err)
		writeError(w, "failed to create ticket", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(ticket)
	if err != nil {
		log.Printf("failed to encode ticket response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write ticket response: %v", err)
	}
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	tickets, err := h.service.ListTickets(r.Context())
	if err != nil {
		log.Printf("failed to list tickets: %v", err)
		writeError(w, "failed to list tickets", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(tickets)
	if err != nil {
		log.Printf("failed to marshal tickets response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write tickets response: %v", err)
	}
}

func (h *TicketHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}

	ticket, err := h.service.GetTicketByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			writeError(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to get ticket: %v", err)
		writeError(w, "failed to get ticket", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(ticket)
	if err != nil {
		log.Printf("failed to marshal ticket response: %v", err)
		writeError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(response); err != nil {
		log.Printf("failed to write ticket response: %v", err)
	}
}

func (h *TicketHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}

	var request updateTicketStatusRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	request.Status = strings.TrimSpace(request.Status)

	switch request.Status {
	case "open", "in_progress", "resolved":
		// Valid status.
	default:
		writeError(w, "invalid ticket status", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateTicketStatus(
		r.Context(),
		id,
		request.Status,
	); err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			writeError(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to update ticket status: %v", err)
		writeError(w, "failed to update ticket status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
