# Startup Guide

This guide walks a new developer through setting up and using the **GPT‑OSS Chat Agent** tool.

## Prerequisites
- **Go** (version 1.20 or later) installed on your machine.
- A **DeepInfra API key** (or compatible OpenAI key) to access the `gpt‑oss‑120b` model.
- Git for cloning the repository.

## Setup Steps
1. **Clone the repository**
   ```bash
   git clone https://github.com/alantheprice/coder.git
   cd coder
   ```

2. **Configure your API key**
   Export the key as an environment variable so the tool can authenticate:
   ```bash
   export DEEPINFRA_API_KEY="YOUR_TOKEN_HERE"
   ```

3. **Build the binary**
   ```bash
   go build -o gpt-chat .
   ```
   This compiles the `main.go` entry point and produces an executable named `gpt-chat`.

4. **Run the tool**
   - **Interactive mode** – the agent will prompt you for tasks:
     ```bash
     ./gpt-chat
     ```
   - **One‑off command** – pipe a request directly:
     ```bash
     echo "Create a simple HTTP server in Go" | ./gpt-chat
     ```

## Testing
The repository includes a test suite to verify the built‑in tools.
```bash
./test_tools.sh   # unit tests for read, write, edit, shell
./test_e2e.sh    # end‑to‑end integration test (requires API key)
```

## Common Issues
- **Missing API key** – ensure `DEEPINFRA_API_KEY` (or `OPENAI_API_KEY`) is set in your shell before running the tool.
- **Compilation errors** – confirm you are using a compatible Go version (`go version`).
- **Permission denied** – on Unix systems, you may need to make the binary executable (`chmod +x gpt-chat`).

## Contributing
If you want to contribute, fork the repo, create a feature branch, make your changes, and submit a pull request. See the main `README.md` for additional contribution guidelines.

---
*Happy hacking!*