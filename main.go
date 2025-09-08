package main

import (
	"bufio"
	"fmt"
	"gpt-chat/agent"
	"gpt-chat/api"
	"log"
	"os"
	"strings"

	"golang.org/x/term"
)

func main() {
	// Check for help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	// Check for local flag
	useLocal := false
	if len(os.Args) > 1 && (os.Args[1] == "--local" || os.Args[1] == "-l") {
		useLocal = true
		// Force Ollama client by unsetting the API key temporarily
		if os.Getenv("DEEPINFRA_API_KEY") != "" {
			fmt.Println("📍 Using local inference (--local flag detected)")
			os.Setenv("DEEPINFRA_API_KEY_BACKUP", os.Getenv("DEEPINFRA_API_KEY"))
			os.Unsetenv("DEEPINFRA_API_KEY")
		}
	}

	// Initialize the agent
	chatAgent, err := agent.NewAgent()
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	fmt.Println("🤖 GPT-OSS Chat Agent initialized successfully!")

	// Show which client is being used
	clientType := api.GetClientTypeFromEnv()
	if clientType == api.OllamaClientType {
		fmt.Println("🏠 Using local gpt-oss:20b model via Ollama")
		fmt.Println("💰 Cost: FREE (local inference)")
	} else {
		fmt.Println("☁️  Using gpt-oss-120b model via DeepInfra")
		fmt.Println("💰 Cost: ~$0.09/M input + $0.45/M output tokens")
	}

	if useLocal {
		fmt.Println("📍 Local mode forced by --local flag")
	}

	fmt.Println("Type your query or press Ctrl+C to exit")
	fmt.Println("=====================================")

	// Check if input is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Input is piped
		scanner := bufio.NewScanner(os.Stdin)
		var input strings.Builder
		for scanner.Scan() {
			input.WriteString(scanner.Text())
			input.WriteString("\n")
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading piped input: %v", err)
		}

		query := strings.TrimSpace(input.String())
		if query != "" {
			processQuery(chatAgent, query)
		}
		return
	}

	// Interactive mode
	fmt.Println("💡 Tip: Use \\ at end of line or paste multiline content directly")

	for {
		query, err := readMultilineInput()
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
		}

		query = strings.TrimSpace(query)

		if query == "" {
			continue
		}

		if query == "exit" || query == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if query == "help" {
			printHelp()
			continue
		}

		processQuery(chatAgent, query)
	}
}

func processQuery(chatAgent *agent.Agent, query string) {
	fmt.Println("\n🔍 Processing your query...")
	fmt.Println("Query:", query)
	fmt.Println("=====================================")

	result, err := chatAgent.ProcessQuery(query)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("\n✅ Task completed!")
	fmt.Println("=====================================")
	fmt.Println(result)
	fmt.Println("=====================================")

	// Print conversation summary
	chatAgent.PrintConversationSummary()
}

func printHelp() {
	fmt.Println(`
🤖 GPT-OSS Chat Agent

A command-line coding assistant using OpenAI's gpt-oss-120b model with 4 core tools:
- shell_command: Execute shell commands for exploration and testing  
- read_file: Read file contents
- write_file: Create new files
- edit_file: Modify existing files with precise string replacement

USAGE:
  Interactive mode:  ./gpt-chat
  Local inference:   ./gpt-chat --local  (uses Ollama gpt-oss:20b)
  Piped input:      echo "your query" | ./gpt-chat
  Help:             ./gpt-chat --help

MULTILINE INPUT:
  - End line with \ to continue on next line
  - Auto-continues for: code blocks, braces {}, brackets [], parentheses ()
  - Auto-continues for indented lines and comma-separated lists
  - Paste multiline content - it will be handled automatically

EXAMPLES:
  ./gpt-chat
  > Create a simple Go HTTP server in server.go
  
  > Fix this code:\
  > func main() {\
  >     // broken code here\
  > }
  
  echo "Fix the bug in main.go where the variable is undefined" | ./gpt-chat

ENVIRONMENT:
  DEEPINFRA_API_KEY: API token for DeepInfra (if not set, uses local Ollama)

MODEL OPTIONS:
  🏠 Local (Ollama):    gpt-oss:20b - FREE, runs locally (14GB VRAM)
  ☁️  Remote (DeepInfra): gpt-oss-120b - Paid, cloud-hosted (~$0.50/M tokens)

SETUP:
  Local:  ollama pull gpt-oss:20b
  Remote: export DEEPINFRA_API_KEY="your_api_key_here"

The agent follows a systematic exploration process and will autonomously:
- Explore your codebase using shell commands
- Read and understand relevant files
- Make precise modifications using the edit tool
- Create new files when needed
- Test and verify changes
- Continue iterating until the task is complete

Type 'help' during interactive mode for this help message.
Type 'exit' or 'quit' to end the session.
`)
}

// readMultilineInput reads user input with support for Shift+Enter to continue
// and Enter to submit. Returns the complete input string.
func readMultilineInput() (string, error) {
	// Try to use raw terminal input if available, fallback to line-by-line
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return readRawMultilineInput()
	}

	// Fallback to simple scanner for non-terminal input
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), scanner.Err()
	}
	return "", scanner.Err()
}

// readRawMultilineInput handles terminal raw input with Shift+Enter support
func readRawMultilineInput() (string, error) {
	var input strings.Builder

	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()

		// Handle different multiline scenarios:
		// 1. Backslash continuation
		if strings.HasSuffix(line, "\\") {
			line = strings.TrimSuffix(line, "\\")
			input.WriteString(line)
			input.WriteString("\n")
			fmt.Print("  ") // Continuation prompt
			continue
		}

		// 2. Detect common multiline patterns and auto-continue
		trimmed := strings.TrimSpace(line)
		if shouldContinueInput(trimmed, input.String()) {
			input.WriteString(line)
			input.WriteString("\n")
			fmt.Print("  ") // Continuation prompt
			continue
		}

		// 3. Regular line - add and finish
		input.WriteString(line)
		break
	}

	return input.String(), scanner.Err()
}

// shouldContinueInput determines if we should automatically continue multiline input
func shouldContinueInput(line, existingInput string) bool {
	// Auto-continue for code blocks and obvious multiline patterns
	if strings.HasSuffix(line, "{") ||
		strings.HasSuffix(line, "[") ||
		strings.HasSuffix(line, "(") ||
		strings.HasSuffix(line, ",") ||
		strings.Contains(line, "```") && !strings.HasSuffix(existingInput, "```") {
		return true
	}

	// Continue if we're inside a code block
	if strings.Contains(existingInput, "```") &&
		strings.Count(existingInput, "```")%2 == 1 {
		return true
	}

	// Continue for indented lines (likely code)
	if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
		return true
	}

	return false
}
