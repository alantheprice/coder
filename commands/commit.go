package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alantheprice/coder/agent"
)

// CommitCommand implements the /commit slash command
type CommitCommand struct{}

// Name returns the command name
func (c *CommitCommand) Name() string {
	return "commit"
}

// Description returns the command description
func (c *CommitCommand) Description() string {
	return "Interactive commit workflow - select files and generate commit messages"
}

// Execute runs the commit command
func (c *CommitCommand) Execute(args []string, chatAgent *agent.Agent) error {
	fmt.Println("🚀 Starting interactive commit workflow...")
	fmt.Println("=============================================")

	// Step 1: Show current git status
	fmt.Println("📊 Current git status:")
	statusOutput, err := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get git status: %v", err)
	}

	if len(statusOutput) == 0 {
		fmt.Println("✅ No changes to commit")
		return nil
	}

	statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
	
	// Step 2: Show available files
	fmt.Println("\n📁 Modified files:")
	for i, line := range statusLines {
		if strings.TrimSpace(line) != "" {
			fmt.Printf("%2d. %s\n", i+1, line)
		}
	}

	// Step 3: Prompt user to select files
	fmt.Println("\n💡 Enter file numbers to commit (comma-separated, 'a' for all, 'q' to quit):")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "quit" {
		fmt.Println("❌ Commit cancelled")
		return nil
	}

	var filesToAdd []string
	
	if input == "a" || input == "all" {
		// Add all modified files
		for _, line := range statusLines {
			if strings.TrimSpace(line) != "" {
				// Extract filename from git status line (format: XY filename)
				parts := strings.SplitN(line, " ", 2)
				if len(parts) >= 2 {
					filesToAdd = append(filesToAdd, strings.TrimSpace(parts[1]))
				}
			}
		}
		fmt.Println("✅ Adding all modified files")
	} else {
		// Parse selected file numbers
		selections := strings.Split(input, ",")
		for _, sel := range selections {
			sel = strings.TrimSpace(sel)
			if sel == "" {
				continue
			}
			
			var index int
			_, err := fmt.Sscanf(sel, "%d", &index)
			if err != nil || index < 1 || index > len(statusLines) {
				fmt.Printf("❌ Invalid selection: %s\n", sel)
				continue
			}
			
			line := statusLines[index-1]
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 2 {
				filesToAdd = append(filesToAdd, strings.TrimSpace(parts[1]))
				fmt.Printf("✅ Adding: %s\n", strings.TrimSpace(parts[1]))
			}
		}
	}

	if len(filesToAdd) == 0 {
		fmt.Println("❌ No files selected")
		return nil
	}

	// Step 4: Stage the selected files
	fmt.Println("\n📦 Staging files...")
	for _, file := range filesToAdd {
		cmd := exec.Command("git", "add", file)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Failed to stage %s: %v\n", file, err)
			fmt.Printf("Output: %s\n", string(output))
		} else {
			fmt.Printf("✅ Staged: %s\n", file)
		}
	}

	// Step 5: Generate commit message from staged diff
	fmt.Println("\n📝 Generating commit message...")
	
	// Get staged diff
	diffOutput, err := exec.Command("git", "diff", "--staged").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get staged diff: %v", err)
	}

	if len(strings.TrimSpace(string(diffOutput))) == 0 {
		fmt.Println("❌ No changes staged")
		return nil
	}

	// Use the agent to generate a commit message
	commitPrompt := fmt.Sprintf(`Generate a concise commit message for the following staged changes. 

Requirements:
- Title: Maximum 120 characters, descriptive and concise
- Blank line after title
- Summary: 200 words or less, brief description of changes
- Focus on what changed and why, not how

Staged changes:
%s

Please generate only the commit message content, no additional commentary.`, string(diffOutput))

	fmt.Println("🤖 Generating commit message with AI...")
	commitMessage, err := chatAgent.ProcessQuery(commitPrompt)
	if err != nil {
		return fmt.Errorf("failed to generate commit message: %v", err)
	}

	// Clean up the commit message
	commitMessage = strings.TrimSpace(commitMessage)
	
	// Step 6: Show preview and confirm
	fmt.Println("\n📋 Commit message preview:")
	fmt.Println("=============================================")
	fmt.Println(commitMessage)
	fmt.Println("=============================================")

	fmt.Println("\n💡 Commit with this message? (y/n):")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Println("❌ Commit cancelled")
		return nil
	}

	// Step 7: Create the commit
	fmt.Println("\n💾 Creating commit...")
	
	// Write commit message to temporary file
	tempFile := "commit_msg.txt"
	err = os.WriteFile(tempFile, []byte(commitMessage), 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary commit message file: %v", err)
	}
	defer os.Remove(tempFile)

	cmd := exec.Command("git", "commit", "-F", tempFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create commit: %v", err)
	}

	fmt.Printf("✅ Commit created successfully!\n")
	fmt.Printf("Output: %s\n", string(output))

	return nil
}