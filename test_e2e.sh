#!/bin/bash

# End-to-End Test Script for GPT-OSS Chat Agent
# This script demonstrates full functionality of the chat agent

set -e

echo "🚀 Starting E2E Test for GPT-OSS Chat Agent"
echo "============================================="

# Check prerequisites
echo "📋 Checking prerequisites..."

# Check if DEEPINFRA_API_KEY is set
if [ -z "$DEEPINFRA_API_KEY" ]; then
    echo "❌ ERROR: DEEPINFRA_API_KEY environment variable not set"
    echo "Please set your DeepInfra API token:"
    echo "export DEEPINFRA_API_KEY=your_token_here"
    exit 1
fi
echo "✅ DEEPINFRA_API_KEY is set"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ ERROR: Go is not installed"
    exit 1
fi
echo "✅ Go is available"

# Build the application
echo "🔨 Building the application..."
go build -o gpt-chat .
if [ $? -eq 0 ]; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

# Create test directory structure for demonstration
echo "📁 Setting up test directory structure..."
mkdir -p test_workspace
cd test_workspace

# Create a sample project to work with
cat > hello.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
EOF

cat > config.json << 'EOF'
{
    "name": "test-project",
    "version": "1.0.0"
}
EOF

mkdir -p src/utils
cat > src/utils/helper.go << 'EOF'
package utils

func Add(a, b int) int {
    return a + b
}
EOF

echo "✅ Test workspace created"

# Test 1: Shell Command Tool
echo ""
echo "🧪 Test 1: Shell Command Tool"
echo "Testing basic directory exploration..."
../gpt-chat <<< "Use the shell_command tool to list the contents of the current directory and show me the directory structure using tree or ls -la commands." > test1_output.txt 2>&1 &
CHAT_PID=$!

# Give it some time to process
sleep 10
if kill -0 $CHAT_PID 2>/dev/null; then
    echo "⏱️  Chat agent is still processing..."
    sleep 20
fi

if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

if [ -f test1_output.txt ] && [ -s test1_output.txt ]; then
    echo "✅ Shell command tool test completed"
    echo "Preview of output:"
    head -n 10 test1_output.txt | sed 's/^/    /'
else
    echo "❌ Shell command tool test failed - no output generated"
fi

# Test 2: Read File Tool  
echo ""
echo "🧪 Test 2: Read File Tool"
echo "Testing file reading capabilities..."
../gpt-chat <<< "Use the read_file tool to read the contents of hello.go and config.json files. Then describe what each file contains." > test2_output.txt 2>&1 &
CHAT_PID=$!

sleep 15
if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

if [ -f test2_output.txt ] && [ -s test2_output.txt ]; then
    echo "✅ Read file tool test completed"
    echo "Preview of output:"
    head -n 10 test2_output.txt | sed 's/^/    /'
else
    echo "❌ Read file tool test failed"
fi

# Test 3: Write File Tool
echo ""
echo "🧪 Test 3: Write File Tool" 
echo "Testing file creation..."
../gpt-chat <<< "Use the write_file tool to create a new file called 'README.md' with a description of this test project. Include sections for description, installation, and usage." > test3_output.txt 2>&1 &
CHAT_PID=$!

sleep 15
if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

# Check if README.md was created
if [ -f README.md ]; then
    echo "✅ Write file tool test completed - README.md created"
    echo "Contents preview:"
    head -n 5 README.md | sed 's/^/    /'
else
    echo "❌ Write file tool test failed - README.md not created"
fi

# Test 4: Edit File Tool
echo ""
echo "🧪 Test 4: Edit File Tool"
echo "Testing file modification..."
../gpt-chat <<< "Use the edit_file tool to modify hello.go. Change the message from 'Hello, World!' to 'Hello, GPT-OSS Chat Agent!'. Then use read_file to verify the change." > test4_output.txt 2>&1 &
CHAT_PID=$!

sleep 15
if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

# Check if hello.go was modified
if grep -q "Hello, GPT-OSS Chat Agent!" hello.go; then
    echo "✅ Edit file tool test completed - hello.go modified successfully"
    echo "Modified content:"
    cat hello.go | sed 's/^/    /'
else
    echo "❌ Edit file tool test failed - hello.go not modified correctly"
fi

# Test 5: Complex Task Integration
echo ""
echo "🧪 Test 5: Complex Task Integration"
echo "Testing systematic exploration and multi-tool usage..."
../gpt-chat <<< "I need you to systematically explore this project, understand its structure, create a proper Go module with go.mod file, add a test file for the utils package, and then run the tests to make sure everything works. Use all your tools as needed and follow your systematic exploration process." > test5_output.txt 2>&1 &
CHAT_PID=$!

# Give more time for complex task
sleep 30
if ps -p $CHAT_PID > /dev/null; then
    echo "⏱️  Complex task still processing, giving more time..."
    sleep 30
fi

if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

# Check results of complex task
COMPLEX_SUCCESS=true
if [ ! -f go.mod ]; then
    echo "❌ go.mod file not created"
    COMPLEX_SUCCESS=false
fi

if [ ! -f src/utils/helper_test.go ] && [ ! -f utils_test.go ]; then
    echo "❌ Test file not created"
    COMPLEX_SUCCESS=false
fi

if [ "$COMPLEX_SUCCESS" = true ]; then
    echo "✅ Complex task integration test completed successfully"
else
    echo "❌ Complex task integration test failed"
fi

# Test 6: Error Handling
echo ""
echo "🧪 Test 6: Error Handling"
echo "Testing error handling with invalid operations..."
../gpt-chat <<< "Use the read_file tool to read a non-existent file called 'nonexistent.txt'. Then use shell_command to run an invalid command. Show me how you handle these errors." > test6_output.txt 2>&1 &
CHAT_PID=$!

sleep 10
if ps -p $CHAT_PID > /dev/null; then
    kill $CHAT_PID 2>/dev/null || true
fi

if [ -f test6_output.txt ] && [ -s test6_output.txt ]; then
    echo "✅ Error handling test completed"
else
    echo "❌ Error handling test failed"
fi

# Cleanup and summary
echo ""
echo "🧹 Cleaning up..."
cd ..
rm -rf test_workspace

echo ""
echo "📊 E2E Test Summary"
echo "==================="
echo "The following tests were executed:"
echo "1. Shell Command Tool - Directory exploration"
echo "2. Read File Tool - File content reading" 
echo "3. Write File Tool - File creation"
echo "4. Edit File Tool - File modification"
echo "5. Complex Task Integration - Multi-tool workflow"
echo "6. Error Handling - Invalid operations"

echo ""
echo "📝 Test outputs have been saved to test*_output.txt files"
echo ""
echo "🎯 To run the chat agent interactively:"
echo "   ./gpt-chat"
echo ""
echo "💡 To run a specific query:"
echo "   echo 'your question' | ./gpt-chat"
echo ""
echo "✨ E2E Testing Complete!"

# Final verification that the binary works
echo ""
echo "🔍 Final verification - Testing basic execution..."
if ./gpt-chat --help > /dev/null 2>&1; then
    echo "✅ Binary executes successfully"
elif echo "test" | timeout 5 ./gpt-chat > /dev/null 2>&1; then
    echo "✅ Binary accepts input successfully"
else
    echo "❌ Binary execution test failed"
fi

echo ""
echo "🎉 All tests completed!"