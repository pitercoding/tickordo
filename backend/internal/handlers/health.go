package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func Health(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
	}

	w.Header().Set("Content-Type", "application/json")

	// The status code is already sent once Encode starts writing, so an
	// error here can only be logged, not turned into a 500 response.
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to write health response: %v", err)
	}
}
