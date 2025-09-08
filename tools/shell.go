package tools

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func ExecuteShellCommand(command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("empty command provided")
	}

	// Create command with timeout
	cmd := exec.Command("bash", "-c", command)

	// Set up timeout
	timeout := 30 * time.Second

	done := make(chan error, 1)
	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			// Check if it's an exit error (command ran but failed)
			if exitError, ok := err.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					return string(output), fmt.Errorf("command failed with exit code %d: %s", status.ExitStatus(), string(output))
				}
			}
			return string(output), fmt.Errorf("command failed: %w", err)
		}
		return string(output), nil
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
}
