# GPT-OSS Chat Agent Architecture

## Overview
A minimal command-line chat application that uses OpenAI's gpt-oss-120b model via DeepInfra to provide an autonomous coding assistant with 4 core tools.

## Core Components

### 1. Main Application (`main.go`)
- Command-line interface for user interaction
- Chat session management
- API communication with DeepInfra
- Tool execution coordination

### 2. API Client (`api/client.go`)
- HTTP client for DeepInfra API
- OpenAI-compatible chat completions format
- Request/response handling
- Error management

### 3. Tool System (`tools/`)
- **Shell Commands** (`shell.go`): Execute system commands
- **Read File** (`read.go`): Read file contents
- **Edit File** (`edit.go`): Modify existing files with precise string replacement
- **Write File** (`write.go`): Create new files or overwrite existing ones

### 4. Agent Logic (`agent/agent.go`)
- System prompt loading from `systematic_exploration_prompt.md`
- Tool call parsing and execution
- Iterative problem-solving workflow
- Response processing and continuation logic

## Data Flow

1. **User Input** → Main application receives user query
2. **System Prompt** → Load systematic exploration prompt
3. **API Request** → Send to DeepInfra with chat history and tool definitions
4. **Tool Calls** → Parse and execute tool calls from model response
5. **Tool Results** → Add results back to chat history
6. **Iteration** → Continue until problem is solved
7. **Response** → Display final answer to user

## Tool Definitions (JSON Schema)

### Shell Command
```json
{
  "name": "shell_command",
  "description": "Execute shell commands to explore directory structure, search files, run programs",
  "parameters": {
    "type": "object",
    "properties": {
      "command": {"type": "string", "description": "Shell command to execute"}
    },
    "required": ["command"]
  }
}
```

### Read File
```json
{
  "name": "read_file", 
  "description": "Read contents of a specific file",
  "parameters": {
    "type": "object",
    "properties": {
      "file_path": {"type": "string", "description": "Path to file to read"}
    },
    "required": ["file_path"]
  }
}
```

### Edit File
```json
{
  "name": "edit_file",
  "description": "Edit existing file by replacing old string with new string",
  "parameters": {
    "type": "object", 
    "properties": {
      "file_path": {"type": "string", "description": "Path to file to edit"},
      "old_string": {"type": "string", "description": "Exact string to replace"},
      "new_string": {"type": "string", "description": "New string to replace with"}
    },
    "required": ["file_path", "old_string", "new_string"]
  }
}
```

### Write File
```json
{
  "name": "write_file",
  "description": "Write content to a new file or overwrite existing file",
  "parameters": {
    "type": "object",
    "properties": {
      "file_path": {"type": "string", "description": "Path to file to write"},
      "content": {"type": "string", "description": "Content to write to file"}
    },
    "required": ["file_path", "content"]
  }
}
```

## Key Features

- **Autonomous Operation**: Agent continues until problem is completely solved
- **Systematic Exploration**: Follows structured approach from system prompt
- **Tool Integration**: Native support for 4 essential coding tools
- **OpenAI Compatibility**: Uses standard chat completions format
- **Minimal Dependencies**: Simple Go implementation with standard library

## Configuration

- **Model**: `openai/gpt-oss-120b` via DeepInfra
- **Endpoint**: `https://api.deepinfra.com/v1/openai/chat/completions`
- **Authentication**: Bearer token via `DEEPINFRA_API_KEY` environment variable
- **System Prompt**: Loaded from `systematic_exploration_prompt.md`

## Error Handling

- API failures with retry logic
- Tool execution errors with proper messaging
- File operation errors with user feedback
- Network connectivity issues

## Project Structure
```
.
├── main.go                          # Main application entry point
├── api/
│   └── client.go                    # DeepInfra API client
├── tools/
│   ├── shell.go                     # Shell command execution
│   ├── read.go                      # File reading
│   ├── edit.go                      # File editing
│   └── write.go                     # File writing
├── agent/
│   └── agent.go                     # Agent logic and tool coordination
├── go.mod                           # Go module definition
├── systematic_exploration_prompt.md # System prompt
├── test_e2e.sh                     # End-to-end test script
└── ARCHITECTURE.md                  # This document
```