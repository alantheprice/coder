package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alantheprice/coder/agent"
	"github.com/alantheprice/coder/api"
	"github.com/chzyer/readline"
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

	// Interactive mode with readline support
	homeDir, _ := os.UserHomeDir()
	historyFile := homeDir + "/.gpt_chat_history"

	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "> ",
		HistoryFile:  historyFile,
		HistoryLimit: 1000,
	})
	if err != nil {
		log.Fatalf("Failed to initialize readline: %v", err)
	}
	defer rl.Close()

	fmt.Println("💡 Tip: Use arrow keys to navigate, backspace to edit, up/down for history, Ctrl+C to exit")

	for {
		query, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("Goodbye!")
				break
			}
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

INPUT FEATURES:
  - Arrow keys for navigation and command history
  - Backspace/Delete for editing
  - Tab for completion (where available)
  - Ctrl+C to exit

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
