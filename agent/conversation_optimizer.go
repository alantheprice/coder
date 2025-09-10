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

// ShellCommandRecord tracks shell commands to detect redundancy
type ShellCommandRecord struct {
	Command     string
	Output      string
	OutputHash  string
	Timestamp   time.Time
	MessageIndex int
	IsTransient bool // Commands like ls, find that become less relevant over time
}

// ConversationOptimizer manages conversation history optimization
type ConversationOptimizer struct {
	fileReads     map[string]*FileReadRecord    // filepath -> latest read record
	shellCommands map[string]*ShellCommandRecord // command -> latest execution record
	enabled       bool
	debug         bool
}

// NewConversationOptimizer creates a new conversation optimizer
func NewConversationOptimizer(enabled bool, debug bool) *ConversationOptimizer {
	return &ConversationOptimizer{
		fileReads:     make(map[string]*FileReadRecord),
		shellCommands: make(map[string]*ShellCommandRecord),
		enabled:       enabled,
		debug:         debug,
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
		} else if co.isRedundantShellCommand(msg, i) {
			// Replace with summary
			summary := co.createShellCommandSummary(msg)
			optimized = append(optimized, api.Message{
				Role:    msg.Role,
				Content: summary,
			})
			if co.debug {
				fmt.Printf("🔄 Optimized redundant shell command: %s\n", co.extractShellCommand(msg.Content))
			}
		} else {
			optimized = append(optimized, msg)
			// Track file reads and shell commands for future optimization
			co.trackFileRead(msg, i)
			co.trackShellCommand(msg, i)
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
		"enabled":           co.enabled,
		"tracked_files":     len(co.fileReads),
		"tracked_commands":  len(co.shellCommands),
		"file_paths":       co.getTrackedFilePaths(),
		"shell_commands":   co.getTrackedCommands(),
	}
}

// getTrackedCommands returns list of tracked shell commands
func (co *ConversationOptimizer) getTrackedCommands() []string {
	commands := make([]string, 0, len(co.shellCommands))
	for command := range co.shellCommands {
		commands = append(commands, command)
	}
	return commands
}

// getTrackedFilePaths returns list of tracked file paths
func (co *ConversationOptimizer) getTrackedFilePaths() []string {
	paths := make([]string, 0, len(co.fileReads))
	for path := range co.fileReads {
		paths = append(paths, path)
	}
	return paths
}

// isRedundantShellCommand checks if this message is a redundant shell command
func (co *ConversationOptimizer) isRedundantShellCommand(msg api.Message, index int) bool {
	if msg.Role != "user" {
		return false
	}

	// Check if this is a shell command result
	if !strings.Contains(msg.Content, "Tool call result for shell_command:") {
		return false
	}

	command := co.extractShellCommand(msg.Content)
	if command == "" {
		return false
	}

	// Check if we have a previous execution of this command
	if record, exists := co.shellCommands[command]; exists {
		// Extract current output
		currentOutput := co.extractShellOutput(msg.Content)
		currentHash := co.hashContent(currentOutput)
		
		// Check if this is a transient command that should be optimized after some time
		if record.IsTransient && record.MessageIndex < index-2 {
			return true
		}
		
		// If output hasn't changed and this isn't the most recent execution, it's redundant
		if record.OutputHash == currentHash && record.MessageIndex < index {
			return true
		}
	}

	return false
}

// trackShellCommand records a shell command execution for future optimization
func (co *ConversationOptimizer) trackShellCommand(msg api.Message, index int) {
	if msg.Role != "user" || !strings.Contains(msg.Content, "Tool call result for shell_command:") {
		return
	}

	command := co.extractShellCommand(msg.Content)
	if command == "" {
		return
	}

	output := co.extractShellOutput(msg.Content)
	hash := co.hashContent(output)
	isTransient := co.isTransientCommand(command)

	co.shellCommands[command] = &ShellCommandRecord{
		Command:      command,
		Output:       output,
		OutputHash:   hash,
		Timestamp:    time.Now(),
		MessageIndex: index,
		IsTransient:  isTransient,
	}
}

// extractShellCommand extracts the shell command from a tool call result message
func (co *ConversationOptimizer) extractShellCommand(content string) string {
	// Pattern: "Tool call result for shell_command: <command>"
	re := regexp.MustCompile(`Tool call result for shell_command:\s*([^\n]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractShellOutput extracts the shell command output from a tool call result message
func (co *ConversationOptimizer) extractShellOutput(content string) string {
	// Find the output after the command line
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return ""
	}
	
	// Skip the first line (tool call result header) and join the rest
	return strings.Join(lines[1:], "\n")
}

// isTransientCommand checks if a command is transient (exploration commands that become stale)
func (co *ConversationOptimizer) isTransientCommand(command string) bool {
	transientPatterns := []string{
		"ls", "find", "grep", "tree", "pwd", "whoami", "date", "ps",
		"df", "du", "which", "whereis", "locate", "file", "stat",
	}
	
	cmdLower := strings.ToLower(command)
	for _, pattern := range transientPatterns {
		if strings.HasPrefix(cmdLower, pattern+" ") || cmdLower == pattern {
			return true
		}
	}
	return false
}

// createShellCommandSummary creates a summary for a redundant shell command
func (co *ConversationOptimizer) createShellCommandSummary(msg api.Message) string {
	command := co.extractShellCommand(msg.Content)
	output := co.extractShellOutput(msg.Content)
	
	// Count lines and characters in output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	lineCount := len(lines)
	charCount := len(output)
	
	// Determine command type
	commandType := "command"
	if co.isTransientCommand(command) {
		commandType = "exploration command"
	}

	return fmt.Sprintf("Tool call result for shell_command: %s\n[OPTIMIZED] Previously executed %s (%d lines output, %d chars) - output unchanged since last execution",
		command, commandType, lineCount, charCount)
}

// Reset clears all optimization state
func (co *ConversationOptimizer) Reset() {
	co.fileReads = make(map[string]*FileReadRecord)
	co.shellCommands = make(map[string]*ShellCommandRecord)
}

// SetEnabled enables or disables optimization
func (co *ConversationOptimizer) SetEnabled(enabled bool) {
	co.enabled = enabled
}

// IsEnabled returns whether optimization is enabled
func (co *ConversationOptimizer) IsEnabled() bool {
	return co.enabled
}