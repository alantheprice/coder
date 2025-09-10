package main

import (
	"fmt"
	"net/http"
	"log"
	"encoding/json"
)

// Simple HTTP server with a bug in the /status endpoint
func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/users", usersHandler)
	http.HandleFunc("/server-info", serverInfoHandler)
	
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
		"status": "running",
		"uptime": "unknown"
	}`
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, status)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: This needs to be implemented
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintf(w, "Users endpoint not implemented yet")
}

// GetServerInfo returns server information as a JSON response
func GetServerInfo() (map[string]interface{}, error) {
	info := map[string]interface{}{
		"name": "Simple Server",
		"version": "1.0.0",
		"status": "running",
		"endpoints": []string{"/", "/status", "/users", "/server-info"},
	}
	
	return info, nil
}

func serverInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, err := GetServerInfo()
	if err != nil {
		http.Error(w, "Failed to get server info", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}