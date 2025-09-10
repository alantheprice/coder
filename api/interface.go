package api

import (
	"fmt"
	"os"
	"strings"
)

// ClientInterface defines the common interface for all API clients
type ClientInterface interface {
	SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error)
	CheckConnection() error
	SetDebug(debug bool)
	SetModel(model string) error
	GetModel() string
	GetProvider() string
}

// ClientType represents the type of client to use
type ClientType string

const (
	DeepInfraClientType ClientType = "deepinfra"
	OllamaClientType    ClientType = "ollama"
	CerebrasClientType  ClientType = "cerebras"
	OpenRouterClientType ClientType = "openrouter"
	GroqClientType      ClientType = "groq"
	DeepSeekClientType  ClientType = "deepseek"
)

// NewUnifiedClient creates a client based on the specified type
func NewUnifiedClient(clientType ClientType) (ClientInterface, error) {
	return NewUnifiedClientWithModel(clientType, "")
}

// NewUnifiedClientWithModel creates a client with a specific model
func NewUnifiedClientWithModel(clientType ClientType, model string) (ClientInterface, error) {
	switch clientType {
	case DeepInfraClientType:
		return NewDeepInfraClientWrapper(model)
	case OllamaClientType:
		return NewOllamaClient()
	case CerebrasClientType:
		return NewCerebrasClientWrapper(model)
	case OpenRouterClientType:
		return NewOpenRouterClientWrapper(model)
	case GroqClientType:
		return NewGroqClientWrapper(model)
	case DeepSeekClientType:
		return NewDeepSeekClientWrapper(model)
	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// NewDeepInfraClientWrapper creates a DeepInfra client wrapper
func NewDeepInfraClientWrapper(model string) (ClientInterface, error) {
	client, err := NewClientWithModel(model)
	if err != nil {
		return nil, err
	}
	return &DeepInfraClientWrapper{client: client}, nil
}

// NewCerebrasClientWrapper creates a Cerebras client wrapper
func NewCerebrasClientWrapper(model string) (ClientInterface, error) {
	// For now, return an error since Cerebras provider is not fully implemented
	return nil, fmt.Errorf("Cerebras provider not yet implemented")
}

// NewOpenRouterClientWrapper creates an OpenRouter client wrapper
func NewOpenRouterClientWrapper(model string) (ClientInterface, error) {
	// For now, return an error since OpenRouter provider is not fully implemented
	return nil, fmt.Errorf("OpenRouter provider not yet implemented")
}

// NewGroqClientWrapper creates a Groq client wrapper
func NewGroqClientWrapper(model string) (ClientInterface, error) {
	// For now, return an error since Groq provider is not fully implemented
	return nil, fmt.Errorf("Groq provider not yet implemented")
}

// NewDeepSeekClientWrapper creates a DeepSeek client wrapper
func NewDeepSeekClientWrapper(model string) (ClientInterface, error) {
	// For now, return an error since DeepSeek provider is not fully implemented
	return nil, fmt.Errorf("DeepSeek provider not yet implemented")
}

// GetClientTypeFromEnv determines which client to use based on environment variables
func GetClientTypeFromEnv() ClientType {
	// Check provider environment variables in priority order
	envProviders := []struct {
		envVar string
		client ClientType
	}{
		{"DEEPINFRA_API_KEY", DeepInfraClientType},
		{"CEREBRAS_API_KEY", CerebrasClientType},
		{"OPENROUTER_API_KEY", OpenRouterClientType},
		{"GROQ_API_KEY", GroqClientType},
		{"DEEPSEEK_API_KEY", DeepSeekClientType},
	}

	for _, provider := range envProviders {
		if apiKey := os.Getenv(provider.envVar); apiKey != "" {
			return provider.client
		}
	}

	// Otherwise, default to Ollama for local inference
	return OllamaClientType
}

// GetAvailableProviders returns a list of all available providers
func GetAvailableProviders() []ClientType {
	return []ClientType{
		DeepInfraClientType,
		OllamaClientType,
		CerebrasClientType,
		OpenRouterClientType,
		GroqClientType,
		DeepSeekClientType,
	}
}

// GetProviderName returns the human-readable name for a provider
func GetProviderName(clientType ClientType) string {
	switch clientType {
	case DeepInfraClientType:
		return "DeepInfra"
	case OllamaClientType:
		return "Ollama (Local)"
	case CerebrasClientType:
		return "Cerebras"
	case OpenRouterClientType:
		return "OpenRouter"
	case GroqClientType:
		return "Groq"
	case DeepSeekClientType:
		return "DeepSeek"
	default:
		return string(clientType)
	}
}

// GetProviderFromString converts a string to ClientType
func GetProviderFromString(providerStr string) (ClientType, error) {
	providerStr = strings.ToLower(providerStr)
	switch providerStr {
	case "deepinfra":
		return DeepInfraClientType, nil
	case "ollama":
		return OllamaClientType, nil
	case "cerebras":
		return CerebrasClientType, nil
	case "openrouter":
		return OpenRouterClientType, nil
	case "groq":
		return GroqClientType, nil
	case "deepseek":
		return DeepSeekClientType, nil
	default:
		return "", fmt.Errorf("unknown provider: %s", providerStr)
	}
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

func (w *DeepInfraClientWrapper) GetModel() string {
	return w.client.model
}

func (w *DeepInfraClientWrapper) GetProvider() string {
	return "deepinfra"
}
