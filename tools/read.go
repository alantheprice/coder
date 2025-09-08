package tools

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ReadFile(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("empty file path provided")
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(filePath)

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", cleanPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to access file %s: %w", cleanPath, err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", cleanPath)
	}

	// Check file size (limit to reasonable size for text files)
	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file too large (>10MB): %s", cleanPath)
	}

	// Open and read the file
	file, err := os.Open(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", cleanPath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	return string(content), nil
}
