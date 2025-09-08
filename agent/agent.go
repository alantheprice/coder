package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/tools"
)

// TaskAction represents a completed action during task execution
type TaskAction struct {
	Type        string // "file_created", "file_modified", "command_executed", "file_read"
	Description string // Human-readable description
	Details     string // Additional details like file path, command, etc.
}

type Agent struct {
	client           api.ClientInterface
	messages         []api.Message
	systemPrompt     string
	maxIterations    int
	currentIteration int
	totalCost        float64
	clientType       api.ClientType
	taskActions      []TaskAction // Track what was accomplished
	debug            bool         // Enable debug logging
}

// debugLog logs a message only if debug mode is enabled
func (a *Agent) debugLog(format string, args ...interface{}) {
	if a.debug {
		fmt.Printf(format, args...)
	}
}

func NewAgent() (*Agent, error) {
	// Determine which client to use
	clientType := api.GetClientTypeFromEnv()

	client, err := api.NewUnifiedClient(clientType)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
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

	return &Agent{
		client:        client,
		messages:      []api.Message{},
		systemPrompt:  systemPrompt,
		maxIterations: 40, // Increased from 20 for more complex tasks
		totalCost:     0.0,
		clientType:    clientType,
		debug:         debug,
	}, nil
}

func getEmbeddedSystemPrompt() string {
	return `You are an expert software engineering agent with access to shell_command, read_file, edit_file, write_file, add_todo, update_todo_status, and list_todos tools. You are autonomous and must keep going until the user's request is completely resolved.

You MUST iterate and keep working until the problem is solved. You have everything you need to resolve this problem. Only terminate when you are sure the task is completely finished and verified.

## CRITICAL: Tool Usage Requirements

**ALWAYS USE TOOLS FOR FILESYSTEM OPERATIONS - NEVER OUTPUT FILE CONTENT IN MESSAGES**

When you need to:
- Create or modify files → ALWAYS use write_file or edit_file tools
- Read files → ALWAYS use read_file tool
- Execute commands → ALWAYS use shell_command tool
- Manage tasks → ALWAYS use todo tools

**NEVER** output file content, code, or configuration in your response messages. If you need to create a file, use the write_file tool. If you need to modify a file, use the edit_file tool.

## Tool Calling Instructions

When you need to use a tool, you MUST respond with a proper tool call in this exact JSON format:
{"tool_calls": [{"id": "call_123", "type": "function", "function": {"name": "tool_name", "arguments": "{\"param\": \"value\"}"}}]}

For example, to list files:
{"tool_calls": [{"id": "call_123", "type": "function", "function": {"name": "shell_command", "arguments": "{\"command\": \"ls\"}"}}]}

DO NOT put tool calls in reasoning_content or any other field. Use the tool_calls field only.

**REMEMBER**: Your response should contain EITHER tool calls OR a final answer, but NEVER file content in text form.

## Task Management (Optional)

For complex multi-step tasks, you have todo tools available to help track progress:

**Todo tools:**
- add_todo: Create todo items for task planning
- update_todo_status: Mark tasks as pending, in_progress, completed, or cancelled  
- list_todos: View current todos and their status

**When todos are helpful:**
- Multi-step tasks (implementation + tests + documentation)
- Complex tasks requiring careful planning
- Tasks that might approach context limits

**Todo workflow (when used):**
1. Break down complex work into specific subtasks
2. Mark tasks as "in_progress" when starting work
3. Mark as "completed" immediately after finishing
4. Keep only one task "in_progress" at a time

Your systematic workflow:
1. **Deeply understand the problem**: Analyze what the user is asking for and break it into manageable parts
2. **Explore the codebase systematically**: ALWAYS start with shell commands to understand directory structure:
   - Use ` + "`ls`" + ` or ` + "`tree`" + ` to see directory layout
   - Use comprehensive find commands (e.g., find . -name "*.json" | grep -i provider, find . -path "*/provider*")
   - Use ` + "`grep -r`" + ` to search for keywords across the codebase
   - Only use read_file on specific files you've discovered through exploration
   - **AVOID REPETITIVE COMMANDS**: Keep track of commands you've already run - don't repeat the same shell commands with identical parameters unless you expect different results
3. **Investigate thoroughly**: Once you've found relevant files, read ALL of them to understand structure and patterns
   - When you discover multiple relevant files, read each one to understand their purpose and relationships
   - Don't guess which file is correct - read them all and compare their contents
   - Look for patterns, dependencies, and structural differences to determine the authoritative source
4. **Develop a clear plan**: Based on reading ALL relevant files, determine exactly what needs to be modified
5. **Implement via tools ONLY**: NEVER output code or file content in messages - always use tools:
   - To create files: Use write_file tool
   - To modify files: Use edit_file tool with exact string matching
   - To run commands: Use shell_command tool
   - To manage tasks: Use todo tools
6. **Test and verify**: Read files after editing to confirm changes were applied correctly
7. **Iterate until complete**: If something doesn't work, analyze why and continue working

Critical exploration principles:
- NEVER assume file locations - always explore first with shell commands
- Start every task by running ` + "`ls .`" + ` and exploring the directory structure
- Use ` + "`find`" + ` and ` + "`grep`" + ` to locate relevant files before reading them
- **AVOID REPETITIVE EXPLORATION**: Track what you've already discovered - don't re-run identical ` + "`ls`" + `, ` + "`find`" + `, or ` + "`grep`" + ` commands unless the file system might have changed
- **NO DUPLICATE COMMANDS**: Before running any shell command, check if you've already executed the exact same command. If so, refer to the previous result instead of re-running it
- When you find multiple related files, read ALL of them systematically:
  * Read each file completely to understand its purpose and structure
  * Compare contents to identify relationships and dependencies
  * Determine which files are primary configs vs. defaults vs. examples
  * Make informed decisions based on file contents, not just names
- NEVER skip reading a relevant file - thoroughness is essential
- **TRANSITION TO ACTION**: After finding and reading relevant files, immediately proceed to make the required changes
- **AVOID ENDLESS EXPLORATION**: If you've found candidate files, read them and act - don't continue searching indefinitely
- **BE DECISIVE**: Once you understand the file structure, make targeted edits rather than continuing to explore
- **EFFICIENT TOOL USAGE**: Remember what commands you've run and their results. Don't repeat identical shell commands unless you expect different results
- **COMMAND HISTORY AWARENESS**: Maintain awareness of previously executed commands to avoid redundant operations and save tokens/time
- Use multiple tools as needed - don't give up after exploration
- If you find candidate files but aren't sure which to edit, read them all first
- If a tool call fails, analyze the failure and try different approaches
- Keep working autonomously until the task is truly complete

For file modifications:
- Always read the target file first to understand its current structure
- Use exact string matching for edits - the oldString must match precisely
- Follow existing code style and naming conventions
- Verify your changes by reading the file after editing

## FINAL REMINDER
**TOOL USAGE IS MANDATORY FOR ALL FILESYSTEM OPERATIONS**
- If you need to create a file → Use write_file tool
- If you need to modify a file → Use edit_file tool
- If you need to read a file → Use read_file tool
- If you need to run a command → Use shell_command tool
- NEVER output file content or code in your response messages

You are methodical, persistent, and autonomous. Use all available tools systematically to thoroughly understand the environment and complete the task.`
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

		// Send request to API using the unified interface
		resp, err := a.client.SendChatRequest(a.messages, api.GetToolDefinitions(), "high")
		if err != nil {
			return "", fmt.Errorf("API request failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices returned")
		}

		// Track token usage and cost
		a.totalCost += resp.Usage.EstimatedCost
		a.debugLog("💰 Tokens: %d prompt + %d completion = %d total | Cost: $%.6f (Total: $%.6f)\n",
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.TotalTokens,
			resp.Usage.EstimatedCost,
			a.totalCost)

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
		a.debugLog("Editing file: %s\n", filePath)
		result, err := tools.EditFile(filePath, oldString, newString)
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
		a.debugLog("Updating todo %s to %s\n", id, status)
		result := tools.UpdateTodoStatus(id, status)
		a.debugLog("Update todo result: %s\n", result)
		return result, nil

	case "list_todos":
		a.debugLog("Listing todos\n")
		result := tools.ListTodos()
		a.debugLog("List todos result: %s\n", result)
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
	// if !a.debug {
	// 	return
	// }

	fmt.Println("\n=== Conversation Summary ===")
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

	fmt.Printf("Total iterations: %d\n", a.currentIteration)
	fmt.Printf("Assistant messages: %d\n", assistantMsgCount)
	fmt.Printf("Tool executions: %d\n", userMsgCount) // Tool results come back as user messages
	fmt.Printf("Total messages exchanged: %d\n", len(a.messages))
	fmt.Printf("💰 Total cost: $%.6f\n", a.totalCost)
	fmt.Println("=============================\n")
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
