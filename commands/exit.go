package commands

import (
	"fmt"
	"os"

	"github.com/alantheprice/coder/agent"
)

// ExitCommand implements the /exit slash command
type ExitCommand struct{}

// Name returns the command name
func (e *ExitCommand) Name() string {
	return "exit"
}

// Description returns the command description
func (e *ExitCommand) Description() string {
	return "Exit the interactive session"
}

// Execute runs the exit command
func (e *ExitCommand) Execute(args []string, chatAgent *agent.Agent) error {
	fmt.Println("👋 Goodbye!")
	os.Exit(0)
	return nil // This line won't be reached due to os.Exit
}