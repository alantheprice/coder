package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	OllamaURL   = "http://localhost:11434/v1/chat/completions"
	OllamaModel = "gpt-oss:20b"
)

type LocalOllamaClient struct {
	httpClient *http.Client
	baseURL    string
	model      string
	debug      bool
}

// Using OpenAI-compatible endpoint, so we reuse existing ChatRequest and ChatResponse structs

func NewOllamaClient() (*LocalOllamaClient, error) {
	return &LocalOllamaClient{
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Longer timeout for local inference
		},
		baseURL: OllamaURL,
		model:   OllamaModel,
		debug:   false, // Will be set later via SetDebug
	}, nil
}

func (c *LocalOllamaClient) SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error) {
	// Convert to harmony format for gpt-oss models
	formatter := NewHarmonyFormatter()
	harmonyText := formatter.FormatMessagesForCompletion(messages, tools)

	// Create a single message with harmony-formatted text
	req := map[string]interface{}{
		"model":      c.model,
		"messages":   []Message{{Role: "user", Content: harmonyText}},
		"max_tokens": 30000,
		// Note: Don't include tools in harmony format - they're embedded in the text
	}

	// Add reasoning effort if provided (Ollama uses reasoning_effort, not reasoning)
	if reasoning != "" {
		req["reasoning_effort"] = reasoning
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Log the request for debugging
	if c.debug {
		log.Printf("Ollama Request URL: %s", c.baseURL)
		log.Printf("Ollama Request Headers: %v", httpReq.Header)
		log.Printf("Ollama Request Body: %s", string(reqBody))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log the response for debugging
	respBody, _ := io.ReadAll(resp.Body)
	if c.debug {
		log.Printf("Ollama Response Status: %s", resp.Status)
		log.Printf("Ollama Response Headers: %v", resp.Header)
		log.Printf("Ollama Response Body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Set cost to 0 for local inference
	chatResp.Usage.EstimatedCost = 0.0

	return &chatResp, nil
}

func (c *LocalOllamaClient) CheckConnection() error {
	// Check if Ollama is running and gpt-oss model is available
	checkURL := "http://localhost:11434/api/tags"

	resp, err := c.httpClient.Get(checkURL)
	if err != nil {
		return fmt.Errorf("Ollama is not running. Please start Ollama first")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API error (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Ollama tags response: %w", err)
	}

	// Check if gpt-oss model is available
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return fmt.Errorf("failed to parse Ollama tags response: %w", err)
	}

	hasGPTOSS := false
	for _, model := range tagsResp.Models {
		if model.Name == "gpt-oss:20b" || model.Name == "gpt-oss:latest" || model.Name == "gpt-oss" {
			hasGPTOSS = true
			break
		}
	}

	if !hasGPTOSS {
		return fmt.Errorf("gpt-oss:20b model not found. Please run: ollama pull gpt-oss:20b")
	}

	return nil
}

func (c *LocalOllamaClient) SetDebug(debug bool) {
	c.debug = debug
}

func (c *LocalOllamaClient) SetModel(model string) {
	c.model = model
}
