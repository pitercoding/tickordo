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

// maxTitleLength matches the VARCHAR(255) limit of tickets.title.
const maxTitleLength = 255

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

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)

	if request.Title == "" || request.Description == "" {
		http.Error(w, "title and description are required", http.StatusBadRequest)
		return
	}

	if utf8.RuneCountInString(request.Title) > maxTitleLength {
		http.Error(w, "title must be at most 255 characters", http.StatusBadRequest)
		return
	}

	ticket, err := h.service.CreateTicket(
		r.Context(),
		request.Title,
		request.Description,
	)
	if err != nil {
		log.Printf("failed to create ticket: %v", err)
		http.Error(w, "failed to create ticket", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(ticket)
	if err != nil {
		log.Printf("failed to encode ticket response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
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
		http.Error(w, "failed to list tickets", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(tickets)
	if err != nil {
		log.Printf("failed to marshal tickets response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
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
		http.Error(w, "invalid ticket id", http.StatusBadRequest)
		return
	}

	ticket, err := h.service.GetTicketByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			http.Error(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to get ticket: %v", err)
		http.Error(w, "failed to get ticket", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(ticket)
	if err != nil {
		log.Printf("failed to marshal ticket response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
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
		http.Error(w, "invalid ticket id", http.StatusBadRequest)
		return
	}

	var request updateTicketStatusRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	request.Status = strings.TrimSpace(request.Status)

	switch request.Status {
	case "open", "in_progress", "resolved":
		// Valid status.
	default:
		http.Error(w, "invalid ticket status", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateTicketStatus(
		r.Context(),
		id,
		request.Status,
	); err != nil {
		if errors.Is(err, repositories.ErrTicketNotFound) {
			http.Error(w, "ticket not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to update ticket status: %v", err)
		http.Error(w, "failed to update ticket status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
