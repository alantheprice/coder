package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alantheprice/coder/types"
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
		model:    "deepseek/deepseek-chat-v3.1:free", // Default OpenRouter model
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
func (p *OpenRouterProvider) SendChatRequest(messages []types.Message, tools []types.Tool, reasoning string) (*types.ChatResponse, error) {
	// Convert messages to OpenRouter format
	openRouterMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		openRouterMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	// Build request payload
	requestBody := map[string]interface{}{
		"model":       p.model,
		"messages":    openRouterMessages,
		"max_tokens":  100000,
		"temperature": 0.7,
	}

	// Add tools if provided
	if len(tools) > 0 {
		requestBody["tools"] = tools
		requestBody["tool_choice"] = "auto"
	}

	reqBody, err := json.Marshal(requestBody)
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
		fmt.Printf("🔍 OpenRouter Request URL: %s\n", "https://openrouter.ai/api/v1/chat/completions")
		fmt.Printf("🔍 OpenRouter Request Body: %s\n", string(reqBody))
	}

	return p.sendRequestWithRetry(httpReq, reqBody)
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

// GetModelContextLimit returns the context limit for the current model
func (p *OpenRouterProvider) GetModelContextLimit() (int, error) {
	model := p.model
	
	// Common OpenRouter model context limits
	switch {
	case strings.Contains(model, "deepseek-chat"):
		return 64000, nil  // DeepSeek Chat supports 64K context
	case strings.Contains(model, "deepseek"):
		return 32000, nil  // Other DeepSeek models
	case strings.Contains(model, "claude-3.5-sonnet"):
		return 200000, nil
	case strings.Contains(model, "claude-3-opus"):
		return 200000, nil
	case strings.Contains(model, "claude-3-sonnet"):
		return 200000, nil
	case strings.Contains(model, "claude-3-haiku"):
		return 200000, nil
	case strings.Contains(model, "gpt-4o"):
		return 128000, nil
	case strings.Contains(model, "gpt-4"):
		return 32000, nil
	case strings.Contains(model, "gemini-pro"):
		return 128000, nil
	case strings.Contains(model, "llama-3.1-405b"):
		return 32000, nil
	case strings.Contains(model, "llama-3.1-70b"):
		return 32000, nil
	case strings.Contains(model, "llama-3.1-8b"):
		return 32000, nil
	default:
		return 32000, nil // Conservative default
	}
}

// sendRequestWithRetry implements exponential backoff retry logic for rate limits
func (p *OpenRouterProvider) sendRequestWithRetry(httpReq *http.Request, reqBody []byte) (*types.ChatResponse, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone the request body for retry attempts
		httpReq.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response body: %w", readErr)
		}

		// Log the response for debugging
		if p.debug {
			fmt.Printf("🔍 OpenRouter Response Status (attempt %d): %s\n", attempt+1, resp.Status)
			fmt.Printf("🔍 OpenRouter Response Body: %s\n", string(respBody))
		}

		// Success case
		if resp.StatusCode == http.StatusOK {
			var chatResp types.ChatResponse
			if err := json.Unmarshal(respBody, &chatResp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response: %w", err)
			}
			return &chatResp, nil
		}

		// Handle error cases
		if resp.StatusCode == 429 {
			// Parse the error response to check for daily limits
			var errorResp map[string]interface{}
			if err := json.Unmarshal(respBody, &errorResp); err == nil {
				if errorObj, ok := errorResp["error"].(map[string]interface{}); ok {
					if message, ok := errorObj["message"].(string); ok {
						// Check for daily limit - don't retry these
						if strings.Contains(strings.ToLower(message), "daily limit") {
							return nil, fmt.Errorf("daily limit exceeded: %s", message)
						}
						
						// For rate limits, implement backoff
						if attempt < maxRetries {
							// Check for rate limit headers to get reset time
							waitTime := p.calculateBackoffDelay(resp, attempt, baseDelay)
							fmt.Printf("⏳ Rate limit hit (attempt %d/%d), waiting %v before retry...\n", 
								attempt+1, maxRetries+1, waitTime)
							time.Sleep(waitTime)
							continue
						}
					}
				}
			}
		}

		// For non-retry errors or max retries exceeded
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// calculateBackoffDelay calculates the delay for exponential backoff
func (p *OpenRouterProvider) calculateBackoffDelay(resp *http.Response, attempt int, baseDelay time.Duration) time.Duration {
	// Try to use X-RateLimit-Reset header if available
	if resetHeader := resp.Header.Get("X-RateLimit-Reset"); resetHeader != "" {
		if resetTime, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
			// Convert from milliseconds to time
			resetAt := time.Unix(resetTime/1000, (resetTime%1000)*1000000)
			waitTime := time.Until(resetAt)
			
			// Add small buffer and cap at reasonable maximum
			waitTime += 2 * time.Second
			if waitTime > 60*time.Second {
				waitTime = 60 * time.Second
			}
			if waitTime > 0 {
				return waitTime
			}
		}
	}

	// Fallback to exponential backoff
	delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
	// Cap at 60 seconds
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}
	return delay
}