package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	DeepInfraURL = "https://api.deepinfra.com/v1/openai/chat/completions"
	Model        = "openai/gpt-oss-120b"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Choice struct {
	Index   int `json:"index"`
	Message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		EstimatedCost    float64 `json:"estimated_cost"`
	} `json:"usage"`
}

type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

type ChatRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	ToolChoice string    `json:"tool_choice,omitempty"`
	MaxTokens  int       `json:"max_tokens,omitempty"`
	Reasoning  string    `json:"reasoning,omitempty"`
}

type Client struct {
	httpClient *http.Client
	apiToken   string
}

func NewClient() (*Client, error) {
	token := os.Getenv("DEEPINFRA_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("DEEPINFRA_API_KEY environment variable not set")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		apiToken: token,
	}, nil
}

func (c *Client) SendChatRequest(req ChatRequest) (*ChatResponse, error) {
	// Convert to harmony format for gpt-oss models
	formatter := NewHarmonyFormatter()
	harmonyText := formatter.FormatMessagesForCompletion(req.Messages, req.Tools)

	// Create a single message with harmony-formatted text
	harmonyReq := ChatRequest{
		Model:     req.Model,
		Messages:  []Message{{Role: "user", Content: harmonyText}},
		MaxTokens: req.MaxTokens,
		Reasoning: req.Reasoning,
		// Note: Don't include Tools in harmony format - they're embedded in the text
	}

	reqBody, err := json.Marshal(harmonyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", DeepInfraURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

func GetToolDefinitions() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "shell_command",
				Description: "Execute shell commands to explore directory structure, search files, run programs",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Shell command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "read_file",
				Description: "Read contents of a specific file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to file to read",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "edit_file",
				Description: "Edit existing file by replacing old string with new string",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to file to edit",
						},
						"old_string": map[string]interface{}{
							"type":        "string",
							"description": "Exact string to replace",
						},
						"new_string": map[string]interface{}{
							"type":        "string",
							"description": "New string to replace with",
						},
					},
					"required": []string{"file_path", "old_string", "new_string"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "write_file",
				Description: "Write content to a new file or overwrite existing file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to file to write",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Content to write to file",
						},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "add_todo",
				Description: "Add a new todo item to track task progress",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Brief title of the todo item",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Optional detailed description",
						},
						"priority": map[string]interface{}{
							"type":        "string",
							"description": "Priority level: high, medium, low",
						},
					},
					"required": []string{"title"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "update_todo_status",
				Description: "Update the status of a todo item",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID of the todo item to update",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "New status: pending, in_progress, completed, cancelled",
						},
					},
					"required": []string{"id", "status"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{
				Name:        "list_todos",
				Description: "List all current todos with their status",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
			},
		},
	}
}
