package api

import (
	"fmt"
	"os"
)

// ClientInterface defines the common interface for both DeepInfra and Ollama clients
type ClientInterface interface {
	SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error)
	CheckConnection() error
	SetDebug(debug bool)
	SetModel(model string) error
}

// ClientType represents the type of client to use
type ClientType string

const (
	DeepInfraClientType ClientType = "deepinfra"
	OllamaClientType    ClientType = "ollama"
)

// NewUnifiedClient creates either a DeepInfra or Ollama client based on the specified type
func NewUnifiedClient(clientType ClientType) (ClientInterface, error) {
	return NewUnifiedClientWithModel(clientType, "")
}

// NewUnifiedClientWithModel creates a client with a specific model
func NewUnifiedClientWithModel(clientType ClientType, model string) (ClientInterface, error) {
	switch clientType {
	case DeepInfraClientType:
		var client *Client
		var err error
		if model != "" {
			client, err = NewClientWithModel(model)
		} else {
			client, err = NewClient()
		}
		if err != nil {
			return nil, err
		}
		return &DeepInfraClientWrapper{client}, nil
	case OllamaClientType:
		return NewOllamaClient()
	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// GetClientTypeFromEnv determines which client to use based on environment variables
func GetClientTypeFromEnv() ClientType {
	// If DEEPINFRA_API_KEY is set, use DeepInfra
	if apiKey := os.Getenv("DEEPINFRA_API_KEY"); apiKey != "" {
		return DeepInfraClientType
	}

	// Otherwise, default to Ollama for local inference
	return OllamaClientType
}

// DeepInfraClientWrapper wraps the existing DeepInfra client to implement ClientInterface
type DeepInfraClientWrapper struct {
	client *Client
}

func (w *DeepInfraClientWrapper) SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error) {
	req := ChatRequest{
		Model:     w.client.model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 100000,
		Reasoning: reasoning,
	}
	return w.client.SendChatRequest(req)
}

func (w *DeepInfraClientWrapper) CheckConnection() error {
	// For DeepInfra, we just check if the API key is set
	if os.Getenv("DEEPINFRA_API_KEY") == "" {
		return fmt.Errorf("DEEPINFRA_API_KEY environment variable not set")
	}
	return nil
}

func (w *DeepInfraClientWrapper) SetDebug(debug bool) {
	w.client.debug = debug
}

func (w *DeepInfraClientWrapper) SetModel(model string) error {
	w.client.model = model
	return nil
}
