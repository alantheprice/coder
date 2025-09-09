package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alantheprice/coder/agent"
	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/commands"
	"github.com/chzyer/readline"
)

// debugLog logs a message only if debug mode is enabled
func debugLog(debug bool, format string, args ...interface{}) {
	if debug {
		fmt.Printf(format, args...)
	}
}

func main() {
	// Parse command line arguments
	var prompt string
	useLocal := false
	model := ""
	debug := os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1"

	args := os.Args[1:] // Skip program name

	// Process flags and positional arguments
	for i, arg := range args {
		switch {
		case arg == "--help" || arg == "-h":
			printHelp()
			return
		case arg == "--local" || arg == "-l":
			useLocal = true
			// Force Ollama client by unsetting the API key temporarily
			if os.Getenv("DEEPINFRA_API_KEY") != "" {
				debugLog(debug, "📍 Using local inference (--local flag detected)\n")
				os.Setenv("DEEPINFRA_API_KEY_BACKUP", os.Getenv("DEEPINFRA_API_KEY"))
				os.Unsetenv("DEEPINFRA_API_KEY")
			}
		case strings.HasPrefix(arg, "--model="):
			model = strings.TrimPrefix(arg, "--model=")
		case !strings.HasPrefix(arg, "-"):
			// This is a positional argument - join all remaining args as the prompt
			prompt = strings.Join(args[i:], " ")
			break
		}
	}

	// Initialize the agent with optional model
	var chatAgent *agent.Agent
	var err error
	if model != "" {
		chatAgent, err = agent.NewAgentWithModel(model)
	} else {
		chatAgent, err = agent.NewAgent()
	}
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	debugLog(debug, "🤖 GPT-OSS Chat Agent initialized successfully!\n")

	// Initialize command registry for slash commands
	cmdRegistry := commands.NewCommandRegistry()

	// Show which client is being used
	clientType := api.GetClientTypeFromEnv()
	if clientType == api.OllamaClientType {
		debugLog(debug, "🏠 Using local gpt-oss:20b model via Ollama\n")
		debugLog(debug, "💰 Cost: FREE (local inference)\n")
	} else {
		modelName := model
		if modelName == "" {
			modelName = api.DefaultModel
		}
		if api.IsGPTOSSModel(modelName) {
			debugLog(debug, "☁️  Using %s model via DeepInfra (harmony syntax)\n", modelName)
		} else {
			debugLog(debug, "☁️  Using %s model via DeepInfra (standard format)\n", modelName)
		}
		debugLog(debug, "💰 Cost: Pay per use (see /models for pricing)\n")
	}

	if useLocal {
		debugLog(debug, "📍 Local mode forced by --local flag\n")
	}

	// Handle different input modes
	if prompt != "" {
		// Non-interactive mode: execute the provided prompt and exit
		debugLog(debug, "🔍 Processing your query...\n")
		debugLog(debug, "Query: %s\n", prompt)
		debugLog(debug, "=====================================\n")
		processQuery(chatAgent, prompt, debug)
		return
	}

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
			debugLog(debug, "🔍 Processing your query...\n")
			debugLog(debug, "Query: %s\n", query)
			debugLog(debug, "=====================================\n")
			processQuery(chatAgent, query, debug)
		}
		return
	}

	// Interactive mode
	debugLog(debug, "Type your query or press Ctrl+C to exit\n")
	debugLog(debug, "=====================================\n")

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

	debugLog(debug, "💡 Tip: Use arrow keys to navigate, backspace to edit, up/down for history, Ctrl+C to exit\n")

	// Show selected model before prompt
	currentModel := chatAgent.GetModel()
	fmt.Printf("🤖 Selected model: %s\n", currentModel)

	for {
		query, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				debugLog(debug, "Goodbye!\n")
				break
			}
			log.Fatalf("Error reading input: %v", err)
		}

		query = strings.TrimSpace(query)

		if query == "" {
			continue
		}

		if query == "exit" || query == "quit" {
			debugLog(debug, "Goodbye!\n")
			break
		}

		if query == "help" {
			printHelp()
			continue
		}

		// Check if it's a slash command
		if cmdRegistry.IsSlashCommand(query) {
			err := cmdRegistry.Execute(query, chatAgent)
			if err != nil {
				fmt.Printf("❌ Command error: %v\n", err)
			}
			continue
		}

		processQuery(chatAgent, query, debug)
	}
}

func processQuery(chatAgent *agent.Agent, query string, debug bool) {
	debugLog(debug, "\n🔍 Processing your query...\n")
	debugLog(debug, "Query: %s\n", query)
	debugLog(debug, "=====================================\n")

	result, err := chatAgent.ProcessQuery(query)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("\n✅ Task completed!")
	fmt.Println("=====================================")
	fmt.Println(result)
	fmt.Println("=====================================")

	// Print conversation summary (always show)
	chatAgent.PrintConversationSummary()
	
	// Save conversation state for continuity
	if err := chatAgent.SaveState("default"); err != nil {
		debugLog(debug, "Warning: Failed to save conversation state: %v\n", err)
	}
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
  Interactive mode:     ./coder
  Non-interactive:      ./coder "your query here"
  Local inference:      ./coder --local "your query"
  Custom model:         ./coder --model=meta-llama/Meta-Llama-3.1-70B-Instruct "your query"
  Piped input:         echo "your query" | ./coder
  Help:                ./coder --help

SLASH COMMANDS (Interactive Mode):
  /help                Show help and available slash commands
  /models              List available models and select model to use
  /models select       Interactive model selection
  /models <model_id>   Set model directly
  /init                Generate or regenerate project context
  /commit              Interactive commit workflow - select files and generate commit messages
  /continuity          Show conversation continuity information
  /exit                Exit the interactive session

INPUT FEATURES:
  - Arrow keys for navigation and command history
  - Backspace/Delete for editing
  - Tab for completion (where available)
  - Ctrl+C to exit

EXAMPLES:
  # Interactive mode
  ./coder
  > Create a simple Go HTTP server in server.go
  
  # Non-interactive mode
  ./coder "Create a simple Go HTTP server in server.go"
  
  # Multi-word prompts (use quotes)
  ./coder "Fix the bug in main.go and add unit tests"
  
  # Local inference
  ./coder --local "Create a Python calculator"
  
  # Use a different model
  ./coder --model=meta-llama/Meta-Llama-3.1-70B-Instruct "Create a Python calculator"
  
  # Piped input
  echo "Fix the bug in main.go where the variable is undefined" | ./coder

ENVIRONMENT:
  DEEPINFRA_API_KEY: API token for DeepInfra (if not set, uses local Ollama)

MODEL OPTIONS:
  🏠 Local (Ollama):    gpt-oss:20b - FREE, runs locally (14GB VRAM)
  ☁️  Remote (DeepInfra): Multiple models available:
     • openai/gpt-oss-120b (default) - Uses harmony syntax
     • meta-llama/Meta-Llama-3.1-70B-Instruct - Standard format
     • microsoft/WizardLM-2-8x22B - Standard format
     • And many others - check DeepInfra docs for full list

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
