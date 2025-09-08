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

	// Start assistant response
	result.WriteString("<|start|>assistant")

	return result.String()
}

// formatToolParameters converts JSON schema to TypeScript-like parameters
func (h *HarmonyFormatter) formatToolParameters(params interface{}) string {
	if params == nil {
		return ""
	}

	// This is a simplified implementation - in practice you'd want to
	// properly parse the JSON schema and convert to TypeScript types
	return "_: any"
}

// NewHarmonyFormatter creates a new harmony formatter
func NewHarmonyFormatter() *HarmonyFormatter {
	return &HarmonyFormatter{}
}
