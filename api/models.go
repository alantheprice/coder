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
	ID            string  `json:"id"`
	Name          string  `json:"name,omitempty"`
	Description   string  `json:"description,omitempty"`
	Provider      string  `json:"provider,omitempty"`
	Size          string  `json:"size,omitempty"`
	Cost          float64 `json:"cost,omitempty"`
	InputCost     float64 `json:"input_cost,omitempty"`
	OutputCost    float64 `json:"output_cost,omitempty"`
	ContextLength int     `json:"context_length,omitempty"`
	Tags          []string `json:"tags,omitempty"`
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

	client := &http.Client{Timeout: 60 * time.Second} // Increased from 30s to 60s
	
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
			ID       string `json:"id"`
			Object   string `json:"object"`
			Created  int64  `json:"created"`
			OwnedBy  string `json:"owned_by"`
			Metadata *struct {
				Description   string  `json:"description,omitempty"`
				ContextLength int     `json:"context_length,omitempty"`
				MaxTokens     int     `json:"max_tokens,omitempty"`
				Pricing       *struct {
					InputTokens     float64 `json:"input_tokens"`
					OutputTokens    float64 `json:"output_tokens"`
					CacheReadTokens float64 `json:"cache_read_tokens,omitempty"`
				} `json:"pricing,omitempty"`
				Tags []string `json:"tags,omitempty"`
			} `json:"metadata,omitempty"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	models := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		modelInfo := ModelInfo{
			ID:       model.ID,
			Provider: "DeepInfra",
		}
		
		// Extract metadata if available
		if model.Metadata != nil {
			modelInfo.Description = model.Metadata.Description
			modelInfo.ContextLength = model.Metadata.ContextLength
			modelInfo.Tags = model.Metadata.Tags
			
			// Extract pricing information
			if model.Metadata.Pricing != nil {
				modelInfo.InputCost = model.Metadata.Pricing.InputTokens
				modelInfo.OutputCost = model.Metadata.Pricing.OutputTokens
				// Use average of input/output for backward compatibility
				modelInfo.Cost = (model.Metadata.Pricing.InputTokens + model.Metadata.Pricing.OutputTokens) / 2.0
			}
		}
		
		models[i] = modelInfo
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