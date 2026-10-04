package main

import (
	"fmt"
	"net/http"
)

func main() {
	const address = ":8080"

	server := &http.Server{
		Addr: address,
	}

	fmt.Printf("Tickordo API running on http://localhost%s\n", address)

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}
