package api

import (
	"fmt"
	"strings"
)

// HarmonyFormatter handles conversion from OpenAI format to harmony format
type HarmonyFormatter struct{}

// FormatMessagesForCompletion converts OpenAI-style messages to harmony format
func (h *HarmonyFormatter) FormatMessagesForCompletion(messages []Message, tools []Tool) string {
	var result strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			result.WriteString(fmt.Sprintf("<|start|>system<|message|>%s<|end|>\n\n", msg.Content))
		case "user":
			result.WriteString(fmt.Sprintf("<|start|>user<|message|>%s<|end|>", msg.Content))
		case "assistant":
			result.WriteString(fmt.Sprintf("<|start|>assistant<|channel|>final<|message|>%s<|end|>\n\n", msg.Content))
		}
	}

	// Add tools to developer message if provided
	if len(tools) > 0 {
		result.WriteString("<|start|>developer<|message|># Tools\n\n")
		result.WriteString("## functions\n\n")
		result.WriteString("namespace functions {\n\n")

		for _, tool := range tools {
			// Convert tool definition to TypeScript-like format
			if tool.Type == "function" {
				result.WriteString(fmt.Sprintf("// %s\ntype %s = (%s) => any;\n\n",
					tool.Function.Description,
					tool.Function.Name,
					h.formatToolParameters(tool.Function.Parameters)))
			}
		}

		result.WriteString("} // namespace functions<|end|>")
	}

	// Start assistant response with comprehensive tool calling guidance
	result.WriteString("<|start|>assistant\n\n")
	result.WriteString("You can call tools by responding with a JSON object containing tool_calls. Use EXACTLY these formats:\n\n")
	
	// Provide specific examples for each tool
	result.WriteString("**Shell command:**\n")
	result.WriteString("{\"tool_calls\": [{\"id\": \"call_1\", \"type\": \"function\", \"function\": {\"name\": \"shell_command\", \"arguments\": \"{\\\"command\\\": \\\"ls -la\\\"}\"}}]}\n\n")
	
	result.WriteString("**Read file:**\n")
	result.WriteString("{\"tool_calls\": [{\"id\": \"call_1\", \"type\": \"function\", \"function\": {\"name\": \"read_file\", \"arguments\": \"{\\\"file_path\\\": \\\"filename.go\\\"}\"}}]}\n\n")
	
	result.WriteString("**Edit file:**\n")
	result.WriteString("{\"tool_calls\": [{\"id\": \"call_1\", \"type\": \"function\", \"function\": {\"name\": \"edit_file\", \"arguments\": \"{\\\"file_path\\\": \\\"filename.go\\\", \\\"old_string\\\": \\\"exact text\\\", \\\"new_string\\\": \\\"replacement\\\"}\"}}]}\n\n")
	
	result.WriteString("**Write file:**\n")
	result.WriteString("{\"tool_calls\": [{\"id\": \"call_1\", \"type\": \"function\", \"function\": {\"name\": \"write_file\", \"arguments\": \"{\\\"file_path\\\": \\\"filename.go\\\", \\\"content\\\": \\\"file contents\\\"}\"}}]}\n\n")
	
	result.WriteString("CRITICAL: Copy these formats exactly. Never use other tool names like 'exec', 'bash', 'cmd', 'open_file', etc.\n\n")

	return result.String()
}

// formatToolParameters converts JSON schema to TypeScript-like parameters
func (h *HarmonyFormatter) formatToolParameters(params interface{}) string {
	if params == nil {
		return "_: any"
	}

	// Parse the JSON schema and convert to TypeScript-like syntax
	if paramsMap, ok := params.(map[string]interface{}); ok {
		if props, exists := paramsMap["properties"]; exists {
			if propsMap, ok := props.(map[string]interface{}); ok {
				var paramParts []string
				for paramName, paramDef := range propsMap {
					if defMap, ok := paramDef.(map[string]interface{}); ok {
						paramType := "string" // default
						if typeVal, exists := defMap["type"]; exists {
							if typeStr, ok := typeVal.(string); ok {
								paramType = typeStr
							}
						}
						paramParts = append(paramParts, fmt.Sprintf("%s: %s", paramName, paramType))
					}
				}
				if len(paramParts) > 0 {
					return fmt.Sprintf("{%s}", strings.Join(paramParts, ", "))
				}
			}
		}
	}
	
	return "_: any"
}

// NewHarmonyFormatter creates a new harmony formatter
func NewHarmonyFormatter() *HarmonyFormatter {
	return &HarmonyFormatter{}
}
