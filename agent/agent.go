package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/config"
	"github.com/alantheprice/coder/tools"
)

// TaskAction represents a completed action during task execution
type TaskAction struct {
	Type        string // "file_created", "file_modified", "command_executed", "file_read"
	Description string // Human-readable description
	Details     string // Additional details like file path, command, etc.
}

// AgentState represents the state of an agent that can be persisted
type AgentState struct {
	Messages        []api.Message `json:"messages"`
	PreviousSummary string        `json:"previous_summary"`
	CompactSummary  string        `json:"compact_summary"`  // New: 5K limit summary for continuity
	TaskActions     []TaskAction  `json:"task_actions"`
	SessionID       string        `json:"session_id"`
}

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
}

// debugLog logs a message only if debug mode is enabled
func (a *Agent) debugLog(format string, args ...interface{}) {
	if a.debug {
		fmt.Printf(format, args...)
	}
}

// getModelContextLimit returns the maximum context window for a model from the API
func (a *Agent) getModelContextLimit() int {
	limit, err := a.client.GetModelContextLimit()
	if err != nil {
		// Fallback to conservative default if API method fails
		if a.debug {
			a.debugLog("⚠️  Failed to get model context limit: %v, using default\n", err)
		}
		return 32000
	}
	return limit
}

// ToolLog logs tool execution messages that are always visible with blue formatting
func (a *Agent) ToolLog(action, target string) {
	const blue = "\033[34m"
	const reset = "\033[0m"
	
	// Format: [4:(15.2K/120K)] read file filename.go
	contextInfo := fmt.Sprintf("[%d:(%s/%s)]", 
		a.currentIteration, 
		a.formatTokenCount(a.currentContextTokens), 
		a.formatTokenCount(a.maxContextTokens))
	
	if target != "" {
		fmt.Printf("%s%s %s%s %s\n", blue, contextInfo, action, reset, target)
	} else {
		fmt.Printf("%s%s %s%s\n", blue, contextInfo, action, reset)
	}
}

// ShowColoredDiff displays a colored diff between old and new content, focusing on actual changes
func (a *Agent) ShowColoredDiff(oldContent, newContent string, maxLines int) {
	const red = "\033[31m"    // Red for deletions
	const green = "\033[32m"  // Green for additions
	const reset = "\033[0m"
	
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	// Find the actual changes by identifying differing regions
	changes := a.findChanges(oldLines, newLines)
	
	if len(changes) == 0 {
		fmt.Println("No changes detected")
		return
	}
	
	fmt.Printf("Diff preview (%d changes, up to %d lines):\n", len(changes), maxLines)
	fmt.Println("----------------------------------------")
	
	linesShown := 0
	
	for _, change := range changes {
		if linesShown >= maxLines {
			fmt.Printf("... (diff truncated at %d lines)\n", maxLines)
			break
		}
		
		// Show some context before the change
		contextStart := change.OldStart - 2
		if contextStart < 0 {
			contextStart = 0
		}
		
		// Show context lines
		for i := contextStart; i < change.OldStart && i < len(oldLines) && linesShown < maxLines; i++ {
			fmt.Printf("  %s\n", oldLines[i])
			linesShown++
		}
		
		// Show deletions
		for i := change.OldStart; i < change.OldStart+change.OldLength && i < len(oldLines) && linesShown < maxLines; i++ {
			fmt.Printf("%s- %s%s\n", red, oldLines[i], reset)
			linesShown++
		}
		
		// Show additions  
		for i := change.NewStart; i < change.NewStart+change.NewLength && i < len(newLines) && linesShown < maxLines; i++ {
			fmt.Printf("%s+ %s%s\n", green, newLines[i], reset)
			linesShown++
		}
		
		if linesShown >= maxLines {
			break
		}
	}
	
	fmt.Println("----------------------------------------")
}

// DiffChange represents a change region in the diff
type DiffChange struct {
	OldStart  int
	OldLength int
	NewStart  int
	NewLength int
}

// findChanges identifies regions where content differs between old and new versions
func (a *Agent) findChanges(oldLines, newLines []string) []DiffChange {
	var changes []DiffChange
	
	oldLen := len(oldLines)
	newLen := len(newLines)
	maxLen := oldLen
	if newLen > oldLen {
		maxLen = newLen
	}
	
	// Simple line-by-line comparison - much more reliable than complex algorithms
	i := 0
	for i < maxLen {
		oldLine := ""
		newLine := ""
		
		if i < oldLen {
			oldLine = oldLines[i]
		}
		if i < newLen {
			newLine = newLines[i]
		}
		
		// If lines are different, record the change
		if oldLine != newLine {
			changeStart := i
			changeLength := 1
			
			// Look ahead to group consecutive changes together
			for i+1 < maxLen {
				nextOld := ""
				nextNew := ""
				if i+1 < oldLen {
					nextOld = oldLines[i+1]
				}
				if i+1 < newLen {
					nextNew = newLines[i+1]
				}
				
				if nextOld != nextNew {
					changeLength++
					i++
				} else {
					break
				}
			}
			
			// Determine old and new lengths for this change
			oldChangeLen := changeLength
			newChangeLen := changeLength
			
			if changeStart+changeLength > oldLen {
				oldChangeLen = oldLen - changeStart
				if oldChangeLen < 0 {
					oldChangeLen = 0
				}
			}
			if changeStart+changeLength > newLen {
				newChangeLen = newLen - changeStart
				if newChangeLen < 0 {
					newChangeLen = 0
				}
			}
			
			changes = append(changes, DiffChange{
				OldStart:  changeStart,
				OldLength: oldChangeLen,
				NewStart:  changeStart,
				NewLength: newChangeLen,
			})
		}
		
		i++
	}
	
	return changes
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
		client:        client,
		messages:      []api.Message{},
		systemPrompt:  systemPrompt,
		maxIterations: 100,
		totalCost:     0.0,
		clientType:    clientType,
		debug:         debug,
		optimizer:     NewConversationOptimizer(optimizationEnabled, debug),
		configManager: configManager,
	}
	
	// Initialize context limits based on model
	agent.maxContextTokens = agent.getModelContextLimit()
	agent.currentContextTokens = 0
	agent.contextWarningIssued = false
	
	// Load previous conversation summary for continuity
	agent.loadPreviousSummary()
	
	return agent, nil
}

// loadPreviousSummary loads the previous conversation summary from the state file
func (a *Agent) loadPreviousSummary() {
	stateFile := ".coder_state.json"
	
	// Check if state file exists
	if _, err := os.Stat(stateFile); err == nil {
		// Load ONLY the summary, not the full conversation state
		if err := a.LoadSummaryFromFile(stateFile); err == nil {
			if a.debug {
				a.debugLog("📁 Loaded previous conversation summary from %s\n", stateFile)
			}
		} else {
			if a.debug {
				a.debugLog("⚠️  Failed to load conversation summary: %v\n", err)
			}
		}
	}
}

// SaveConversationSummary saves the conversation summary to the state file
func (a *Agent) SaveConversationSummary() error {
	// Generate summary before saving
	_ = a.GenerateConversationSummary() // Generate summary to update state
	
	// Save state to file
	stateFile := ".coder_state.json"
	if err := a.SaveStateToFile(stateFile); err != nil {
		return fmt.Errorf("failed to save conversation state: %v", err)
	}
	
	if a.debug {
		a.debugLog("💾 Saved conversation summary to %s\n", stateFile)
	}
	
	return nil
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

func (a *Agent) ProcessQuery(userQuery string) (string, error) {
	// Initialize with system prompt and user query
	a.messages = []api.Message{
		{Role: "system", Content: a.systemPrompt},
		{Role: "user", Content: userQuery},
	}

	a.currentIteration = 0

	for a.currentIteration < a.maxIterations {
		a.currentIteration++

		a.debugLog("Iteration %d/%d\n", a.currentIteration, a.maxIterations)

		// Optimize conversation before sending to API
		optimizedMessages := a.optimizer.OptimizeConversation(a.messages)
		
		if a.debug && len(optimizedMessages) < len(a.messages) {
			saved := len(a.messages) - len(optimizedMessages)
			a.debugLog("🔄 Conversation optimized: %d messages → %d messages (saved %d)\n", 
				len(a.messages), len(optimizedMessages), saved)
		}

		// Check context size and manage if approaching limit
		contextTokens := a.estimateContextTokens(optimizedMessages)
		a.currentContextTokens = contextTokens
		
		// Check if we're approaching the context limit (80%)
		contextThreshold := int(float64(a.maxContextTokens) * 0.8)
		if contextTokens > contextThreshold {
			if !a.contextWarningIssued {
				a.debugLog("⚠️  Context approaching limit: %s/%s (%.1f%%)\n", 
					a.formatTokenCount(contextTokens), 
					a.formatTokenCount(a.maxContextTokens),
					float64(contextTokens)/float64(a.maxContextTokens)*100)
				a.contextWarningIssued = true
			}
			
			// Perform aggressive optimization when near limit
			optimizedMessages = a.optimizer.AggressiveOptimization(optimizedMessages)
			contextTokens = a.estimateContextTokens(optimizedMessages)
			a.currentContextTokens = contextTokens
			
			if a.debug {
				a.debugLog("🔄 Aggressive optimization applied: %s context tokens\n", 
					a.formatTokenCount(contextTokens))
			}
		}

		// Send request to API using the unified interface
		resp, err := a.client.SendChatRequest(optimizedMessages, api.GetToolDefinitions(), "high")
		if err != nil {
			return "", fmt.Errorf("API request failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices returned")
		}

		// Track token usage and cost
		cachedTokens := resp.Usage.PromptTokensDetails.CachedTokens
		
		// Use actual cost from API (already accounts for cached tokens)
		a.totalCost += resp.Usage.EstimatedCost
		a.totalTokens += resp.Usage.TotalTokens
		a.promptTokens += resp.Usage.PromptTokens
		a.completionTokens += resp.Usage.CompletionTokens
		a.cachedTokens += cachedTokens
		
		// Calculate cost savings for display purposes only
		cachedCostSavings := a.calculateCachedCost(cachedTokens)
		a.cachedCostSavings += cachedCostSavings
		
		// Only show context information in debug mode
		if a.debug {
			a.debugLog("💰 Response: %d prompt + %d completion | Cost: $%.6f | Context: %s/%s\n",
				resp.Usage.PromptTokens,
				resp.Usage.CompletionTokens,
				resp.Usage.EstimatedCost,
				a.formatTokenCount(a.currentContextTokens),
				a.formatTokenCount(a.maxContextTokens))
			
			if cachedTokens > 0 {
				a.debugLog("📋 Cached tokens: %d | Savings: $%.6f\n",
					cachedTokens, cachedCostSavings)
			}
		}

		choice := resp.Choices[0]

		// Add assistant's message to history
		a.messages = append(a.messages, api.Message{
			Role:             "assistant",
			Content:          choice.Message.Content,
			ReasoningContent: choice.Message.ReasoningContent,
		})

		// Check if there are tool calls to execute
		if len(choice.Message.ToolCalls) > 0 {
			a.debugLog("Executing %d tool calls\n", len(choice.Message.ToolCalls))

			toolResults := make([]string, 0)
			for _, toolCall := range choice.Message.ToolCalls {
				result, err := a.executeTool(toolCall)
				if err != nil {
					result = fmt.Sprintf("Error executing tool %s: %s", toolCall.Function.Name, err.Error())
				}
				toolResults = append(toolResults, fmt.Sprintf("Tool: %s\nResult: %s", toolCall.Function.Name, result))

				// Add tool result as a message
				a.messages = append(a.messages, api.Message{
					Role:    "user",
					Content: fmt.Sprintf("Tool call result for %s: %s", toolCall.Function.Name, result),
				})
			}

			// Continue the loop to get next response
			continue
		} else {
			// Check if content or reasoning_content contains tool calls that weren't properly parsed
			toolCalls := a.extractToolCallsFromContent(choice.Message.Content)
			if len(toolCalls) == 0 {
				// Also check reasoning_content
				toolCalls = a.extractToolCallsFromContent(choice.Message.ReasoningContent)
			}

			if len(toolCalls) > 0 {
				a.debugLog("Detected %d tool calls in content/reasoning_content, executing them\n", len(toolCalls))

				toolResults := make([]string, 0)
				for _, toolCall := range toolCalls {
					result, err := a.executeTool(toolCall)
					if err != nil {
						result = fmt.Sprintf("Error executing tool %s: %s", toolCall.Function.Name, err.Error())
					}
					toolResults = append(toolResults, fmt.Sprintf("Tool: %s\nResult: %s", toolCall.Function.Name, result))

					// Add tool result as a message
					a.messages = append(a.messages, api.Message{
						Role:    "user",
						Content: fmt.Sprintf("Tool call result for %s: %s", toolCall.Function.Name, result),
					})
				}

				// Continue the loop to get next response
				continue
			}

			// No tool calls, check if we're done
			if choice.FinishReason == "stop" {
				// Check if content or reasoning_content contains malformed tool calls and remind the agent
				if a.containsMalformedToolCalls(choice.Message.Content) || a.containsMalformedToolCalls(choice.Message.ReasoningContent) {
					a.debugLog("⚠️  Detected malformed tool calls in response. Reminding agent to use proper tool call format...\n")

					// Add a reminder message to help the agent
					a.messages = append(a.messages, api.Message{
						Role:    "user",
						Content: "REMINDER: Please use proper tool call format with the 'tool_calls' field, not in the content or reasoning_content. Tool calls should be in JSON format like: {\"tool_calls\": [{\"id\": \"call_123\", \"type\": \"function\", \"function\": {\"name\": \"tool_name\", \"arguments\": \"{\\\"param\\\": \\\"value\\\"}\"}}]}",
					})
					continue
				}
				
				// Check if the response looks incomplete or if the agent is declining the task
				if a.isIncompleteResponse(choice.Message.Content) {
					a.debugLog("⚠️  Detected potentially incomplete response. Encouraging agent to continue...\n")
					
					// Add encouragement to continue
					a.messages = append(a.messages, api.Message{
						Role:    "user", 
						Content: "Please continue working on the task. You have all the tools needed to complete this request. Start by exploring the codebase systematically using shell_command, then read multiple files in parallel to reduce turns and save tokens. Group related file reads together in a single tool call batch.",
					})
					continue
				}
				
				return choice.Message.Content, nil
			}
		}
	}

	return "", fmt.Errorf("maximum iterations (%d) reached without completion", a.maxIterations)
}

func (a *Agent) executeTool(toolCall api.ToolCall) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Log the tool call for debugging
	a.debugLog("🔧 Executing tool: %s with args: %v\n", toolCall.Function.Name, args)
	
	// Validate tool name and provide helpful error for common mistakes
	validTools := []string{"shell_command", "read_file", "write_file", "edit_file", "add_todo", "update_todo_status", "list_todos", "add_bulk_todos", "auto_complete_todos", "get_next_todo", "list_all_todos", "get_active_todos_compact", "archive_completed", "update_todo_status_bulk"}
	isValidTool := false
	for _, valid := range validTools {
		if toolCall.Function.Name == valid {
			isValidTool = true
			break
		}
	}
	
	if !isValidTool {
		// Check for common misnamed tools and suggest corrections
		suggestion := a.suggestCorrectToolName(toolCall.Function.Name)
		if suggestion != "" {
			return "", fmt.Errorf("unknown tool '%s'. Did you mean '%s'? Valid tools are: %v", 
				toolCall.Function.Name, suggestion, validTools)
		}
		return "", fmt.Errorf("unknown tool '%s'. Valid tools are: %v", toolCall.Function.Name, validTools)
	}

	switch toolCall.Function.Name {
	case "shell_command":
		command, ok := args["command"].(string)
		if !ok {
			// Try alternative parameter name for backward compatibility
			command, ok = args["cmd"].(string)
			if !ok {
				return "", fmt.Errorf("invalid command argument")
			}
		}
		a.ToolLog("executing command", command)
		a.debugLog("Executing shell command: %s\n", command)
		result, err := tools.ExecuteShellCommand(command)
		a.debugLog("Shell command result: %s, error: %v\n", result, err)
		return result, err

	case "read_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			// Try alternative parameter name for backward compatibility
			filePath, ok = args["path"].(string)
			if !ok {
				return "", fmt.Errorf("invalid file_path argument")
			}
		}
		a.ToolLog("reading file", filePath)
		a.debugLog("Reading file: %s\n", filePath)
		result, err := tools.ReadFile(filePath)
		a.debugLog("Read file result: %s, error: %v\n", result, err)
		return result, err

	case "write_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			// Try alternative parameter name for backward compatibility
			filePath, ok = args["path"].(string)
			if !ok {
				return "", fmt.Errorf("invalid file_path argument")
			}
		}
		content, ok := args["content"].(string)
		if !ok {
			return "", fmt.Errorf("invalid content argument")
		}
		a.ToolLog("writing file", filePath)
		a.debugLog("Writing file: %s\n", filePath)
		result, err := tools.WriteFile(filePath, content)
		a.debugLog("Write file result: %s, error: %v\n", result, err)
		return result, err

	case "edit_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			// Try alternative parameter name for backward compatibility
			filePath, ok = args["path"].(string)
			if !ok {
				return "", fmt.Errorf("invalid file_path argument")
			}
		}
		oldString, ok := args["old_string"].(string)
		if !ok {
			return "", fmt.Errorf("invalid old_string argument")
		}
		newString, ok := args["new_string"].(string)
		if !ok {
			return "", fmt.Errorf("invalid new_string argument")
		}
		
		// Read the original content for diff display
		originalContent, err := tools.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read original file for diff: %w", err)
		}
		
		a.ToolLog("editing file", filePath)
		a.debugLog("Editing file: %s\n", filePath)
		result, err := tools.EditFile(filePath, oldString, newString)
		
		if err == nil {
			// Read the new content and show diff
			newContent, readErr := tools.ReadFile(filePath)
			if readErr == nil {
				a.ShowColoredDiff(originalContent, newContent, 50)
			}
		}
		
		a.debugLog("Edit file result: %s, error: %v\n", result, err)
		return result, err

	case "add_todo":
		title, ok := args["title"].(string)
		if !ok {
			return "", fmt.Errorf("invalid title argument")
		}
		description := ""
		if desc, ok := args["description"].(string); ok {
			description = desc
		}
		priority := ""
		if prio, ok := args["priority"].(string); ok {
			priority = prio
		}
		a.ToolLog("adding todo", title)
		a.debugLog("Adding todo: %s\n", title)
		result := tools.AddTodo(title, description, priority)
		a.debugLog("Add todo result: %s\n", result)
		return result, nil

	case "update_todo_status":
		id, ok := args["id"].(string)
		if !ok {
			return "", fmt.Errorf("invalid id argument")
		}
		status, ok := args["status"].(string)
		if !ok {
			return "", fmt.Errorf("invalid status argument")
		}
		// Show better ToolLog message based on status
		var logMessage string
		switch status {
		case "in_progress":
			// Extract the todo title for a better message
			todoTitle := ""
			for _, item := range tools.GetAllTodos() {
				if item.ID == id {
					todoTitle = item.Title
					break
				}
			}
			if todoTitle != "" {
				logMessage = fmt.Sprintf("starting %s", todoTitle)
			} else {
				logMessage = fmt.Sprintf("starting %s", id)
			}
		case "completed":
			// Extract the todo title for a better message
			todoTitle := ""
			for _, item := range tools.GetAllTodos() {
				if item.ID == id {
					todoTitle = item.Title
					break
				}
			}
			if todoTitle != "" {
				logMessage = fmt.Sprintf("completed %s", todoTitle)
			} else {
				logMessage = fmt.Sprintf("completed %s", id)
			}
		default:
			logMessage = fmt.Sprintf("%s -> %s", id, status)
		}
		a.ToolLog("todo update", logMessage)
		a.debugLog("Updating todo %s to %s\n", id, status)
		result := tools.UpdateTodoStatus(id, status)
		a.debugLog("Update todo result: %s\n", result)
		return result, nil

	case "list_todos":
		a.ToolLog("listing todos", "")
		a.debugLog("Listing todos\n")
		result := tools.ListTodos()
		a.debugLog("List todos result: %s\n", result)
		return result, nil

	case "add_bulk_todos":
		todosRaw, ok := args["todos"]
		if !ok {
			return "", fmt.Errorf("missing todos argument")
		}
		
		// Parse the todos array
		todosSlice, ok := todosRaw.([]interface{})
		if !ok {
			return "", fmt.Errorf("todos must be an array")
		}
		
		var todos []struct {
			Title       string
			Description string
			Priority    string
		}
		
		for _, todoRaw := range todosSlice {
			todoMap, ok := todoRaw.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("each todo must be an object")
			}
			
			todo := struct {
				Title       string
				Description string
				Priority    string
			}{}
			
			if title, ok := todoMap["title"].(string); ok {
				todo.Title = title
			}
			if desc, ok := todoMap["description"].(string); ok {
				todo.Description = desc
			}
			if prio, ok := todoMap["priority"].(string); ok {
				todo.Priority = prio
			}
			
			todos = append(todos, todo)
		}
		
		// Show the todo titles being created
		todoTitles := make([]string, len(todos))
		for i, todo := range todos {
			todoTitles[i] = todo.Title
		}
		if len(todoTitles) <= 3 {
			a.ToolLog("adding todos", strings.Join(todoTitles, ", "))
		} else {
			a.ToolLog("adding todos", fmt.Sprintf("%s, %s, +%d more", todoTitles[0], todoTitles[1], len(todoTitles)-2))
		}
		a.debugLog("Adding bulk todos: %d items\n", len(todos))
		result := tools.AddBulkTodos(todos)
		a.debugLog("Add bulk todos result: %s\n", result)
		return result, nil

	case "auto_complete_todos":
		context, ok := args["context"].(string)
		if !ok {
			return "", fmt.Errorf("invalid context argument")
		}
		a.ToolLog("auto completing todos", context)
		a.debugLog("Auto completing todos with context: %s\n", context)
		result := tools.AutoCompleteTodos(context)
		a.debugLog("Auto complete result: %s\n", result)
		return result, nil

	case "get_next_todo":
		a.ToolLog("getting next todo", "")
		a.debugLog("Getting next todo\n")
		result := tools.GetNextTodo()
		a.debugLog("Next todo result: %s\n", result)
		return result, nil

	case "list_all_todos":
		a.ToolLog("listing all todos", "full context")
		result := tools.ListAllTodos()
		return result, nil

	case "get_active_todos_compact":
		a.ToolLog("getting active todos", "compact")
		result := tools.GetActiveTodosCompact()
		return result, nil

	case "archive_completed":
		a.ToolLog("archiving completed", "")
		result := tools.ArchiveCompleted()
		return result, nil

	case "update_todo_status_bulk":
		updatesRaw, ok := args["updates"]
		if !ok {
			return "", fmt.Errorf("missing updates argument")
		}
		
		updatesSlice, ok := updatesRaw.([]interface{})
		if !ok {
			return "", fmt.Errorf("updates must be an array")
		}
		
		var updates []struct {
			ID     string
			Status string
		}
		
		for _, updateRaw := range updatesSlice {
			updateMap, ok := updateRaw.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("each update must be an object")
			}
			
			update := struct {
				ID     string
				Status string
			}{}
			
			if id, ok := updateMap["id"].(string); ok {
				update.ID = id
			}
			if status, ok := updateMap["status"].(string); ok {
				update.Status = status
			}
			
			updates = append(updates, update)
		}
		
		a.ToolLog("bulk status update", fmt.Sprintf("%d items", len(updates)))
		result := tools.UpdateTodoStatusBulk(updates)
		return result, nil

	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
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

func (a *Agent) PrintConversationSummary() {

	fmt.Println("\n📊 Conversation Summary")
	fmt.Println("══════════════════════════════")
	
	assistantMsgCount := 0
	userMsgCount := 0
	toolCallCount := 0

	for _, msg := range a.messages {
		switch msg.Role {
		case "assistant":
			assistantMsgCount++
			if strings.Contains(msg.Content, "tool_calls") {
				toolCallCount++
			}
		case "user":
			if msg.Content != a.messages[1].Content { // Skip original user query
				userMsgCount++
			}
		}
	}

	// Conversation metrics
	fmt.Printf("🔄 Iterations:      %d\n", a.currentIteration)
	fmt.Printf("🤖 Assistant msgs:   %d\n", assistantMsgCount)
	fmt.Printf("⚡ Tool executions:  %d\n", userMsgCount) // Tool results come back as user messages
	fmt.Printf("📨 Total messages:   %d\n", len(a.messages))
	fmt.Println()
	
	// Calculate actual processed tokens (excluding cached ones)
	actualProcessedTokens := a.totalTokens - a.cachedTokens
	
	// Token usage section with better formatting
	fmt.Println("🔢 Token Usage")
	fmt.Println("──────────────────────────────")
	fmt.Printf("📦 Total processed:    %s\n", a.formatTokenCount(a.totalTokens))
	fmt.Printf("📝 Actual processed:   %s (%d prompt + %d completion)\n", 
		a.formatTokenCount(actualProcessedTokens), a.promptTokens, a.completionTokens)
	
	// Context window information
	contextUsage := float64(a.currentContextTokens) / float64(a.maxContextTokens) * 100
	fmt.Printf("🪟 Context window:     %s/%s (%.1f%% used)\n", 
		a.formatTokenCount(a.currentContextTokens), 
		a.formatTokenCount(a.maxContextTokens), 
		contextUsage)
	
	if a.cachedTokens > 0 {
		efficiency := float64(a.cachedTokens)/float64(a.totalTokens)*100
		fmt.Printf("♻️  Cached reused:     %s\n", a.formatTokenCount(a.cachedTokens))
		fmt.Printf("💰 Cost savings:       $%.6f\n", a.cachedCostSavings)
		fmt.Printf("📈 Efficiency:        %.1f%% tokens cached\n", efficiency)
		
		// Add efficiency rating
		var efficiencyRating string
		switch {
		case efficiency >= 50:
			efficiencyRating = "🏆 Excellent"
		case efficiency >= 30:
			efficiencyRating = "✅ Good"
		case efficiency >= 15:
			efficiencyRating = "📊 Average"
		default:
			efficiencyRating = "📉 Low"
		}
		fmt.Printf("🏅 Efficiency rating: %s\n", efficiencyRating)
	}
	
	fmt.Println()
	fmt.Printf("💵 Total cost:        $%.6f\n", a.totalCost)
	
	// Add cost per iteration
	if a.currentIteration > 0 {
		costPerIteration := a.totalCost / float64(a.currentIteration)
		fmt.Printf("📋 Cost per iteration: $%.6f\n", costPerIteration)
	}
	
	// Show optimization stats if enabled
	if a.optimizer.IsEnabled() {
		stats := a.optimizer.GetOptimizationStats()
		fmt.Println()
		fmt.Println("🔄 Conversation Optimization")
		fmt.Println("──────────────────────────────")
		fmt.Printf("📁 Files tracked:     %d\n", stats["tracked_files"])
		fmt.Printf("⚡ Commands tracked:  %d\n", stats["tracked_commands"])
		
		if trackedFiles, ok := stats["file_paths"].([]string); ok && len(trackedFiles) > 0 {
			if len(trackedFiles) <= 3 {
				fmt.Printf("📂 Tracked files:     %s\n", strings.Join(trackedFiles, ", "))
			} else {
				fmt.Printf("📂 Tracked files:     %s, +%d more\n", 
					strings.Join(trackedFiles[:2], ", "), len(trackedFiles)-2)
			}
		}
		
		if trackedCommands, ok := stats["shell_commands"].([]string); ok && len(trackedCommands) > 0 {
			if len(trackedCommands) <= 3 {
				fmt.Printf("🔧 Tracked commands:  %s\n", strings.Join(trackedCommands, ", "))
			} else {
				fmt.Printf("🔧 Tracked commands:  %s, +%d more\n", 
					strings.Join(trackedCommands[:2], ", "), len(trackedCommands)-2)
			}
		}
	}
	
	fmt.Println("══════════════════════════════")
	fmt.Println()
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

// GetModel gets the current model being used by the agent
func (a *Agent) GetModel() string {
	// Use the interface method to get the model
	return a.client.GetModel()
}

// GetMessages returns the current conversation messages
func (a *Agent) GetMessages() []api.Message {
	return a.messages
}

// SetModel changes the current model and persists the choice
func (a *Agent) SetModel(model string) error {
	// Determine which provider this model belongs to
	requiredProvider, err := a.determineProviderForModel(model)
	if err != nil {
		return fmt.Errorf("failed to determine provider for model %s: %w", model, err)
	}
	
	// Check if we need to switch providers
	if requiredProvider != a.clientType {
		if a.debug {
			a.debugLog("🔄 Switching from %s to %s for model %s\n", 
				api.GetProviderName(a.clientType), api.GetProviderName(requiredProvider), model)
		}
		
		// Create a new client with the required provider
		newClient, err := api.NewUnifiedClientWithModel(requiredProvider, model)
		if err != nil {
			return fmt.Errorf("failed to create client for provider %s: %w", api.GetProviderName(requiredProvider), err)
		}
		
		// Set debug mode on the new client
		newClient.SetDebug(a.debug)
		
		// Check connection
		if err := newClient.CheckConnection(); err != nil {
			return fmt.Errorf("connection check failed for provider %s: %w", api.GetProviderName(requiredProvider), err)
		}
		
		// Switch to the new client
		a.client = newClient
		a.clientType = requiredProvider
	} else {
		// Same provider, just update the model
		if err := a.client.SetModel(model); err != nil {
			return fmt.Errorf("failed to set model on client: %w", err)
		}
	}
	
	// Save to configuration
	if err := a.configManager.SetProviderAndModel(requiredProvider, model); err != nil {
		return fmt.Errorf("failed to save model selection: %w", err)
	}
	
	return nil
}

// determineProviderForModel determines which provider a model belongs to by checking all available models
func (a *Agent) determineProviderForModel(modelID string) (api.ClientType, error) {
	// Get all available models from all providers
	allProviders := []api.ClientType{
		api.OpenRouterClientType,  // Check OpenRouter first as it has most models
		api.DeepInfraClientType,
		api.CerebrasClientType,
		api.GroqClientType,
		api.DeepSeekClientType,
		api.OllamaClientType,
	}
	
	if a.debug {
		a.debugLog("🔍 Determining provider for model: %s\n", modelID)
	}
	
	for _, provider := range allProviders {
		if a.debug {
			a.debugLog("🔍 Checking provider: %s\n", api.GetProviderName(provider))
		}
		
		// Check if this provider is available
		if !a.isProviderAvailable(provider) {
			if a.debug {
				a.debugLog("❌ Provider %s not available\n", api.GetProviderName(provider))
			}
			continue
		}
		
		if a.debug {
			a.debugLog("✅ Provider %s is available, checking models\n", api.GetProviderName(provider))
		}
		
		// Get models for this provider
		models, err := a.getModelsForProvider(provider)
		if err != nil {
			if a.debug {
				a.debugLog("❌ Failed to get models for %s: %v\n", api.GetProviderName(provider), err)
			}
			continue
		}
		
		if a.debug {
			a.debugLog("✅ Got %d models from %s\n", len(models), api.GetProviderName(provider))
		}
		
		// Check if this provider has the model
		for _, model := range models {
			if model.ID == modelID {
				if a.debug {
					a.debugLog("🎉 Found model %s in provider %s\n", modelID, api.GetProviderName(provider))
				}
				return provider, nil
			}
		}
		
		if a.debug {
			a.debugLog("❌ Model %s not found in provider %s\n", modelID, api.GetProviderName(provider))
		}
	}
	
	return "", fmt.Errorf("model %s not found in any available provider", modelID)
}

// getModelsForProvider gets models for a specific provider without environment manipulation
func (a *Agent) getModelsForProvider(provider api.ClientType) ([]api.ModelInfo, error) {
	// Check if provider is available first
	if !a.isProviderAvailable(provider) {
		return nil, fmt.Errorf("provider %s not available", api.GetProviderName(provider))
	}
	
	// For each provider, directly call the appropriate function based on current environment
	// This avoids the complexity of environment manipulation
	switch provider {
	case api.OpenRouterClientType:
		if os.Getenv("OPENROUTER_API_KEY") != "" {
			// Backup all other keys temporarily 
			deepinfraKey := os.Getenv("DEEPINFRA_API_KEY")
			cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
			groqKey := os.Getenv("GROQ_API_KEY")
			deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
			
			// Clear other keys temporarily
			os.Unsetenv("DEEPINFRA_API_KEY")
			os.Unsetenv("CEREBRAS_API_KEY")
			os.Unsetenv("GROQ_API_KEY")
			os.Unsetenv("DEEPSEEK_API_KEY")
			
			// Get OpenRouter models
			models, err := api.GetAvailableModels()
			
			// Restore other keys
			if deepinfraKey != "" {
				os.Setenv("DEEPINFRA_API_KEY", deepinfraKey)
			}
			if cerebrasKey != "" {
				os.Setenv("CEREBRAS_API_KEY", cerebrasKey)
			}
			if groqKey != "" {
				os.Setenv("GROQ_API_KEY", groqKey)
			}
			if deepseekKey != "" {
				os.Setenv("DEEPSEEK_API_KEY", deepseekKey)
			}
			
			return models, err
		}
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
		
	case api.DeepInfraClientType:
		if os.Getenv("DEEPINFRA_API_KEY") != "" {
			// Similar approach for DeepInfra
			openrouterKey := os.Getenv("OPENROUTER_API_KEY")
			cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
			groqKey := os.Getenv("GROQ_API_KEY")
			deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
			
			os.Unsetenv("OPENROUTER_API_KEY")
			os.Unsetenv("CEREBRAS_API_KEY")
			os.Unsetenv("GROQ_API_KEY")
			os.Unsetenv("DEEPSEEK_API_KEY")
			
			models, err := api.GetAvailableModels()
			
			if openrouterKey != "" {
				os.Setenv("OPENROUTER_API_KEY", openrouterKey)
			}
			if cerebrasKey != "" {
				os.Setenv("CEREBRAS_API_KEY", cerebrasKey)
			}
			if groqKey != "" {
				os.Setenv("GROQ_API_KEY", groqKey)
			}
			if deepseekKey != "" {
				os.Setenv("DEEPSEEK_API_KEY", deepseekKey)
			}
			
			return models, err
		}
		return nil, fmt.Errorf("DEEPINFRA_API_KEY not set")
		
	case api.CerebrasClientType:
		if os.Getenv("CEREBRAS_API_KEY") != "" {
			openrouterKey := os.Getenv("OPENROUTER_API_KEY")
			deepinfraKey := os.Getenv("DEEPINFRA_API_KEY")
			groqKey := os.Getenv("GROQ_API_KEY")
			deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
			
			os.Unsetenv("OPENROUTER_API_KEY")
			os.Unsetenv("DEEPINFRA_API_KEY")
			os.Unsetenv("GROQ_API_KEY")
			os.Unsetenv("DEEPSEEK_API_KEY")
			
			models, err := api.GetAvailableModels()
			
			if openrouterKey != "" {
				os.Setenv("OPENROUTER_API_KEY", openrouterKey)
			}
			if deepinfraKey != "" {
				os.Setenv("DEEPINFRA_API_KEY", deepinfraKey)
			}
			if groqKey != "" {
				os.Setenv("GROQ_API_KEY", groqKey)
			}
			if deepseekKey != "" {
				os.Setenv("DEEPSEEK_API_KEY", deepseekKey)
			}
			
			return models, err
		}
		return nil, fmt.Errorf("CEREBRAS_API_KEY not set")
		
	case api.GroqClientType:
		if os.Getenv("GROQ_API_KEY") != "" {
			openrouterKey := os.Getenv("OPENROUTER_API_KEY")
			deepinfraKey := os.Getenv("DEEPINFRA_API_KEY")
			cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
			deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
			
			os.Unsetenv("OPENROUTER_API_KEY")
			os.Unsetenv("DEEPINFRA_API_KEY")
			os.Unsetenv("CEREBRAS_API_KEY")
			os.Unsetenv("DEEPSEEK_API_KEY")
			
			models, err := api.GetAvailableModels()
			
			if openrouterKey != "" {
				os.Setenv("OPENROUTER_API_KEY", openrouterKey)
			}
			if deepinfraKey != "" {
				os.Setenv("DEEPINFRA_API_KEY", deepinfraKey)
			}
			if cerebrasKey != "" {
				os.Setenv("CEREBRAS_API_KEY", cerebrasKey)
			}
			if deepseekKey != "" {
				os.Setenv("DEEPSEEK_API_KEY", deepseekKey)
			}
			
			return models, err
		}
		return nil, fmt.Errorf("GROQ_API_KEY not set")
		
	case api.DeepSeekClientType:
		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			openrouterKey := os.Getenv("OPENROUTER_API_KEY")
			deepinfraKey := os.Getenv("DEEPINFRA_API_KEY")
			cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
			groqKey := os.Getenv("GROQ_API_KEY")
			
			os.Unsetenv("OPENROUTER_API_KEY")
			os.Unsetenv("DEEPINFRA_API_KEY")
			os.Unsetenv("CEREBRAS_API_KEY")
			os.Unsetenv("GROQ_API_KEY")
			
			models, err := api.GetAvailableModels()
			
			if openrouterKey != "" {
				os.Setenv("OPENROUTER_API_KEY", openrouterKey)
			}
			if deepinfraKey != "" {
				os.Setenv("DEEPINFRA_API_KEY", deepinfraKey)
			}
			if cerebrasKey != "" {
				os.Setenv("CEREBRAS_API_KEY", cerebrasKey)
			}
			if groqKey != "" {
				os.Setenv("GROQ_API_KEY", groqKey)
			}
			
			return models, err
		}
		return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
		
	case api.OllamaClientType:
		// For Ollama, we need to clear API keys to ensure it's selected
		openrouterKey := os.Getenv("OPENROUTER_API_KEY")
		deepinfraKey := os.Getenv("DEEPINFRA_API_KEY")
		cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
		groqKey := os.Getenv("GROQ_API_KEY")
		deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
		
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("DEEPINFRA_API_KEY")
		os.Unsetenv("CEREBRAS_API_KEY")
		os.Unsetenv("GROQ_API_KEY")
		os.Unsetenv("DEEPSEEK_API_KEY")
		
		models, err := api.GetAvailableModels()
		
		if openrouterKey != "" {
			os.Setenv("OPENROUTER_API_KEY", openrouterKey)
		}
		if deepinfraKey != "" {
			os.Setenv("DEEPINFRA_API_KEY", deepinfraKey)
		}
		if cerebrasKey != "" {
			os.Setenv("CEREBRAS_API_KEY", cerebrasKey)
		}
		if groqKey != "" {
			os.Setenv("GROQ_API_KEY", groqKey)
		}
		if deepseekKey != "" {
			os.Setenv("DEEPSEEK_API_KEY", deepseekKey)
		}
		
		return models, err
		
	default:
		return nil, fmt.Errorf("unknown provider type: %s", provider)
	}
}


// isProviderAvailable checks if a provider is currently available
func (a *Agent) isProviderAvailable(provider api.ClientType) bool {
	// For Ollama, check if it's running
	if provider == api.OllamaClientType {
		client, err := api.NewUnifiedClient(api.OllamaClientType)
		if err != nil {
			return false
		}
		return client.CheckConnection() == nil
	}
	
	// For other providers, check if API key is set
	envVar := a.getProviderEnvVar(provider)
	if envVar == "" {
		return false
	}
	
	return os.Getenv(envVar) != ""
}

// getProviderEnvVar returns the environment variable name for a provider
func (a *Agent) getProviderEnvVar(provider api.ClientType) string {
	switch provider {
	case api.DeepInfraClientType:
		return "DEEPINFRA_API_KEY"
	case api.CerebrasClientType:
		return "CEREBRAS_API_KEY"
	case api.OpenRouterClientType:
		return "OPENROUTER_API_KEY"
	case api.GroqClientType:
		return "GROQ_API_KEY"
	case api.DeepSeekClientType:
		return "DEEPSEEK_API_KEY"
	case api.OllamaClientType:
		return "" // Ollama doesn't use an API key
	default:
		return ""
	}
}

// GetProvider returns the current provider name
func (a *Agent) GetProvider() string {
	return a.client.GetProvider()
}

// GetProviderType returns the current provider type
func (a *Agent) GetProviderType() api.ClientType {
	return a.clientType
}

// GetConfigManager returns the configuration manager
func (a *Agent) GetConfigManager() *config.Manager {
	return a.configManager
}

// extractToolCallsFromContent attempts to parse tool calls from the assistant's content or reasoning_content
func (a *Agent) extractToolCallsFromContent(content string) []api.ToolCall {
	var toolCalls []api.ToolCall

	if content == "" {
		return toolCalls
	}

	// Look for tool_calls JSON structure in content
	if strings.Contains(content, "tool_calls") {
		// Try to extract and parse tool calls from content
		start := strings.Index(content, `{"tool_calls":`)
		if start != -1 {
			// Find the end of the JSON object
			end := strings.LastIndex(content[start:], "}")
			if end != -1 {
				jsonStr := content[start : start+end+1]

				var toolCallData struct {
					ToolCalls []api.ToolCall `json:"tool_calls"`
				}

				if err := json.Unmarshal([]byte(jsonStr), &toolCallData); err == nil {
					toolCalls = toolCallData.ToolCalls
				}
			}
		}
	}

	// Also check for alternative formats like {"cmd": ["bash", "-lc", "ls -R"]}
	if strings.Contains(content, `"cmd":`) {
		// Try to parse the cmd format
		var cmdData struct {
			Cmd []string `json:"cmd"`
		}

		if err := json.Unmarshal([]byte(content), &cmdData); err == nil && len(cmdData.Cmd) > 0 {
			// Convert cmd format to shell_command tool call
			command := strings.Join(cmdData.Cmd[1:], " ") // Skip the shell (e.g., "bash")
			if len(cmdData.Cmd) > 1 {
				command = strings.Join(cmdData.Cmd[1:], " ")
			}

			toolCall := api.ToolCall{
				ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "shell_command",
					Arguments: fmt.Sprintf(`{"command": "%s"}`, command),
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

// containsMalformedToolCalls checks if content contains tool call-like patterns that aren't properly formatted
func (a *Agent) containsMalformedToolCalls(content string) bool {
	if content == "" {
		return false
	}

	// Check for common patterns that indicate malformed tool calls
	patterns := []string{
		`{"tool_calls":`,
		`"function":`,
		`"arguments":`,
		`shell_command`,
		`read_file`,
		`write_file`,
		`edit_file`,
		`"cmd":`, // Also detect the cmd format
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return false
}

// isIncompleteResponse checks if a response looks incomplete or is declining the task prematurely
func (a *Agent) isIncompleteResponse(content string) bool {
	if content == "" {
		return true // Empty responses are definitely incomplete
	}
	
	content = strings.ToLower(content)
	
	// Common patterns that indicate the agent is giving up too early
	declinePatterns := []string{
		"i'm not able to",
		"i cannot",
		"i can't",
		"not possible to",
		"unable to",
		"can only work with",
		"cannot modify",
		"cannot add",
		"cannot create",
	}
	
	// If it's a short response with decline language, it's likely incomplete
	if len(content) < 200 {
		for _, pattern := range declinePatterns {
			if strings.Contains(content, pattern) {
				return true
			}
		}
	}
	
	// If there's no evidence of tool usage or exploration, likely incomplete
	toolEvidencePatterns := []string{
		"ls",
		"read",
		"write",
		"edit",
		"shell",
		"file",
		"directory",
		"explore",
		"implement",
		"create",
	}
	
	hasToolEvidence := false
	for _, pattern := range toolEvidencePatterns {
		if strings.Contains(content, pattern) {
			hasToolEvidence = true
			break
		}
	}
	
	// Short response without tool evidence suggests giving up early
	if len(content) < 300 && !hasToolEvidence {
		return true
	}
	
	return false
}

// suggestCorrectToolName suggests the correct tool name for common mistakes
func (a *Agent) suggestCorrectToolName(invalidName string) string {
	// Common tool name mappings
	corrections := map[string]string{
		"exec":         "shell_command",
		"bash":         "shell_command", 
		"cmd":          "shell_command",
		"command":      "shell_command",
		"run":          "shell_command",
		"execute":      "shell_command",
		"read":         "read_file",
		"cat":          "read_file",
		"open":         "read_file",
		"write":        "write_file",
		"save":         "write_file",
		"create":       "write_file",
		"edit":         "edit_file",
		"modify":       "edit_file",
		"change":       "edit_file",
		"replace":      "edit_file",
		"todo":         "add_todo",
		"task":         "add_todo",
		"update":       "update_todo_status",
		"status":       "update_todo_status",
		"list":         "list_todos",
		"show":         "list_todos",
	}
	
	if suggestion, exists := corrections[strings.ToLower(invalidName)]; exists {
		return suggestion
	}
	
	return ""
}

// estimateContextTokens estimates the token count for messages
func (a *Agent) estimateContextTokens(messages []api.Message) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		totalChars += len(msg.ReasoningContent)
	}
	// Rough estimate: 4 chars per token (conservative)
	return totalChars / 4
}

// formatTokenCount formats token count with thousands separators
// calculateCachedCost calculates the cost savings from cached tokens
func (a *Agent) calculateCachedCost(cachedTokens int) float64 {
	if cachedTokens == 0 {
		return 0.0
	}
	
	// Calculate cost savings based on model pricing (input token rate)
	costPerToken := 0.0
	model := a.GetModel()
	
	// Get input token pricing based on model and provider
	provider := a.GetProvider()
	
	// OpenRouter-specific pricing (updated January 2025)
	if provider == "openrouter" {
		if strings.Contains(model, "deepseek-chat") || strings.Contains(model, "deepseek-r1") {
			// DeepSeek models on OpenRouter: ~$0.55 per million input tokens
			costPerToken = 0.55 / 1000000
		} else if strings.Contains(model, "gpt-4o") {
			// GPT-4o on OpenRouter: $2.50 per million input tokens
			costPerToken = 2.50 / 1000000
		} else if strings.Contains(model, "gpt-4") {
			// GPT-4 on OpenRouter: $30 per million input tokens
			costPerToken = 30.0 / 1000000
		} else if strings.Contains(model, "claude-3.5-sonnet") {
			// Claude 3.5 Sonnet: $3.00 per million input tokens
			costPerToken = 3.00 / 1000000
		} else if strings.Contains(model, "claude-3-opus") {
			// Claude 3 Opus: $15.00 per million input tokens
			costPerToken = 15.0 / 1000000
		} else if strings.Contains(model, "claude-3-sonnet") {
			// Claude 3 Sonnet: $3.00 per million input tokens
			costPerToken = 3.00 / 1000000
		} else if strings.Contains(model, "claude-3-haiku") {
			// Claude 3 Haiku: $0.25 per million input tokens
			costPerToken = 0.25 / 1000000
		} else if strings.Contains(model, "llama-3.1-405b") {
			// Llama 3.1 405B: ~$5.00 per million input tokens
			costPerToken = 5.0 / 1000000
		} else if strings.Contains(model, "llama-3.1-70b") {
			// Llama 3.1 70B: ~$0.88 per million input tokens
			costPerToken = 0.88 / 1000000
		} else if strings.Contains(model, "llama-3.1-8b") {
			// Llama 3.1 8B: ~$0.18 per million input tokens
			costPerToken = 0.18 / 1000000
		} else {
			// Default OpenRouter pricing (use DeepSeek rate as conservative estimate)
			costPerToken = 0.55 / 1000000
		}
	} else if strings.Contains(model, "gpt-oss") {
		// GPT-OSS pricing: $0.30 per million input tokens
		costPerToken = 0.30 / 1000000
	} else if strings.Contains(model, "qwen3-coder") {
		// Qwen3-Coder-480B-A35B-Instruct-Turbo pricing: $0.30 per million input tokens
		costPerToken = 0.30 / 1000000
	} else if strings.Contains(model, "llama") {
		// Llama pricing: $0.36 per million tokens
		costPerToken = 0.36 / 1000000
	} else {
		// Default pricing (conservative estimate)
		costPerToken = 1.0 / 1000000
	}
	
	costSavings := float64(cachedTokens) * costPerToken
	
	return costSavings
}

func (a *Agent) formatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	}
	
	// Convert to thousands format with one decimal place
	thousands := float64(tokens) / 1000.0
	return fmt.Sprintf("%.1fK", thousands)
}

// ClearConversationHistory clears the conversation history
func (a *Agent) ClearConversationHistory() {
	a.messages = []api.Message{}
	a.previousSummary = ""
	a.taskActions = []TaskAction{}
	a.optimizer.Reset()
}

// SetConversationOptimization enables or disables conversation optimization
// Note: Optimization is always enabled by default for optimal performance
func (a *Agent) SetConversationOptimization(enabled bool) {
	a.optimizer.SetEnabled(enabled)
	if a.debug {
		if enabled {
			a.debugLog("🔄 Conversation optimization enabled\n")
		} else {
			a.debugLog("🔄 Conversation optimization disabled\n")
		}
	}
}

// GetOptimizationStats returns conversation optimization statistics
func (a *Agent) GetOptimizationStats() map[string]interface{} {
	return a.optimizer.GetOptimizationStats()
}

// ExportState exports the current agent state for persistence
func (a *Agent) ExportState() ([]byte, error) {
	// Generate compact summary for next session continuity
	compactSummary := a.GenerateCompactSummary()
	
	state := AgentState{
		Messages:        a.messages,
		PreviousSummary: a.previousSummary,
		CompactSummary:  compactSummary,  // Store 5K-limited summary for continuity
		TaskActions:     a.taskActions,
		SessionID:       a.sessionID,
	}
	return json.Marshal(state)
}

// ImportState imports agent state from JSON data
func (a *Agent) ImportState(data []byte) error {
	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	a.messages = state.Messages
	a.previousSummary = state.PreviousSummary
	a.taskActions = state.TaskActions
	a.sessionID = state.SessionID
	return nil
}

// SaveStateToFile saves the agent state to a file
func (a *Agent) SaveStateToFile(filename string) error {
	stateData, err := a.ExportState()
	if err != nil {
		return err
	}
	return os.WriteFile(filename, stateData, 0644)
}

// LoadStateFromFile loads agent state from a file
func (a *Agent) LoadStateFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return a.ImportState(data)
}

// LoadSummaryFromFile loads ONLY the compact summary from a state file for minimal continuity
func (a *Agent) LoadSummaryFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	
	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	// Only load the compact summary, not the full conversation state
	if state.CompactSummary != "" {
		a.previousSummary = state.CompactSummary
		if a.debug {
			a.debugLog("📄 Loaded compact summary (%d chars)\n", len(state.CompactSummary))
		}
	} else if state.PreviousSummary != "" {
		// Fallback to legacy summary if compact summary not available
		a.previousSummary = state.PreviousSummary
		if a.debug {
			a.debugLog("📄 Loaded legacy summary (%d chars)\n", len(state.PreviousSummary))
		}
	}
	
	return nil
}



// GenerateActionSummary creates a summary of completed actions for continuity
func (a *Agent) GenerateActionSummary() string {
	if len(a.taskActions) == 0 {
		return "No actions completed yet."
	}
	
	var summary strings.Builder
	summary.WriteString("Previous actions completed:\n")
	
	for i, action := range a.taskActions {
		summary.WriteString(fmt.Sprintf("%d. %s: %s", i+1, action.Type, action.Description))
		if action.Details != "" {
			summary.WriteString(fmt.Sprintf(" (%s)", action.Details))
		}
		summary.WriteString("\n")
	}
	
	return summary.String()
}

// GenerateConversationSummary creates a comprehensive summary of the conversation including todos
func (a *Agent) GenerateConversationSummary() string {
	var summary strings.Builder
	
	// Add conversation metrics
	summary.WriteString("📊 CONVERSATION SUMMARY\n")
	summary.WriteString("══════════════════════════════\n\n")
	
	// Add task actions summary
	if len(a.taskActions) > 0 {
		summary.WriteString("🎯 COMPLETED ACTIONS:\n")
		summary.WriteString("──────────────────────────────\n")
		
		// Group actions by type
		actionCounts := make(map[string]int)
		for _, action := range a.taskActions {
			actionCounts[action.Type]++
		}
		
		for actionType, count := range actionCounts {
			summary.WriteString(fmt.Sprintf("• %s: %d actions\n", actionType, count))
		}
		summary.WriteString("\n")
	}
	
	// Add todo summary
	todoSummary := tools.GetTaskSummary()
	if todoSummary != "No tasks tracked in this session." {
		summary.WriteString("📋 TASK PROGRESS:\n")
		summary.WriteString("──────────────────────────────\n")
		summary.WriteString(todoSummary)
		summary.WriteString("\n")
	}
	
	// Add key files explored
	stats := a.optimizer.GetOptimizationStats()
	if trackedFiles, ok := stats["file_paths"].([]string); ok && len(trackedFiles) > 0 {
		summary.WriteString("📂 KEY FILES EXPLORED:\n")
		summary.WriteString("──────────────────────────────\n")
		for _, file := range trackedFiles {
			summary.WriteString(fmt.Sprintf("• %s\n", file))
		}
		summary.WriteString("\n")
	}
	
	// Add conversation metrics
	summary.WriteString("📈 CONVERSATION METRICS:\n")
	summary.WriteString("──────────────────────────────\n")
	summary.WriteString(fmt.Sprintf("• Iterations: %d\n", a.currentIteration))
	summary.WriteString(fmt.Sprintf("• Total cost: $%.6f\n", a.totalCost))
	summary.WriteString(fmt.Sprintf("• Total tokens: %s\n", a.formatTokenCount(a.totalTokens)))
	
	if a.cachedTokens > 0 {
		efficiency := float64(a.cachedTokens)/float64(a.totalTokens)*100
		summary.WriteString(fmt.Sprintf("• Efficiency: %.1f%% tokens cached\n", efficiency))
	}
	
	summary.WriteString("══════════════════════════════\n")
	
	return summary.String()
}

// GenerateCompactSummary creates a compact summary for session continuity (max 5K context)
func (a *Agent) GenerateCompactSummary() string {
	var summary strings.Builder
	
	// Start with a session continuity header
	summary.WriteString("🔄 PREVIOUS SESSION CONTEXT\n")
	summary.WriteString("════════════════════════════\n\n")
	
	// Add accomplished todos with context
	todoSummary := tools.GetTaskSummary()
	if todoSummary != "No tasks tracked in this session." {
		summary.WriteString("✅ ACCOMPLISHED TASKS:\n")
		summary.WriteString("─────────────────────────────\n")
		
		// Get completed tasks with more detail
		completedTasks := tools.GetCompletedTasks()
		if len(completedTasks) > 0 {
			for i, task := range completedTasks {
				if i >= 8 { // Limit to 8 tasks to control size
					summary.WriteString("  ... and more\n")
					break
				}
				summary.WriteString(fmt.Sprintf("• %s\n", task))
			}
		} else {
			// Fallback to basic summary if detailed tasks not available
			lines := strings.Split(todoSummary, "\n")
			for _, line := range lines {
				if strings.Contains(line, "completed") || strings.Contains(line, "✅") {
					summary.WriteString(fmt.Sprintf("• %s\n", strings.TrimSpace(line)))
				}
			}
		}
		summary.WriteString("\n")
	}
	
	// Add key technical changes (limited and focused)
	if len(a.taskActions) > 0 {
		summary.WriteString("🔧 KEY TECHNICAL CHANGES:\n")
		summary.WriteString("─────────────────────────────\n")
		
		// Focus on the most important actions, limit to save space
		importantActions := []string{}
		for _, action := range a.taskActions {
			if action.Type == "file_modified" || action.Type == "file_created" {
				importantActions = append(importantActions, 
					fmt.Sprintf("• %s: %s", action.Type, action.Description))
			}
		}
		
		// Limit to most recent 6 actions
		start := 0
		if len(importantActions) > 6 {
			start = len(importantActions) - 6
			summary.WriteString("  [Recent changes shown]\n")
		}
		
		for i := start; i < len(importantActions); i++ {
			summary.WriteString(importantActions[i] + "\n")
		}
		summary.WriteString("\n")
	}
	
	// Add key files touched (limited list)
	stats := a.optimizer.GetOptimizationStats()
	if trackedFiles, ok := stats["file_paths"].([]string); ok && len(trackedFiles) > 0 {
		summary.WriteString("📄 KEY FILES:\n")
		summary.WriteString("─────────────────────────────\n")
		
		// Limit to 8 files to control summary size
		count := len(trackedFiles)
		if count > 8 {
			count = 8
		}
		
		for i := 0; i < count; i++ {
			summary.WriteString(fmt.Sprintf("• %s\n", trackedFiles[i]))
		}
		
		if len(trackedFiles) > 8 {
			summary.WriteString(fmt.Sprintf("  ... and %d more files\n", len(trackedFiles)-8))
		}
		summary.WriteString("\n")
	}
	
	// Add concise session metrics
	summary.WriteString("📊 SESSION METRICS:\n")
	summary.WriteString("─────────────────────────────\n")
	summary.WriteString(fmt.Sprintf("• Cost: $%.4f", a.totalCost))
	if a.cachedTokens > 0 {
		efficiency := float64(a.cachedTokens)/float64(a.totalTokens)*100
		summary.WriteString(fmt.Sprintf(" (%.0f%% cached)", efficiency))
	}
	summary.WriteString("\n")
	
	summary.WriteString("════════════════════════════\n")
	
	// Ensure summary is under 5K characters
	result := summary.String()
	if len(result) > 5000 {
		// Truncate and add indicator
		result = result[:4950] + "...\n[Summary truncated to 5K limit]\n════════════════════════════\n"
	}
	
	return result
}

// AddTaskAction records a completed task action for continuity
func (a *Agent) AddTaskAction(actionType, description, details string) {
	a.taskActions = append(a.taskActions, TaskAction{
		Type:        actionType,
		Description: description,
		Details:     details,
	})
}

// SetPreviousSummary sets the summary of previous actions for continuity
func (a *Agent) SetPreviousSummary(summary string) {
	a.previousSummary = summary
}

// GetPreviousSummary returns the summary of previous actions
func (a *Agent) GetPreviousSummary() string {
	return a.previousSummary
}

// SetSessionID sets the session identifier for continuity
func (a *Agent) SetSessionID(sessionID string) {
	a.sessionID = sessionID
}

// GetSessionID returns the session identifier
func (a *Agent) GetSessionID() string {
	return a.sessionID
}

// ProcessQueryWithContinuity processes a query with continuity from previous actions
func (a *Agent) ProcessQueryWithContinuity(userQuery string) (string, error) {
	// Load previous state if available
	if a.previousSummary != "" {
		continuityPrompt := fmt.Sprintf(`
CONTINUITY FROM PREVIOUS SESSION:
%s

CURRENT TASK:
%s

Please continue working on this task chain, building upon the previous actions.`, 
			a.previousSummary, userQuery)
		
		return a.ProcessQuery(continuityPrompt)
	}
	
	// No previous state, process normally
	return a.ProcessQuery(userQuery)
}
