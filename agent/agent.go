package agent

import (
	"encoding/json"
	"fmt"
	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/tools"
	"strings"
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
}

func NewAgent() (*Agent, error) {
	// Determine which client to use
	clientType := api.GetClientTypeFromEnv()

	client, err := api.NewUnifiedClient(clientType)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

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
	}, nil
}

func getEmbeddedSystemPrompt() string {
	return `You are an expert software engineering agent with access to shell_command, read_file, edit_file, write_file, add_todo, update_todo_status, and list_todos tools. You are autonomous and must keep going until the user's request is completely resolved.

You MUST iterate and keep working until the problem is solved. You have everything you need to resolve this problem. Only terminate when you are sure the task is completely finished and verified.

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
3. **Explore the codebase systematically**: ALWAYS start with shell commands to understand directory structure:
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
5. **Implement incrementally**: Make precise changes using edit_file with exact string matching
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

		fmt.Printf("Iteration %d/%d\n", a.currentIteration, a.maxIterations)

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
		fmt.Printf("💰 Tokens: %d prompt + %d completion = %d total | Cost: $%.6f (Total: $%.6f)\n",
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.TotalTokens,
			resp.Usage.EstimatedCost,
			a.totalCost)

		choice := resp.Choices[0]

		// Add assistant's message to history
		a.messages = append(a.messages, api.Message{
			Role:    "assistant",
			Content: choice.Message.Content,
		})

		// Check if there are tool calls to execute
		if len(choice.Message.ToolCalls) > 0 {
			fmt.Printf("Executing %d tool calls\n", len(choice.Message.ToolCalls))

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
			// No tool calls, check if we're done
			if choice.FinishReason == "stop" {
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

	switch toolCall.Function.Name {
	case "shell_command":
		command, ok := args["command"].(string)
		if !ok {
			return "", fmt.Errorf("invalid command argument")
		}
		fmt.Printf("Executing shell command: %s\n", command)
		return tools.ExecuteShellCommand(command)

	case "read_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			return "", fmt.Errorf("invalid file_path argument")
		}
		fmt.Printf("Reading file: %s\n", filePath)
		return tools.ReadFile(filePath)

	case "write_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			return "", fmt.Errorf("invalid file_path argument")
		}
		content, ok := args["content"].(string)
		if !ok {
			return "", fmt.Errorf("invalid content argument")
		}
		fmt.Printf("Writing file: %s\n", filePath)
		return tools.WriteFile(filePath, content)

	case "edit_file":
		filePath, ok := args["file_path"].(string)
		if !ok {
			return "", fmt.Errorf("invalid file_path argument")
		}
		oldString, ok := args["old_string"].(string)
		if !ok {
			return "", fmt.Errorf("invalid old_string argument")
		}
		newString, ok := args["new_string"].(string)
		if !ok {
			return "", fmt.Errorf("invalid new_string argument")
		}
		fmt.Printf("Editing file: %s\n", filePath)
		return tools.EditFile(filePath, oldString, newString)

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
		fmt.Printf("Adding todo: %s\n", title)
		return tools.AddTodo(title, description, priority), nil

	case "update_todo_status":
		id, ok := args["id"].(string)
		if !ok {
			return "", fmt.Errorf("invalid id argument")
		}
		status, ok := args["status"].(string)
		if !ok {
			return "", fmt.Errorf("invalid status argument")
		}
		fmt.Printf("Updating todo %s to %s\n", id, status)
		return tools.UpdateTodoStatus(id, status), nil

	case "list_todos":
		fmt.Println("Listing todos")
		return tools.ListTodos(), nil

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
