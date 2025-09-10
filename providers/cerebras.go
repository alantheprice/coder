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

// CerebrasProvider implements the OpenAI-compatible Cerebras API
type CerebrasProvider struct {
	baseURL    string
	apiToken   string
	model      string
	debug      bool
	httpClient *http.Client
}

// NewCerebrasProvider creates a new Cerebras provider instance
func NewCerebrasProvider() (*CerebrasProvider, error) {
	token := os.Getenv("CEREBRAS_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("CEREBRAS_API_KEY environment variable not set")
	}

	return &CerebrasProvider{
		baseURL:    "https://api.cerebras.ai/v1/chat/completions",
		apiToken:   token,
		model:      "cerebras/btlm-3b-8k-base", // Default Cerebras model
		httpClient: &http.Client{Timeout: 300 * time.Second},
	}, nil
}

// NewCerebrasProviderWithModel creates a Cerebras provider with a specific model
func NewCerebrasProviderWithModel(model string) (*CerebrasProvider, error) {
	provider, err := NewCerebrasProvider()
	if err != nil {
		return nil, err
	}
	provider.model = model
	return provider, nil
}

// SendChatRequest sends a chat completion request to Cerebras API
func (p *CerebrasProvider) SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error) {
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

	httpReq, err := http.NewRequest("POST", p.baseURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiToken)

	if p.debug {
		fmt.Printf("Cerebras Request URL: %s\n", p.baseURL)
		fmt.Printf("Cerebras Request Body: %s\n", string(reqBody))
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if p.debug {
		fmt.Printf("Cerebras Response Status: %s\n", resp.Status)
		fmt.Printf("Cerebras Response Body: %s\n", string(respBody))
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

// CheckConnection checks if the Cerebras API connection is valid
func (p *CerebrasProvider) CheckConnection() error {
	if p.apiToken == "" {
		return fmt.Errorf("CEREBRAS_API_KEY environment variable not set")
	}
	return nil
}

// SetDebug enables or disables debug mode
func (p *CerebrasProvider) SetDebug(debug bool) {
	p.debug = debug
}

// SetModel sets the model to use
func (p *CerebrasProvider) SetModel(model string) error {
	p.model = model
	return nil
}

// GetModel returns the current model
func (p *CerebrasProvider) GetModel() string {
	return p.model
}

// GetProvider returns the provider name
func (p *CerebrasProvider) GetProvider() string {
	return "cerebras"
}