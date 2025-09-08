package main

import (
	"bufio"
	"fmt"
	"gpt-chat/agent"
	"log"
	"os"
	"strings"
)

func main() {
	// Check for help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	// Initialize the agent
	chatAgent, err := agent.NewAgent("systematic_exploration_prompt.md")
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	fmt.Println("🤖 GPT-OSS Chat Agent initialized successfully!")
	fmt.Println("Using OpenAI gpt-oss-120b model via DeepInfra")
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
	fmt.Print("\n> ")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		query := strings.TrimSpace(scanner.Text())

		if query == "" {
			fmt.Print("> ")
			continue
		}

		if query == "exit" || query == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if query == "help" {
			printHelp()
			fmt.Print("\n> ")
			continue
		}

		processQuery(chatAgent, query)
		fmt.Print("\n> ")
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
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
  Piped input:      echo "your query" | ./gpt-chat
  Help:             ./gpt-chat --help

EXAMPLES:
  ./gpt-chat
  > Create a simple Go HTTP server in server.go
  
  echo "Fix the bug in main.go where the variable is undefined" | ./gpt-chat

ENVIRONMENT:
  DEEPINFRA_API_KEY: Required API token for DeepInfra access

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
