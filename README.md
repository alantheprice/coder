# GPT-OSS Chat Agent

A minimal command‑line coding assistant that uses the OpenAI **gpt‑oss‑120b** model via DeepInfra. It provides four core tools (shell, read, edit, write) and follows a systematic exploration workflow to autonomously solve programming tasks.

## Features
- **Autonomous operation** – continues iterating until a problem is solved.
- **Four built‑in tools**: `shell_command`, `read_file`, `edit_file`, `write_file`.
- **Systematic exploration** – guided by `systematic_exploration_prompt.md`.
- **Zero external dependencies** – pure Go standard library.
- **Test suite** – `./test_tools.sh` and `./test_e2e.sh`.

## Project Structure
```
.
├── main.go               # CLI entry point
├── api/
│   └── client.go        # DeepInfra API client
├── agent/
│   └── agent.go         # Core agent logic
├── tools/
│   ├── shell.go         # Execute shell commands
│   ├── read.go          # Read file contents
│   ├── edit.go          # Edit file via string replacement
│   └── write.go         # Create/overwrite files
├── demo/                 # Example server implementation and tests
│   ├── server.go
│   └── server_test.go
├── systematic_exploration_prompt.md  # System prompt for the agent
├── ARCHITECTURE.md       # Technical overview
├── README.md             # This file
├── go.mod
├── test_tools.sh         # Unit tests for the tools
├── test_e2e.sh          # End‑to‑end integration test
└── validate.sh           # Helper validation script
```

## Installation
```bash
# Clone the repository
git clone https://github.com/your-org/gpt-oss-chat-agent.git
cd gpt-oss-chat-agent

# Set your DeepInfra API key
export DEEPINFRA_API_KEY="YOUR_TOKEN_HERE"

# Build the binary
go build -o gpt-chat .
```

## Quick Usage
```bash
# Interactive mode – the agent will prompt for tasks
./gpt-chat

# One‑off command (pipe the request)
echo "Create a simple HTTP server in Go" | ./gpt-chat
```

## Available Tools (used internally by the agent)
- **shell_command** – run arbitrary shell commands.
- **read_file** – read the contents of a file.
- **edit_file** – replace an exact string with another string.
- **write_file** – create or overwrite a file with given content.

## Testing
```bash
# Test individual tool implementations
./test_tools.sh

# Run the full end‑to‑end test suite (requires DEEPINFRA_API_KEY)
./test_e2e.sh
```

## Contributing
Contributions are welcome! Please fork the repo, create a feature branch, make your changes, and submit a pull request. Ensure that any new code is covered by tests.

## License
This project is licensed under the Apache 2.0 License.
