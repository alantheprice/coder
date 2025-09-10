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

// ShowColoredDiff displays a colored diff between old and new content (limited to first 50 lines)
func (a *Agent) ShowColoredDiff(oldContent, newContent string, maxLines int) {
	const red = "\033[31m"    // Red for deletions
	const green = "\033[32m"  // Green for additions
	const reset = "\033[0m"
	
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	// Simple line-by-line diff (not a full LCS implementation)
	maxOld := len(oldLines)
	maxNew := len(newLines)
	lineCount := 0
	
	fmt.Println("Diff preview (first 50 lines):")
	fmt.Println("----------------------------------------")
	
	i, j := 0, 0
	for i < maxOld && j < maxNew && lineCount < maxLines {
		if oldLines[i] == newLines[j] {
			// Lines are the same, show context
			fmt.Printf("  %s\n", oldLines[i])
			i++
			j++
		} else {
			// Lines differ, show deletion and addition
			fmt.Printf("%s- %s%s\n", red, oldLines[i], reset)
			fmt.Printf("%s+ %s%s\n", green, newLines[j], reset)
			i++
			j++
		}
		lineCount++
	}
	
	// Show remaining deletions
	for i < maxOld && lineCount < maxLines {
		fmt.Printf("%s- %s%s\n", red, oldLines[i], reset)
		i++
		lineCount++
	}
	
	// Show remaining additions
	for j < maxNew && lineCount < maxLines {
		fmt.Printf("%s+ %s%s\n", green, newLines[j], reset)
		j++
		lineCount++
	}
	
	if lineCount >= maxLines && (i < maxOld || j < maxNew) {
		fmt.Println("... (diff truncated)")
	}
	fmt.Println("----------------------------------------")
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
		// User specified a model, use current best provider
		clientType, _, err = configManager.GetBestProvider()
		if err != nil {
			return nil, fmt.Errorf("no available providers: %w", err)
		}
		finalModel = model
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

// SetModel changes the current model and persists the choice
func (a *Agent) SetModel(model string) error {
	// Update the client model
	if err := a.client.SetModel(model); err != nil {
		return fmt.Errorf("failed to set model on client: %w", err)
	}
	
	// Save to configuration
	if err := a.configManager.SetProviderAndModel(a.clientType, model); err != nil {
		return fmt.Errorf("failed to save model selection: %w", err)
	}
	
	return nil
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
	
	// Get input token pricing based on model
	if strings.Contains(model, "gpt-oss") {
		// GPT-OSS pricing: $0.30 per million input tokens
		costPerToken = 0.30 / 1000000
	} else if strings.Contains(model, "qwen3-coder") {
		// Qwen3-Coder-480B-A35B-Instruct-Turbo pricing: $0.30 per million input tokens
		costPerToken = 0.30 / 1000000
	} else if strings.Contains(model, "llama") {
		// Llama pricing: $0.36 per million tokens
		costPerToken = 0.36 / 1000000
	} else {
		// Default pricing (use GPT-OSS input rate)
		costPerToken = 0.30 / 1000000
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
	state := AgentState{
		Messages:       a.messages,
		PreviousSummary: a.previousSummary,
		TaskActions:    a.taskActions,
		SessionID:      a.sessionID,
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
