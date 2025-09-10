package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alantheprice/coder/api"
)

// GroqProvider implements the OpenAI-compatible Groq API
const GroqProviderName = "groq"

var GroqProvider = ProviderConfig{
	Name:        GroqProviderName,
	DisplayName: "Groq",
	BaseURL:     "https://api.groq.com/openai/v1/chat/completions",
	APIKeyEnv:   "GROQ_API_KEY",
	DefaultModel: "llama-3.3-70b-versatile",
	SupportedModels: []string{
		"llama-3.3-70b-versatile",
		"llama-3.1-8b-instant",
		"llama-3.1-70b-versatile",
		"llama-3.1-405b-reasoning",
		"llama-3.1-405b-reasoning:extended",
		"mixtral-8x7b-32768",
		"gemma2-9b-it",
	},
	IsAvailable: func() bool {
		return os.Getenv("GROQ_API_KEY") != ""
	},
	CreateClient: func(model string) (api.ClientInterface, error) {
		return createGroqClient(model)
	},
	GetModels: func() ([]api.ModelInfo, error) {
		return getGroqModels()
	},
}

func createGroqClient(model string) (api.ClientInterface, error) {
	token := os.Getenv("GROQ_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("GROQ_API_KEY environment variable not set")
	}

	if model == "" {
		model = GroqProvider.DefaultModel
	}

	return &GroqClient{
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		apiToken:   token,
		debug:      false,
		model:      model,
		baseURL:    GroqProvider.BaseURL,
	}, nil
}

func getGroqModels() ([]api.ModelInfo, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not set")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	
	req, err := http.NewRequest("GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	models := make([]api.ModelInfo, len(response.Data))
	for i, model := range response.Data {
		models[i] = api.ModelInfo{
			ID:       model.ID,
			Provider: "Groq",
		}
	}
	
	return models, nil
}

// GroqClient implements the Groq API client
type GroqClient struct {
	httpClient *http.Client
	apiToken   string
	debug      bool
	model      string
	baseURL    string
}

func (c *GroqClient) SendChatRequest(messages []api.Message, tools []api.Tool, reasoning string) (*api.ChatResponse, error) {
	req := api.ChatRequest{
		Model:     c.model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 100000,
		Reasoning: reasoning,
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp api.ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

func (c *GroqClient) CheckConnection() error {
	if os.Getenv("GROQ_API_KEY") == "" {
		return fmt.Errorf("GROQ_API_KEY environment variable not set")
	}
	return nil
}

func (c *GroqClient) SetDebug(debug bool) {
	c.debug = debug
}

func (c *GroqClient) SetModel(model string) error {
	c.model = model
	return nil
}

func (c *GroqClient) GetModel() string {
	return c.model
}

func (c *GroqClient) GetProvider() string {
	return GroqProviderName
}