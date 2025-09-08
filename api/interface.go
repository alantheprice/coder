package api

// ClientInterface defines the common interface for both DeepInfra and Ollama clients
type ClientInterface interface {
	SendChatRequest(messages []Message, tools []Tool, reasoning string) (*ChatResponse, error)
	CheckConnection() error
}

// ClientType represents the type of client to use
type ClientType string

const (
	DeepInfraClient ClientType = "deepinfra"
	OllamaClient    ClientType = "ollama"
)

// NewClient creates either a DeepInfra or Ollama client based on the specified type
func NewUnifiedClient(clientType ClientType) (ClientInterface, error) {
	switch clientType {
	case DeepInfraClient:
		return NewClient()
	case OllamaClient:
		return NewOllamaClient()
	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// GetClientTypeFromEnv determines which client to use based on environment variables
func GetClientTypeFromEnv() ClientType {
	// If DEEPINFRA_API_KEY is set, use DeepInfra
	if apiKey := os.Getenv("DEEPINFRA_API_KEY"); apiKey != "" {
		return DeepInfraClient
	}

	// Otherwise, default to Ollama for local inference
	return OllamaClient
}
