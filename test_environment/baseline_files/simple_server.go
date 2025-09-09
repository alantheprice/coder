package main

import (
	"fmt"
	"net/http"
	"log"
)

// Simple HTTP server with a bug in the /status endpoint
func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/users", usersHandler)
	
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Welcome to the simple server!")
}

// BUG: This handler has an issue - it doesn't set the content type
// and returns malformed JSON
func statusHandler(w http.ResponseWriter, r *http.Request) {
	status := `{
		"status": "running"
		"uptime": "unknown"
	}`
	fmt.Fprintf(w, status)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: This needs to be implemented
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintf(w, "Users endpoint not implemented yet")
}