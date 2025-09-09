package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Provider    string  `json:"provider,omitempty"`
	Size        string  `json:"size,omitempty"`
	Cost        float64 `json:"cost,omitempty"`
}

// ModelsListInterface defines methods for listing available models
type ModelsListInterface interface {
	ListAvailableModels() ([]ModelInfo, error)
	GetDefaultModel() string
	IsModelAvailable(modelID string) bool
}

// GetAvailableModels returns available models for the current provider
func GetAvailableModels() ([]ModelInfo, error) {
	clientType := GetClientTypeFromEnv()
	
	switch clientType {
	case DeepInfraClientType:
		return getDeepInfraModels()
	case OllamaClientType:
		return getOllamaModels()
	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// getDeepInfraModels gets available models from DeepInfra API
func getDeepInfraModels() ([]ModelInfo, error) {
	apiKey := os.Getenv("DEEPINFRA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DEEPINFRA_API_KEY not set")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequest("GET", "https://api.deepinfra.com/v1/openai/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepInfra API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	models := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		models[i] = ModelInfo{
			ID:       model.ID,
			Provider: "DeepInfra",
		}
		
		// Add special descriptions and current pricing for known models
		switch model.ID {
		case "openai/gpt-oss-120b":
			models[i].Description = "GPT-OSS 120B (default) - Uses harmony syntax - $0.30/1M input + $1.20/1M output"
			models[i].Cost = 0.30 // Input rate per 1M tokens (output rate shown in description)
		case "Qwen/Qwen3-Coder-480B-A35B-Instruct-Turbo":
			models[i].Description = "Qwen3 Coder 480B Turbo - Code generation specialist - $0.30/1M input + $1.20/1M output"
			models[i].Cost = 0.30 // Input rate per 1M tokens
		case "Qwen/Qwen3-235B-A22B-Instruct":
			models[i].Description = "Qwen3 235B - Large reasoning model - $0.13/1M input + $0.60/1M output"
			models[i].Cost = 0.13
		case "Qwen/QwQ-32B-Preview":
			models[i].Description = "QwQ 32B Preview - Reasoning focused - $0.15/1M input + $0.40/1M output"
			models[i].Cost = 0.15
		case "Qwen/Qwen3-32B-Instruct":
			models[i].Description = "Qwen3 32B - Balanced performance - $0.10/1M input + $0.30/1M output"
			models[i].Cost = 0.10
		case "meta-llama/Meta-Llama-3.1-70B-Instruct":
			models[i].Description = "Meta Llama 3.1 70B Instruct - Standard format"
			models[i].Cost = 0.36 // Legacy pricing format
		case "microsoft/WizardLM-2-8x22B":
			models[i].Description = "Microsoft WizardLM-2 8x22B - Enhanced reasoning"
			models[i].Cost = 0.63 // Legacy pricing format
		default:
			models[i].Description = "Available on DeepInfra - See deepinfra.com/pricing for current rates"
			models[i].Cost = 0.0 // Unknown pricing
		}
	}
	
	// Sort models alphabetically by ID
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			if models[i].ID > models[j].ID {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
	
	return models, nil
}

// getOllamaModels gets available models from local Ollama installation
func getOllamaModels() ([]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return nil, fmt.Errorf("Ollama is not running. Please start Ollama first")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (status %d)", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Models []struct {
			Name       string    `json:"name"`
			Size       int64     `json:"size"`
			Digest     string    `json:"digest"`
			ModifiedAt time.Time `json:"modified_at"`
			Details    struct {
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	models := make([]ModelInfo, len(response.Models))
	for i, model := range response.Models {
		sizeGB := float64(model.Size) / (1024 * 1024 * 1024)
		
		models[i] = ModelInfo{
			ID:       model.Name,
			Provider: "Ollama (Local)",
			Size:     fmt.Sprintf("%.1fGB", sizeGB),
			Cost:     0.0, // Local models are free
		}
		
		// Add descriptions for known models
		if model.Name == "gpt-oss:20b" || model.Name == "gpt-oss:latest" || model.Name == "gpt-oss" {
			models[i].Description = "GPT-OSS 20B - Local inference, free to use"
		} else {
			models[i].Description = fmt.Sprintf("Local %s model", model.Details.Family)
		}
	}
	
	return models, nil
}