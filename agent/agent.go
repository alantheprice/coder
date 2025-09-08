package agent

import (
	"encoding/json"
	"fmt"
	"gpt-chat/api"
	"gpt-chat/tools"
	"io"
	"os"
	"strings"
)

type Agent struct {
	client           *api.Client
	messages         []api.Message
	systemPrompt     string
	maxIterations    int
	currentIteration int
	totalCost        float64
}

func NewAgent(systemPromptFile string) (*Agent, error) {
	client, err := api.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Load system prompt
	systemPrompt, err := loadSystemPrompt(systemPromptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	return &Agent{
		client:        client,
		messages:      []api.Message{},
		systemPrompt:  systemPrompt,
		maxIterations: 40, // Increased from 20 for more complex tasks
		totalCost:     0.0,
	}, nil
}

func loadSystemPrompt(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
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

		// Send request to API
		req := api.ChatRequest{
			Model:      api.Model,
			Messages:   a.messages,
			Tools:      api.GetToolDefinitions(),
			ToolChoice: "auto",
			MaxTokens:  4000,
			Reasoning:  "high",
		}

		resp, err := a.client.SendChatRequest(req)
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
