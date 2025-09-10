package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alantheprice/coder/agent"
	"github.com/alantheprice/coder/api"
	"github.com/alantheprice/coder/commands"
	"github.com/alantheprice/coder/config"
	"github.com/alantheprice/coder/tools"
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
	provider := ""
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
			provider = "ollama" // Force Ollama when --local is used
		case strings.HasPrefix(arg, "--model="):
			model = strings.TrimPrefix(arg, "--model=")
		case strings.HasPrefix(arg, "--provider="):
			provider = strings.TrimPrefix(arg, "--provider=")
		case !strings.HasPrefix(arg, "-"):
			// This is a positional argument - join all remaining args as the prompt
			prompt = strings.Join(args[i:], " ")
			break
		}
	}

	// Handle provider override if specified
	if provider != "" {
		if err := setProviderOverride(provider, useLocal); err != nil {
			log.Fatalf("Failed to set provider: %v", err)
		}
	}

	// Initialize the agent with optional model and provider
	var chatAgent *agent.Agent
	var err error

	// If model is specified, provider must also be specified (unless --local is used)
	if model != "" && provider == "" && !useLocal {
		log.Fatalf("Error: When specifying a model with --model, you must also specify --provider.\nExample: ./coder --provider=openrouter --model=deepseek/deepseek-chat-v3.1:free \"your query\"")
	}

	if model != "" {
		chatAgent, err = agent.NewAgentWithModel(model)
	} else {
		chatAgent, err = agent.NewAgent()
	}
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	debugLog(debug, "🤖 Coder initialized successfully!\n")

	// Initialize command registry for slash commands
	cmdRegistry := commands.NewCommandRegistry()

	// Show which provider is being used
	providerType := chatAgent.GetProviderType()
	providerName := api.GetProviderName(providerType)
	modelName := chatAgent.GetModel()

	if providerType == api.OllamaClientType {
		fmt.Printf("🤖 Selected model: %s via %s\n", modelName, providerName)
		debugLog(debug, "🏠 Using local gpt-oss:20b model via Ollama\n")
		debugLog(debug, "💰 Cost: FREE (local inference)\n")
	} else {
		if api.IsGPTOSSModel(modelName) {
			fmt.Printf("🤖 Selected model: %s via %s (harmony syntax)\n", modelName, providerName)
		} else {
			fmt.Printf("🤖 Selected model: %s via %s (standard format)\n", modelName, providerName)
		}
		debugLog(debug, "☁️  Using %s model via %s\n", modelName, providerName)
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

	// Set up a channel to catch interrupt signals for graceful shutdown
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine to handle graceful shutdown
	go func() {
		<-interruptChannel
		fmt.Println("\n🛑 Interrupt received! Shutting down gracefully...")
		chatAgent.PrintConciseSummary()
		os.Exit(0)
	}()

	for {
		query, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("\n👋 Goodbye! Here's your session summary:")
				chatAgent.PrintConciseSummary()
				break
			}
			log.Fatalf("Error reading input: %v", err)
		}

		query = strings.TrimSpace(query)

		if query == "" {
			continue
		}

		if query == "exit" || query == "quit" {
			fmt.Println("👋 Goodbye! Here's your session summary:")
			chatAgent.PrintConciseSummary()
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

// isShellCommand checks if the input looks like a shell command
func isShellCommand(input string) bool {
	input = strings.TrimSpace(input)

	// Common shell command prefixes
	shellPrefixes := []string{
		"ls", "cd", "pwd", "cat", "echo", "grep", "find", "git",
		"go ", "python", "node", "npm", "yarn", "docker", "kubectl",
		"curl", "wget", "ssh", "scp", "mv", "cp", "rm", "mkdir",
		"touch", "chmod", "chown", "ps", "top", "kill", "df", "du",
		"tar", "zip", "unzip", "gzip", "gunzip", "head", "tail",
		"diff", "patch", "make", "gcc", "g++", "clang", "javac",
		"rustc", "cargo", "dotnet", "php", "ruby", "perl", "awk",
		"sed", "cut", "sort", "uniq", "wc", "tee", "xargs", "env",
		"export", "source", "./", ".\\", "#", "$",
	}

	for _, prefix := range shellPrefixes {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}

	// Check for shell operators and redirection (but be more specific to avoid false positives)
	if strings.Contains(input, " && ") || strings.Contains(input, " || ") ||
		strings.Contains(input, " | ") {
		return true
	}

	// Check for redirection operators with surrounding spaces or at word boundaries
	// This avoids matching things like '<|return|>' or 'file<something>'
	if strings.Contains(input, " > ") || strings.Contains(input, " >> ") ||
		strings.Contains(input, " < ") || strings.HasSuffix(input, ">") ||
		strings.HasPrefix(input, ">") || strings.HasSuffix(input, "<") ||
		strings.HasPrefix(input, "<") {
		return true
	}

	return false
}

// executeShellCommandDirectly executes a shell command directly and prints output
func executeShellCommandDirectly(command string, debug bool) {
	debugLog(debug, "⚡ Direct shell command detected: %s\n", command)
	debugLog(debug, "=====================================\n")

	result, err := tools.ExecuteShellCommand(command)
	if err != nil {
		fmt.Printf("❌ Command failed: %v\n", err)
		fmt.Printf("Output: %s\n", result)
	} else {
		fmt.Printf("✅ Command executed successfully:\n")
		fmt.Printf("Output: %s\n", result)
	}

	debugLog(debug, "=====================================\n")
}

func processQuery(chatAgent *agent.Agent, query string, debug bool) {
	// Check if this is a shell command that should be executed directly
	if isShellCommand(query) {
		executeShellCommandDirectly(query, debug)
		return
	}

	// Validate input length before sending to LLM
	if !validateQueryLength(query) {
		return
	}

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

	// Print concise summary after task completion
	chatAgent.PrintConciseSummary()

	// Save conversation state for continuity
	if err := chatAgent.SaveState("default"); err != nil {
		debugLog(debug, "Warning: Failed to save conversation state: %v\n", err)
	}

	// Generate and save conversation summary for next run continuity
	if err := chatAgent.SaveConversationSummary(); err != nil {
		debugLog(debug, "Warning: Failed to save conversation summary: %v\n", err)
	}
}

// validateQueryLength validates query length and prompts for confirmation if needed
func validateQueryLength(query string) bool {
	queryLen := len(strings.TrimSpace(query))

	// Absolute minimum: reject anything under 3 characters
	if queryLen < 3 {
		fmt.Printf("❌ Query too short (%d characters). Minimum 3 characters required.\n", queryLen)
		return false
	}

	// For queries under 20 characters, ask for confirmation
	if queryLen < 20 {
		fmt.Printf("⚠️  Short query detected (%d characters): \"%s\"\n", queryLen, query)
		fmt.Print("Are you sure you want to process this? (y/N): ")

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("❌ Query cancelled.")
			return false
		}

		fmt.Println("✅ Proceeding with short query...")
	}

	return true
}

func printHelp() {
	fmt.Println(`
🤖 Coding agent

A command-line coding assistant using different providers with 4 core tools:
- shell_command: Execute shell commands for exploration and testing  
- read_file: Read file contents
- write_file: Create new files
- edit_file: Modify existing files with precise string replacement

USAGE:
  Interactive mode:     ./coder
  Non-interactive:      ./coder "your query here"
  Local inference:      ./coder --local "your query"
  Custom model:         ./coder --provider=deepinfra --model=deepseek-ai/ "your query"
  Custom provider:      ./coder --provider=ollama "your query"
  Piped input:         echo "your query" | ./coder
  Help:                ./coder --help

SLASH COMMANDS (Interactive Mode):
  /help                Show help and available slash commands
  /models              List available models and select model to use
  /models select       Interactive model selection
  /models <model_id>   Set model directly
  /provider            Show provider status and switch providers
  /provider select     Interactive provider selection
  /provider <name>     Switch to specific provider
  /init                Generate or regenerate project context
  /commit              Interactive commit workflow - select files and generate commit messages
  /continuity          Show conversation continuity information
  /info                Show detailed conversation summary and token usage
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
  
  # Use a different model (provider must be specified)
  ./coder --provider=deepinfra --model=meta-llama/Meta-Llama-3.1-70B-Instruct "Create a Python calculator"
  
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

// setProviderOverride temporarily overrides the provider for this session
func setProviderOverride(providerName string, useLocal bool) error {
	// Convert provider name to ClientType
	provider, err := config.GetProviderFromConfigName(strings.ToLower(providerName))
	if err != nil {
		return fmt.Errorf("unknown provider '%s'. Available: deepinfra, ollama, cerebras, openrouter, groq, deepseek", providerName)
	}

	// For local flag, force to Ollama and disable API keys temporarily
	if useLocal || provider == api.OllamaClientType {
		// Backup and unset API keys to force Ollama
		if os.Getenv("DEEPINFRA_API_KEY") != "" {
			os.Setenv("DEEPINFRA_API_KEY_BACKUP", os.Getenv("DEEPINFRA_API_KEY"))
			os.Unsetenv("DEEPINFRA_API_KEY")
		}
		if os.Getenv("CEREBRAS_API_KEY") != "" {
			os.Setenv("CEREBRAS_API_KEY_BACKUP", os.Getenv("CEREBRAS_API_KEY"))
			os.Unsetenv("CEREBRAS_API_KEY")
		}
		if os.Getenv("OPENROUTER_API_KEY") != "" {
			os.Setenv("OPENROUTER_API_KEY_BACKUP", os.Getenv("OPENROUTER_API_KEY"))
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		if os.Getenv("GROQ_API_KEY") != "" {
			os.Setenv("GROQ_API_KEY_BACKUP", os.Getenv("GROQ_API_KEY"))
			os.Unsetenv("GROQ_API_KEY")
		}
		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			os.Setenv("DEEPSEEK_API_KEY_BACKUP", os.Getenv("DEEPSEEK_API_KEY"))
			os.Unsetenv("DEEPSEEK_API_KEY")
		}
		fmt.Printf("📍 Using local inference (Ollama)\n")
		return nil
	}

	// For other providers, temporarily unset other API keys to force selection
	switch provider {
	case api.DeepInfraClientType:
		// Keep DEEPINFRA_API_KEY, unset others
		if os.Getenv("CEREBRAS_API_KEY") != "" {
			os.Setenv("CEREBRAS_API_KEY_BACKUP", os.Getenv("CEREBRAS_API_KEY"))
			os.Unsetenv("CEREBRAS_API_KEY")
		}
		if os.Getenv("OPENROUTER_API_KEY") != "" {
			os.Setenv("OPENROUTER_API_KEY_BACKUP", os.Getenv("OPENROUTER_API_KEY"))
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		if os.Getenv("GROQ_API_KEY") != "" {
			os.Setenv("GROQ_API_KEY_BACKUP", os.Getenv("GROQ_API_KEY"))
			os.Unsetenv("GROQ_API_KEY")
		}
		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			os.Setenv("DEEPSEEK_API_KEY_BACKUP", os.Getenv("DEEPSEEK_API_KEY"))
			os.Unsetenv("DEEPSEEK_API_KEY")
		}
		// Add similar cases for other providers as needed
	}

	fmt.Printf("📍 Using provider: %s\n", api.GetProviderName(provider))
	return nil
}
