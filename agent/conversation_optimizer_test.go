package agent

import (
	"testing"

	"github.com/alantheprice/coder/api"
)

func TestConversationOptimizer(t *testing.T) {
	optimizer := NewConversationOptimizer(true, false)

	// Test data - simulate a conversation with redundant file reads
	messages := []api.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Please read agent.go"},
		{Role: "assistant", Content: "I'll read the file for you."},
		{Role: "user", Content: "Tool call result for read_file: agent/agent.go\npackage agent\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"},
		{Role: "assistant", Content: "I've read the file. Now let me read it again to demonstrate optimization."},
		{Role: "user", Content: "Tool call result for read_file: agent/agent.go\npackage agent\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"},
	}

	optimized := optimizer.OptimizeConversation(messages)

	// Verify optimization occurred
	if len(optimized) >= len(messages) {
		t.Errorf("Expected optimization to reduce message count, got %d -> %d", len(messages), len(optimized))
	}

	// Check that the second file read was optimized
	lastMsg := optimized[len(optimized)-1]
	if !containsString(lastMsg.Content, "[OPTIMIZED]") {
		t.Errorf("Expected last message to contain [OPTIMIZED], got: %s", lastMsg.Content)
	}

	// Verify stats
	stats := optimizer.GetOptimizationStats()
	if stats["tracked_files"].(int) == 0 {
		t.Errorf("Expected tracked files > 0, got %d", stats["tracked_files"])
	}
}

func TestFileReadDetection(t *testing.T) {
	optimizer := NewConversationOptimizer(true, false)

	// Test file path extraction
	content := "Tool call result for read_file: agent/agent.go\npackage agent\n\nfunc main() {}"
	filePath := optimizer.extractFilePath(content)
	if filePath != "agent/agent.go" {
		t.Errorf("Expected file path 'agent/agent.go', got '%s'", filePath)
	}

	// Test file content extraction
	fileContent := optimizer.extractFileContent(content)
	expected := "package agent\n\nfunc main() {}"
	if fileContent != expected {
		t.Errorf("Expected file content '%s', got '%s'", expected, fileContent)
	}
}

func TestOptimizationDisabled(t *testing.T) {
	optimizer := NewConversationOptimizer(false, false)

	messages := []api.Message{
		{Role: "user", Content: "Tool call result for read_file: test.go\ncontent"},
		{Role: "user", Content: "Tool call result for read_file: test.go\ncontent"},
	}

	optimized := optimizer.OptimizeConversation(messages)

	// Should not optimize when disabled
	if len(optimized) != len(messages) {
		t.Errorf("Expected no optimization when disabled, got %d -> %d", len(messages), len(optimized))
	}
}

func TestFileContentChange(t *testing.T) {
	optimizer := NewConversationOptimizer(true, false)

	messages := []api.Message{
		{Role: "user", Content: "Tool call result for read_file: test.go\noriginal content"},
		{Role: "user", Content: "Tool call result for read_file: test.go\nmodified content"},
	}

	optimized := optimizer.OptimizeConversation(messages)

	// Should not optimize if content changed
	if len(optimized) != len(messages) {
		t.Errorf("Expected no optimization when content changed, got %d -> %d", len(messages), len(optimized))
	}
}

func TestCreateFileReadSummary(t *testing.T) {
	optimizer := NewConversationOptimizer(true, false)

	msg := api.Message{
		Role:    "user",
		Content: "Tool call result for read_file: test.go\npackage main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}",
	}

	summary := optimizer.createFileReadSummary(msg)

	if !containsString(summary, "[OPTIMIZED]") {
		t.Errorf("Expected summary to contain [OPTIMIZED], got: %s", summary)
	}

	if !containsString(summary, "test.go") {
		t.Errorf("Expected summary to contain file path, got: %s", summary)
	}

	if !containsString(summary, "Go file") {
		t.Errorf("Expected summary to identify Go file type, got: %s", summary)
	}
}

// Helper function to check if string contains substring
func containsString(text, substr string) bool {
	return len(text) >= len(substr) && findSubstring(text, substr) != -1
}

// Simple substring search
func findSubstring(text, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}