package api

import (
	"fmt"
	"os"
)

// ClientInterface defines the common interface for both DeepInfra and Ollama clients
type ClientInterface interface {
	SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error)
	CheckConnection() error
}

// ClientType represents the type of client to use
type ClientType string

const (
	DeepInfraClientType ClientType = "deepinfra"
	OllamaClientType    ClientType = "ollama"
)

// NewUnifiedClient creates either a DeepInfra or Ollama client based on the specified type
func NewUnifiedClient(clientType ClientType) (ClientInterface, error) {
	switch clientType {
	case DeepInfraClientType:
		client, err := NewClient()
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
		Model:     Model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: 4000,
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
