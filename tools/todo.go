package tools

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`             // pending, in_progress, completed, cancelled
	Priority    string    `json:"priority,omitempty"` // high, medium, low
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TodoManager manages the todo list for the current session
type TodoManager struct {
	items []TodoItem
	mutex sync.RWMutex
}

var globalTodoManager = &TodoManager{
	items: make([]TodoItem, 0),
}

// AddTodo adds a new todo item
func AddTodo(title, description, priority string) string {
	globalTodoManager.mutex.Lock()
	defer globalTodoManager.mutex.Unlock()

	if priority == "" {
		priority = "medium"
	}

	item := TodoItem{
		ID:          fmt.Sprintf("todo_%d", len(globalTodoManager.items)+1),
		Title:       title,
		Description: description,
		Status:      "pending",
		Priority:    priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	globalTodoManager.items = append(globalTodoManager.items, item)
	return fmt.Sprintf("✅ Added todo: %s (ID: %s)", title, item.ID)
}

// UpdateTodoStatus updates the status of a todo item
func UpdateTodoStatus(id, status string) string {
	globalTodoManager.mutex.Lock()
	defer globalTodoManager.mutex.Unlock()

	validStatuses := map[string]bool{
		"pending":     true,
		"in_progress": true,
		"completed":   true,
		"cancelled":   true,
	}

	if !validStatuses[status] {
		return fmt.Sprintf("❌ Invalid status: %s. Valid statuses: pending, in_progress, completed, cancelled", status)
	}

	for i, item := range globalTodoManager.items {
		if item.ID == id {
			globalTodoManager.items[i].Status = status
			globalTodoManager.items[i].UpdatedAt = time.Now()

			emoji := getStatusEmoji(status)
			return fmt.Sprintf("%s Updated todo %s: %s", emoji, id, item.Title)
		}
	}

	return fmt.Sprintf("❌ Todo not found: %s", id)
}

// ListTodos returns a formatted list of all todos
func ListTodos() string {
	globalTodoManager.mutex.RLock()
	defer globalTodoManager.mutex.RUnlock()

	if len(globalTodoManager.items) == 0 {
		return "📝 No todos yet"
	}

	var result strings.Builder
	result.WriteString("📝 **Current Todos:**\n\n")

	// Group by status
	statusGroups := map[string][]TodoItem{
		"in_progress": {},
		"pending":     {},
		"completed":   {},
		"cancelled":   {},
	}

	for _, item := range globalTodoManager.items {
		statusGroups[item.Status] = append(statusGroups[item.Status], item)
	}

	// Show in progress first
	for _, status := range []string{"in_progress", "pending", "completed", "cancelled"} {
		items := statusGroups[status]
		if len(items) == 0 {
			continue
		}

		result.WriteString(fmt.Sprintf("### %s %s\n", getStatusEmoji(status), strings.Title(strings.Replace(status, "_", " ", -1))))
		for _, item := range items {
			priority := ""
			if item.Priority != "" {
				priority = fmt.Sprintf(" [%s]", strings.ToUpper(item.Priority))
			}
			result.WriteString(fmt.Sprintf("- **%s**%s: %s", item.Title, priority, item.ID))
			if item.Description != "" {
				result.WriteString(fmt.Sprintf(" - %s", item.Description))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	return result.String()
}

// GetTaskSummary generates a markdown summary of completed work
func GetTaskSummary() string {
	globalTodoManager.mutex.RLock()
	defer globalTodoManager.mutex.RUnlock()

	if len(globalTodoManager.items) == 0 {
		return "No tasks tracked in this session."
	}

	var result strings.Builder
	result.WriteString("## 📋 Task Summary\n\n")

	completed := 0
	inProgress := 0
	pending := 0
	cancelled := 0

	var completedTasks []TodoItem
	var inProgressTasks []TodoItem

	for _, item := range globalTodoManager.items {
		switch item.Status {
		case "completed":
			completed++
			completedTasks = append(completedTasks, item)
		case "in_progress":
			inProgress++
			inProgressTasks = append(inProgressTasks, item)
		case "pending":
			pending++
		case "cancelled":
			cancelled++
		}
	}

	// Progress overview
	total := len(globalTodoManager.items)
	result.WriteString(fmt.Sprintf("**Progress:** %d/%d tasks completed", completed, total))
	if inProgress > 0 {
		result.WriteString(fmt.Sprintf(" (%d in progress)", inProgress))
	}
	result.WriteString("\n\n")

	// Show completed tasks
	if len(completedTasks) > 0 {
		result.WriteString("### ✅ Completed\n")
		for _, item := range completedTasks {
			result.WriteString(fmt.Sprintf("- %s", item.Title))
			if item.Description != "" {
				result.WriteString(fmt.Sprintf(": %s", item.Description))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	// Show in progress tasks
	if len(inProgressTasks) > 0 {
		result.WriteString("### 🔄 In Progress\n")
		for _, item := range inProgressTasks {
			result.WriteString(fmt.Sprintf("- %s", item.Title))
			if item.Description != "" {
				result.WriteString(fmt.Sprintf(": %s", item.Description))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	if pending > 0 {
		result.WriteString(fmt.Sprintf("### ⏳ %d tasks remaining\n\n", pending))
	}

	return result.String()
}

// ClearTodos clears all todos (for new sessions)
func ClearTodos() string {
	globalTodoManager.mutex.Lock()
	defer globalTodoManager.mutex.Unlock()

	count := len(globalTodoManager.items)
	globalTodoManager.items = make([]TodoItem, 0)
	return fmt.Sprintf("🗑️ Cleared %d todos", count)
}

func getStatusEmoji(status string) string {
	switch status {
	case "pending":
		return "⏳"
	case "in_progress":
		return "🔄"
	case "completed":
		return "✅"
	case "cancelled":
		return "❌"
	default:
		return "📝"
	}
}
