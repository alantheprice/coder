#!/bin/bash

# Simple tool testing without API calls
echo "🧪 Testing individual tools..."

cd /Users/alanp/dev/personal/gpt-oss-test

# Create a simple Go test program to test tools directly
cat > tool_tester.go << 'EOF'
package main

import (
	"fmt"
	"gpt-chat/tools"
	"os"
)

func main() {
	// Test 1: Shell command
	fmt.Println("=== Testing Shell Command ===")
	result, err := tools.ExecuteShellCommand("echo 'Hello from shell'")
	if err != nil {
		fmt.Printf("Shell command error: %v\n", err)
	} else {
		fmt.Printf("Shell result: %s\n", result)
	}
	
	// Test 2: Write file
	fmt.Println("\n=== Testing Write File ===")
	result, err = tools.WriteFile("test_file.txt", "Hello, World!\nThis is a test file.")
	if err != nil {
		fmt.Printf("Write file error: %v\n", err)
	} else {
		fmt.Printf("Write result: %s\n", result)
	}
	
	// Test 3: Read file
	fmt.Println("\n=== Testing Read File ===")
	content, err := tools.ReadFile("test_file.txt")
	if err != nil {
		fmt.Printf("Read file error: %v\n", err)
	} else {
		fmt.Printf("Read result: %s\n", content)
	}
	
	// Test 4: Edit file
	fmt.Println("\n=== Testing Edit File ===")
	result, err = tools.EditFile("test_file.txt", "Hello, World!", "Hello, GPT-OSS!")
	if err != nil {
		fmt.Printf("Edit file error: %v\n", err)
	} else {
		fmt.Printf("Edit result: %s\n", result)
	}
	
	// Verify edit worked
	fmt.Println("\n=== Verifying Edit ===")
	content, err = tools.ReadFile("test_file.txt")
	if err != nil {
		fmt.Printf("Read after edit error: %v\n", err)
	} else {
		fmt.Printf("Final content: %s\n", content)
	}
	
	// Cleanup
	os.Remove("test_file.txt")
	fmt.Println("\n✅ All tool tests completed!")
}
EOF

# Run the tool test
echo "Running tool tests..."
go run tool_tester.go

# Cleanup
rm tool_tester.go

echo "✅ Tool testing completed!"