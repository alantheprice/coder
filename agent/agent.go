package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/config"
	"github.com/alantheprice/coder/tools"
)


type Agent struct {
	client                api.ClientInterface
	messages              []api.Message
	systemPrompt          string
	maxIterations         int
	currentIteration      int
	totalCost             float64
	clientType            api.ClientType
	taskActions           []TaskAction // Track what was accomplished
	debug                 bool         // Enable debug logging
	totalTokens           int          // Track total tokens used across all requests
	promptTokens          int          // Track total prompt tokens
	completionTokens      int          // Track total completion tokens
	cachedTokens          int          // Track tokens that were cached/reused
	cachedCostSavings     float64      // Track cost savings from cached tokens
	previousSummary       string       // Summary of previous actions for continuity
	sessionID             string       // Unique session identifier
	optimizer             *ConversationOptimizer // Conversation optimization
	configManager         *config.Manager        // Configuration management
	currentContextTokens  int          // Current context size being sent to model
	maxContextTokens      int          // Model's maximum context window
	contextWarningIssued  bool         // Whether we've warned about approaching context limit
	shellCommandHistory   map[string]*ShellCommandResult // Track shell commands for deduplication
}




func NewAgent() (*Agent, error) {
	return NewAgentWithModel("")
}

func NewAgentWithModel(model string) (*Agent, error) {
	// Initialize configuration manager
	configManager, err := config.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}

	// Determine best provider and model
	var clientType api.ClientType
	var finalModel string
	
	if model != "" {
		finalModel = model
		// When a model is specified, use the best available provider
		// The provider should be explicitly set via command line --provider flag
		// or via interactive /provider selection before this point
		clientType, _, _ = configManager.GetBestProvider()
	} else {
		// Use configured provider and model
		clientType, finalModel, err = configManager.GetBestProvider()
		if err != nil {
			return nil, fmt.Errorf("no available providers: %w", err)
		}
	}

	// Create the client
	client, err := api.NewUnifiedClientWithModel(clientType, finalModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Save the selection for future use
	if err := configManager.SetProviderAndModel(clientType, finalModel); err != nil {
		// Log warning but don't fail - this is not critical
		fmt.Printf("⚠️  Warning: Failed to save provider selection: %v\n", err)
	}

	// Check if debug mode is enabled
	debug := os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1"

	// Set debug mode on the client
	client.SetDebug(debug)

	// Check connection
	if err := client.CheckConnection(); err != nil {
		return nil, fmt.Errorf("client connection check failed: %w", err)
	}

	// Use embedded system prompt
	systemPrompt := getEmbeddedSystemPrompt()

	// Clear old todos at session start
	tools.ClearTodos()

	// Conversation optimization is always enabled
	optimizationEnabled := true

	agent := &Agent{
		client:              client,
		messages:            []api.Message{},
		systemPrompt:        systemPrompt,
		maxIterations:       100,
		totalCost:           0.0,
		clientType:          clientType,
		debug:               debug,
		optimizer:           NewConversationOptimizer(optimizationEnabled, debug),
		configManager:       configManager,
		shellCommandHistory: make(map[string]*ShellCommandResult),
	}
	
	// Initialize context limits based on model
	agent.maxContextTokens = agent.getModelContextLimit()
	agent.currentContextTokens = 0
	agent.contextWarningIssued = false
	
	// Load previous conversation summary for continuity
	agent.loadPreviousSummary()
	
	return agent, nil
}




func getProjectContext() string {
	// Check for project context files in order of priority
	contextFiles := []string{
		".cursor/markdown/project.md",
		".cursor/markdown/context.md", 
		".claude/project.md",
		".claude/context.md",
		".project_context.md",
		"PROJECT_CONTEXT.md",
	}
	
	for _, filePath := range contextFiles {
		content, err := tools.ReadFile(filePath)
		if err == nil && strings.TrimSpace(content) != "" {
			return fmt.Sprintf("PROJECT CONTEXT:\n%s", content)
		}
	}
	
	return ""
}

// Basic getter methods
func (a *Agent) GetConfigManager() *config.Manager {
	return a.configManager
}

func (a *Agent) GetTotalCost() float64 {
	return a.totalCost
}

func (a *Agent) GetCurrentIteration() int {
	return a.currentIteration
}

func (a *Agent) GetMaxIterations() int {
	return a.maxIterations
}

func (a *Agent) GetMessages() []api.Message {
	return a.messages
}

func (a *Agent) GetConversationHistory() []api.Message {
	return a.messages
}

func (a *Agent) GetLastAssistantMessage() string {
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == "assistant" {
			return a.messages[i].Content
		}
	}
	return ""
}

