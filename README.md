# Coder - Autonomous Coding Assistant

A command-line coding assistant written in Go that uses OpenAI's gpt-oss-120b model (via DeepInfra) or local gpt-oss:20b (via Ollama). The agent operates autonomously with core tools and follows systematic exploration patterns.

## Features

- **Autonomous operation** - continues iterating until tasks are completely solved
- **7 built-in tools** - comprehensive development toolkit
- **Multi-model support** - works with local Ollama, DeepInfra, and other providers
- **Systematic exploration** - follows structured phases for problem-solving
- **Todo management** - built-in project/task tracking system
- **Real-time feedback** - displays progress, costs, and iteration summaries
- **Interactive & non-interactive modes** - supports both CLI usage patterns
- **Conversation continuity** - maintains state across sessions

## Current Tools Available

| Tool | Description | Usage
|------|-------------|-------
| **shell_command** | Execute shell commands for exploration, testing, and operations | System commands, directory exploration, build testing
| **read_file** | Read file contents | Code analysis, configuration inspection, documentation review
| **write_file** | Create new files or overwrite existing | Code creation, documentation, configuration files
| **edit_file** | Modify existing files with precise string replacement | Refactoring, bug fixes, updates
| **add_todo** | Create and track development tasks | Project management, task planning
| **update_todo_status** | Update progress on tracked tasks | Progress tracking, completion management
| **list_todos** | View all current tasks and their status | Task review, sprint management

## Supported Models & Providers

### Local Options (FREE)
- **Ollama**: gpt-oss:20b (14GB VRAM required)
- Local inference with zero cloud costs

### Cloud Options (via DeepInfra)
- **openai/gpt-oss-120b** (default) - Uses harmony syntax
- **meta-llama/Meta-Llama-3.1-70B-Instruct** - Standard format
- **microsoft/WizardLM-2-8x22B** - Enhanced reasoning
- **anthropic/claude-3-haiku** - Claude compatibility
- **google/gemini-flash** - Gemini integration
- Many other OpenAI-compatible models

## Installation

### Prerequisites
- Go 1.19 or later
- For local inference: Ollama installed with `ollama pull gpt-oss:20b`

### Quick Setup
```bash
# Clone repository
git clone https://github.com/alantheprice/coder.git
cd coder

# Build
go build -o coder

# Set API key for cloud (optional - local mode works without)
export DEEPINFRA_API_KEY="your_api_key_here"

# Verify installation
./coder --help
```

## Quick Usage

### Interactive Mode (Recommended)
```bash
./coder
# Type your query and interact naturally
> Create a Go REST API with CRUD endpoints
> Fix the bug in main.go and add unit tests
> Refactor utils.go to use idiomatic Go patterns
```

### Non-Interactive Mode
```bash
# Single command execution
echo "Create a simple HTTP server" | ./coder

# Direct query
./coder "Implement a binary search tree in Go"

# Piped input
cat requirements.txt | ./coder
```

### Local vs Cloud Selection
```bash
# Force local inference (Ollama)
./coder --local "your task here"

# Use specific cloud model
./coder --model=meta-llama/Meta-Llama-3.1-70B-Instruct "create a calculator"

# Interactive model selection
./coder
> /models select
```

### Slash Commands (Interactive Mode)
```bash
/models              # View and switch models
/help               # Show detailed help
/models select      # Interactive model picker
exit                # End session
```

## Testing & Validation

The project includes comprehensive testing across multiple dimensions:

```bash
# End-to-end integration tests
./test_e2e.sh

# Cost tracking tests
./test_actual_costs.sh

# Cache tracking tests
./test_cache_tracking.sh

# Continuity tests
./test_continuity.sh

# Manual validation
./validate.sh
```

## Project Structure

```
coder/
├── main.go                          # CLI entry point with argument parsing
├── agent/                           # Core agent logic and orchestration
│   ├── agent.go                     # Main agent implementation
│   └── persistence.go               # State persistence for continuity
├── api/                             # API client abstractions
│   ├── client.go                    # Generic API client interface
│   ├── ollama.go                    # Ollama local client
│   ├── harmony.go                   # GPT-OSS harmony support
│   ├── models.go                    # Model registry and selection
│   └── interface.go                 # Common client interface
├── tools/                           # Core development tools
│   ├── shell.go                     # System command execution
│   ├── read.go                      # File reading functionality
│   ├── write.go                     # File creation
│   ├── edit.go                      # Precise file modification
│   └── todo.go                      # Task tracking system
├── commands/                        # Slash command system
│   ├── commands.go                  # Command registry and handling
│   ├── continuity.go                # Continuity command
│   ├── help.go                      # Help command
│   └── models.go                    # Models command
├── test_environment/                # Comprehensive test scenarios
│   ├── baseline_files/              # Reference implementations
│   ├── work_scenario_*/             # Test workspaces
│   └── validation scripts
├── test_e2e.sh                      # Integration test suite
├── test_actual_costs.sh             # Cost tracking tests
├── test_cache_tracking.sh           # Cache tracking tests
├── test_continuity.sh               # Continuity tests
├── validate.sh                      # Code validation helper
└── [various documentation].md       # Architecture docs
```

## Development Workflow

### Systematic Process
The agent follows these phases for every task:

1. **PHASE 1: UNDERSTAND & PLAN**
   - Parses user requirements
   - Breaks tasks into measurable steps
   - Identifies necessary files and modifications

2. **PHASE 2: EXPLORE**
   - Uses shell commands to explore workspace
   - Reads relevant code files
   - Documents findings and current state

3. **PHASE 3: IMPLEMENT**
   - Creates/modifies files using precise tools
   - Validates changes through testing
   - Iterates based on feedback and requirements

4. **PHASE 4: VERIFY & COMPLETE**
   - Confirms all requirements are met
   - Tests the complete solution
   - Provides comprehensive completion summary

### Real-time Features
- **Progress tracking**: Shows current iteration, tokens used, cost
- **Live diffs**: Displays file changes with colored diff output  
- **Task summary**: Lists all actions taken during the session
- **Cost transparency**: Shows exact token usage and costs
- **Cached token tracking**: Shows cost savings from cached tokens
- **Continuity support**: Maintains conversation state across sessions

## Environment Configuration

### Environment Variables
```bash
# API Keys
DEEPINFRA_API_KEY="your_key_here"
OLLAMA_HOST="http://localhost:11434"  # Custom Ollama location

# Debug Mode
DEBUG=1                    # Enable verbose logging
DEBUG=true                 # Alternative debug flag

# Model Selection (runtime overrides)
MODEL="openai/gpt-oss-120b"  # Specific model to use
```

### Custom Configuration
```bash
# Create symbolic link for global access
sudo ln -s $(pwd)/coder /usr/local/bin/coder

# Usage from anywhere
coder "create new project in /tmp/newproject"

# Custom model via env
export MODEL="meta-llama/Meta-Llama-3.1-405B-Instruct"
coder "complex refactoring task"
```

## Examples

### File Creation
```bash
./coder "Create a todo API with PostgreSQL backend"
# Creates: models/todo.go, handlers/todo.go, main.go, migrations/
```

### Code Refactoring
```bash
./coder "Refactor main.go to use clean architecture"
# Analyzes current structure, implements separation of concerns
```

### Bug Fixing
```bash
./coder "Fix the nil pointer in userService.Authenticate()"
# Uses read_file to examine the bug, edit_file to fix precisely
```

### Testing
```bash
./coder "Add comprehensive tests for the user service"
# Creates *_test.go files, uses go test to validate
```

### Documentation
```bash
./coder "Write API documentation for the todo service"
# Generates README_API.md for all endpoints and schemas
```

## Performance & Cost

### Token Efficiency
- **Input tokens**: Prompt context + conversation history
- **Output tokens**: Response + tool result integration
- **Total tracking**: Real-time cost calculation with DeepInfra rates
- **Cached tokens**: Tracks token reuse for cost savings

### Local vs Cloud
| Mode | Speed | Cost | VRAM | Features |
|------|-------|------|------|----------|
| Ollama | Medium | Free | 14GB | Local files only |
| DeepInfra | Fast | Pay per use | 0GB | Full cloud features |

## Troubleshooting

### Common Issues

**Connection Errors**
```bash
# Check Ollama
ollama pull gpt-oss:20b
ollama list  # Verify gpt-oss:20b is available

# Check API key
export DEEPINFRA_API_KEY="correct_key"
./coder --model=openai/gpt-oss-120b --help
```

**Tool Errors**
- Verify workspace permissions (`ls -la` for current directory)
- Check file exists before editing (`read_file` before `edit_file`)
- Ensure Go development environment (for `go build` testing)

**Debug Mode**
```bash
DEBUG=1 ./coder "your complex task"
# Shows all API calls, tool executions, and debugging info
```

## Contributing

We welcome contributions! The project's systematic approach makes it easy to:

1. **Add new tools** - Extend `tools/` directory following existing patterns
2. **Support new models** - Add client implementations in `api/` 
3. **Enhance system prompt** - Improve embedded prompts in `agent/agent.go`
4. **Add test scenarios** - Create new test cases in `test_environment/`

### Development Setup
```bash
git clone https://github.com/alantheprice/coder.git
cd coder
go mod tidy
go build -o coder

# Run comprehensive test suite
./test_e2e.sh
```

## License
Apache 2.0 License - See LICENSE file for details.

## Changelog
- **v2.1** - Added conversation continuity, enhanced cost tracking
- **v2.0** - Added multi-model support, todo system, comprehensive testing
- **v1.2** - Enhanced interactive mode with slash commands
- **v1.1** - Added Ollama local support
- **v1.0** - Initial stable release with 4 core tools