package agent

import (
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alantheprice/coder/api"
)

// FileReadRecord tracks file reads to detect redundancy
type FileReadRecord struct {
	FilePath    string
	Content     string
	ContentHash string
	Timestamp   time.Time
	MessageIndex int
}

// ConversationOptimizer manages conversation history optimization
type ConversationOptimizer struct {
	fileReads map[string]*FileReadRecord // filepath -> latest read record
	enabled   bool
	debug     bool
}

// NewConversationOptimizer creates a new conversation optimizer
func NewConversationOptimizer(enabled bool, debug bool) *ConversationOptimizer {
	return &ConversationOptimizer{
		fileReads: make(map[string]*FileReadRecord),
		enabled:   enabled,
		debug:     debug,
	}
}

// OptimizeConversation optimizes the conversation history by removing redundant content
func (co *ConversationOptimizer) OptimizeConversation(messages []api.Message) []api.Message {
	if !co.enabled {
		return messages
	}

	optimized := make([]api.Message, 0, len(messages))
	
	for i, msg := range messages {
		if co.isRedundantFileRead(msg, i) {
			// Replace with summary
			summary := co.createFileReadSummary(msg)
			optimized = append(optimized, api.Message{
				Role:    msg.Role,
				Content: summary,
			})
			if co.debug {
				fmt.Printf("🔄 Optimized redundant file read: %s\n", co.extractFilePath(msg.Content))
			}
		} else {
			optimized = append(optimized, msg)
			// Track file reads for future optimization
			co.trackFileRead(msg, i)
		}
	}

	return optimized
}

// isRedundantFileRead checks if this message is a redundant file read
func (co *ConversationOptimizer) isRedundantFileRead(msg api.Message, index int) bool {
	if msg.Role != "user" {
		return false
	}

	// Check if this is a file read result
	if !strings.Contains(msg.Content, "Tool call result for read_file:") {
		return false
	}

	filePath := co.extractFilePath(msg.Content)
	if filePath == "" {
		return false
	}

	// Check if we have a previous read of this file
	if record, exists := co.fileReads[filePath]; exists {
		// Extract current content
		currentContent := co.extractFileContent(msg.Content)
		currentHash := co.hashContent(currentContent)
		
		// If content hasn't changed and this isn't the most recent read, it's redundant
		if record.ContentHash == currentHash && record.MessageIndex < index {
			return true
		}
	}

	return false
}

// trackFileRead records a file read for future optimization
func (co *ConversationOptimizer) trackFileRead(msg api.Message, index int) {
	if msg.Role != "user" || !strings.Contains(msg.Content, "Tool call result for read_file:") {
		return
	}

	filePath := co.extractFilePath(msg.Content)
	if filePath == "" {
		return
	}

	content := co.extractFileContent(msg.Content)
	hash := co.hashContent(content)

	co.fileReads[filePath] = &FileReadRecord{
		FilePath:     filePath,
		Content:      content,
		ContentHash:  hash,
		Timestamp:    time.Now(),
		MessageIndex: index,
	}
}

// extractFilePath extracts the file path from a tool call result message
func (co *ConversationOptimizer) extractFilePath(content string) string {
	// Pattern: "Tool call result for read_file: <filepath>"
	re := regexp.MustCompile(`Tool call result for read_file:\s*([^\s\n]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractFileContent extracts the file content from a tool call result message
func (co *ConversationOptimizer) extractFileContent(content string) string {
	// Find the content after the file path
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return ""
	}
	
	// Skip the first line (tool call result header) and join the rest
	return strings.Join(lines[1:], "\n")
}

// hashContent creates a hash of file content for comparison
func (co *ConversationOptimizer) hashContent(content string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}

// createFileReadSummary creates a summary for a redundant file read
func (co *ConversationOptimizer) createFileReadSummary(msg api.Message) string {
	filePath := co.extractFilePath(msg.Content)
	content := co.extractFileContent(msg.Content)
	
	// Count lines and characters
	lines := strings.Split(strings.TrimSpace(content), "\n")
	lineCount := len(lines)
	charCount := len(content)
	
	// Determine file type
	fileType := "file"
	if strings.HasSuffix(filePath, ".go") {
		fileType = "Go file"
	} else if strings.HasSuffix(filePath, ".md") {
		fileType = "Markdown file"
	} else if strings.HasSuffix(filePath, ".json") {
		fileType = "JSON file"
	}

	return fmt.Sprintf("Tool call result for read_file: %s\n[OPTIMIZED] Previously read %s (%d lines, %d chars) - content unchanged since last read",
		filePath, fileType, lineCount, charCount)
}

// GetOptimizationStats returns statistics about optimization
func (co *ConversationOptimizer) GetOptimizationStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         co.enabled,
		"tracked_files":   len(co.fileReads),
		"file_paths":     co.getTrackedFilePaths(),
	}
}

// getTrackedFilePaths returns list of tracked file paths
func (co *ConversationOptimizer) getTrackedFilePaths() []string {
	paths := make([]string, 0, len(co.fileReads))
	for path := range co.fileReads {
		paths = append(paths, path)
	}
	return paths
}

// Reset clears all optimization state
func (co *ConversationOptimizer) Reset() {
	co.fileReads = make(map[string]*FileReadRecord)
}

// SetEnabled enables or disables optimization
func (co *ConversationOptimizer) SetEnabled(enabled bool) {
	co.enabled = enabled
}

// IsEnabled returns whether optimization is enabled
func (co *ConversationOptimizer) IsEnabled() bool {
	return co.enabled
}