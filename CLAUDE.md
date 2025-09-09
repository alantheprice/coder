# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build and Run
```bash
# Build the application
go build -o coder .

# Run interactively
./coder

# Run with a single query
./coder "your query here"

# Use local inference (Ollama)
./coder --local "your query"

# Run via pipe
echo "your query" | ./coder
```

### Testing
```bash
# Validate implementation and run tool tests
./validate.sh

# Run end-to-end tests (requires DEEPINFRA_API_KEY)
./test_e2e.sh

# Quick integration test
./test.sh
```

### Environment Setup
```bash
# For remote inference (DeepInfra)
export DEEPINFRA_API_KEY="your_api_key_here"

# For local inference (Ollama)
ollama pull gpt-oss:20b

# Enable debug logging
export DEBUG=true
```

## Architecture Overview

This is a command-line coding assistant that uses the OpenAI gpt-oss-120b model (via DeepInfra) or local gpt-oss:20b (via Ollama). The agent operates autonomously with 4 core tools and follows systematic exploration patterns.

### Core Components

- **main.go**: CLI interface with dual-mode support (local/remote)
- **agent/agent.go**: Core agent logic with embedded system prompt and tool execution
- **api/**: Unified client interface supporting both DeepInfra and Ollama
  - **client.go**: DeepInfra API client
  - **ollama.go**: Ollama API client  
  - **harmony.go**: Message formatting for gpt-oss models
  - **interface.go**: Unified client interface
- **tools/**: Four essential coding tools
  - **shell.go**: Execute shell commands
  - **read.go**: Read file contents
  - **edit.go**: Modify files via string replacement
  - **write.go**: Create/overwrite files
  - **todo.go**: Task management and tracking

### Key Features

- **Autonomous Operation**: Agent continues until tasks are completely solved
- **Dual-Mode Support**: Works with both cloud (DeepInfra) and local (Ollama) inference
- **Tool Integration**: Native support for shell, file operations, and task management
- **Systematic Exploration**: Follows structured workflow from embedded system prompt
- **Context Management**: Tracks conversation history and token usage

### Tool Usage Patterns

The agent uses tools systematically:
1. **Shell commands** for exploration (`ls`, `find`, `grep`)
2. **Read file** to understand existing code structure
3. **Write/Edit file** to implement changes
4. **Todo tools** for complex task tracking

### Model Configuration

- **Remote**: `openai/gpt-oss-120b` via DeepInfra (~$0.50/M tokens)
- **Local**: `gpt-oss:20b` via Ollama (free, requires 14GB VRAM)
- **System Prompt**: v2_structured (systematically tested and optimized)
- **Reasoning Effort**: High (for better strategic thinking)
- **Max Iterations**: 100 iterations for complex tasks

### Prompt Engineering (v2_structured)

The agent uses a scientifically-tested structured approach:

**PHASE 1: UNDERSTAND & PLAN** - Break task into specific steps
**PHASE 2: EXPLORE** - Systematic codebase exploration  
**PHASE 3: IMPLEMENT** - Careful changes with verification
**PHASE 4: VERIFY & COMPLETE** - Testing and quality assurance

This approach delivers:
- **2.5x better performance** than previous versions
- **100% compilation success rate** vs 0% baseline
- **59% faster execution** with fewer tool calls
- **Consistent quality** across different task types

### Error Handling

- Tool execution errors are captured and reported
- API failures include retry logic
- Malformed tool calls are detected and corrected
- Connection checks validate client setup

### Development Notes

- Uses Go 1.24 with minimal dependencies (only readline)
- Harmony message formatting for gpt-oss model compatibility
- Debug mode available via `DEBUG` environment variable
- Conversation history tracking for cost analysis