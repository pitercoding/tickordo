package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorResponse is the JSON body returned for every API error.
type errorResponse struct {
	Error string `json:"error"`
}

// writeError sends an error as JSON, e.g. {"error": "ticket not found"}.
// It takes the same arguments as http.Error, but responds with
// application/json instead of text/plain.
func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The status code is already sent, so an encoding error can only be logged.
	if err := json.NewEncoder(w).Encode(errorResponse{Error: message}); err != nil {
		log.Printf("failed to write error response: %v", err)
	}
}
