package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenRouterProvider implements the OpenAI-compatible OpenRouter API
type OpenRouterProvider struct {
	httpClient *http.Client
	apiToken   string
	debug      bool
	model      string
}

// NewOpenRouterProvider creates a new OpenRouter provider instance
func NewOpenRouterProvider() (*OpenRouterProvider, error) {
	token := os.Getenv("OPENROUTER_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	return &OpenRouterProvider{
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		apiToken: token,
		debug:    false,
		model:    "openrouter/openai/gpt-4o", // Default OpenRouter model
	}, nil
}

// NewOpenRouterProviderWithModel creates an OpenRouter provider with a specific model
func NewOpenRouterProviderWithModel(model string) (*OpenRouterProvider, error) {
	provider, err := NewOpenRouterProvider()
	if err != nil {
		return nil, err
	}
	provider.model = model
	return provider, nil
}

// SendChatRequest sends a chat completion request to OpenRouter
func (p *OpenRouterProvider) SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error) {
	req := ChatRequest{
		Model:     p.model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 100000,
		Reasoning: reasoning,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiToken)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/alantheprice/coder") // Required by OpenRouter
	httpReq.Header.Set("X-Title", "Coder AI Assistant")                         // Required by OpenRouter

	// Log the request for debugging
	if p.debug {
		log.Printf("OpenRouter Request URL: %s", "https://openrouter.ai/api/v1/chat/completions")
		log.Printf("OpenRouter Request Headers: %v", httpReq.Header)
		log.Printf("OpenRouter Request Body: %s", string(reqBody))
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log the response for debugging
	respBody, _ := io.ReadAll(resp.Body)
	if p.debug {
		log.Printf("OpenRouter Response Status: %s", resp.Status)
		log.Printf("OpenRouter Response Headers: %v", resp.Header)
		log.Printf("OpenRouter Response Body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// CheckConnection checks if the OpenRouter connection is valid
func (p *OpenRouterProvider) CheckConnection() error {
	if p.apiToken == "" {
		return fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}
	return nil
}

// SetDebug enables or disables debug mode
func (p *OpenRouterProvider) SetDebug(debug bool) {
	p.debug = debug
}

// SetModel sets the model to use
func (p *OpenRouterProvider) SetModel(model string) error {
	p.model = model
	return nil
}

// GetModel returns the current model
func (p *OpenRouterProvider) GetModel() string {
	return p.model
}

// GetProvider returns the provider name
func (p *OpenRouterProvider) GetProvider() string {
	return "openrouter"
}