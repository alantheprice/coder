package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	OllamaURL   = "http://localhost:11434/api/chat"
	OllamaModel = "gpt-oss:20b"
)

type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

type OllamaMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Tools    []Tool                 `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type OllamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
	// Note: Ollama doesn't provide token usage stats like DeepInfra
	// We'll need to estimate or skip this feature for local models
}

func NewOllamaClient() (*OllamaClient, error) {
	return &OllamaClient{
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Longer timeout for local inference
		},
		baseURL: OllamaURL,
		model:   OllamaModel,
	}, nil
}

func (c *OllamaClient) SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error) {
	// Convert our Message format to OllamaMessage format
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	options := make(map[string]interface{})

	// Set reasoning effort for gpt-oss models
	if reasoning != "" {
		options["reasoning_effort"] = reasoning
	}

	// Set temperature and other params
	options["temperature"] = 0.7
	options["top_p"] = 0.9

	req := OllamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Tools:    tools,
		Stream:   false,
		Options:  options,
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

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert Ollama response to our ChatResponse format
	chatResp := &ChatResponse{
		ID:      "ollama-" + time.Now().Format("20060102150405"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   c.model,
		Choices: []Choice{
			{
				Index: 0,
				Message: struct {
					Role      string     `json:"role"`
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls,omitempty"`
				}{
					Role:      ollamaResp.Message.Role,
					Content:   ollamaResp.Message.Content,
					ToolCalls: ollamaResp.Message.ToolCalls,
				},
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			EstimatedCost    float64 `json:"estimated_cost"`
		}{
			// Ollama doesn't provide token counts, so we estimate
			PromptTokens:     len(reqBody) / 4, // Rough estimate: 4 chars per token
			CompletionTokens: len(ollamaResp.Message.Content) / 4,
			TotalTokens:      (len(reqBody) + len(ollamaResp.Message.Content)) / 4,
			EstimatedCost:    0.0, // Local inference is free!
		},
	}

	return chatResp, nil
}

func (c *OllamaClient) CheckConnection() error {
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

func (c *OllamaClient) SetModel(model string) {
	c.model = model
}
