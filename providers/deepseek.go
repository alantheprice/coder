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

// DeepSeekProvider implements the OpenAI-compatible DeepSeek API
// DeepSeek API endpoint: https://api.deepseek.com/v1/chat/completions
type DeepSeekProvider struct {
	httpClient *http.Client
	apiToken   string
	debug      bool
	model      string
}

// NewDeepSeekProvider creates a new DeepSeek provider instance
func NewDeepSeekProvider() (*DeepSeekProvider, error) {
	token := os.Getenv("DEEPSEEK_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY environment variable not set")
	}

	return &DeepSeekProvider{
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		apiToken: token,
		debug:    false,
		model:    "deepseek-chat", // Default DeepSeek model
	}, nil
}

// NewDeepSeekProviderWithModel creates a DeepSeek provider with a specific model
func NewDeepSeekProviderWithModel(model string) (*DeepSeekProvider, error) {
	provider, err := NewDeepSeekProvider()
	if err != nil {
		return nil, err
	}
	provider.model = model
	return provider, nil
}

// GetEndpoint returns the DeepSeek API endpoint
func (p *DeepSeekProvider) GetEndpoint() string {
	return "https://api.deepseek.com/v1/chat/completions"
}

// GetAPIKey returns the DeepSeek API key
func (p *DeepSeekProvider) GetAPIKey() string {
	return p.apiToken
}

// GetModel returns the current model
func (p *DeepSeekProvider) GetModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *DeepSeekProvider) SetModel(model string) {
	p.model = model
}

// SetDebug enables or disables debug mode
func (p *DeepSeekProvider) SetDebug(debug bool) {
	p.debug = debug
}

// GetHTTPClient returns the HTTP client
func (p *DeepSeekProvider) GetHTTPClient() *http.Client {
	return p.httpClient
}

// IsDebug returns whether debug mode is enabled
func (p *DeepSeekProvider) IsDebug() bool {
	return p.debug
}

// GetProviderName returns the provider name
func (p *DeepSeekProvider) GetProviderName() string {
	return "deepseek"
}

// SendChatRequest sends a chat completion request to DeepSeek API
func (p *DeepSeekProvider) SendChatRequest(messages []interface{}, tools []interface{}, reasoning string) (interface{}, error) {
	// Prepare the request payload
	payload := map[string]interface{}{
		"model":       p.model,
		"messages":    messages,
		"max_tokens":  100000,
	}

	// Add tools if provided
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	// Add reasoning if provided
	if reasoning != "" {
		payload["reasoning"] = reasoning
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", p.GetEndpoint(), strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiToken)

	// Log request for debugging
	if p.debug {
		fmt.Printf("DeepSeek Request URL: %s\n", p.GetEndpoint())
		fmt.Printf("DeepSeek Request Headers: %v\n", req.Header)
		fmt.Printf("DeepSeek Request Body: %s\n", string(jsonData))
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response for debugging
	if p.debug {
		fmt.Printf("DeepSeek Response Status: %s\n", resp.Status)
		fmt.Printf("DeepSeek Response Body: %s\n", string(respBody))
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response, nil
}

// CheckConnection verifies that the provider can connect (API key is set)
func (p *DeepSeekProvider) CheckConnection() error {
	if p.apiToken == "" {
		return fmt.Errorf("DEEPSEEK_API_KEY environment variable not set")
	}
	return nil
}

// GetSupportedModels returns the list of supported models for DeepSeek
func (p *DeepSeekProvider) GetSupportedModels() []string {
	return []string{
		"deepseek-chat",
		"deepseek-coder",
		"deepseek-llm",
	}
}

// GetDefaultModel returns the default model for DeepSeek
func (p *DeepSeekProvider) GetDefaultModel() string {
	return "deepseek-chat"
}