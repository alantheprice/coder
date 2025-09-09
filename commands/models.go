package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alantheprice/coder/agent"
	"github.com/alantheprice/coder/api"
)

// ModelsCommand implements the /models slash command
type ModelsCommand struct{}

// Name returns the command name
func (m *ModelsCommand) Name() string {
	return "models"
}

// Description returns the command description
func (m *ModelsCommand) Description() string {
	return "List available models and select which model to use"
}

// Execute runs the models command
func (m *ModelsCommand) Execute(args []string, chatAgent *agent.Agent) error {
	// If no arguments, list available models
	if len(args) == 0 {
		return m.listModels()
	}

	// If arguments provided, handle model selection
	if len(args) == 1 {
		if args[0] == "select" {
			return m.selectModel(chatAgent)
		} else {
			// Direct model selection by ID
			return m.setModel(args[0], chatAgent)
		}
	}

	return fmt.Errorf("usage: /models [select|<model_id>]")
}

// listModels displays all available models
func (m *ModelsCommand) listModels() error {
	fmt.Println("\n📋 Available Models:")
	fmt.Println("====================")

	models, err := api.GetAvailableModels()
	if err != nil {
		return fmt.Errorf("failed to get available models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models available.")
		return nil
	}

	// Get current provider info
	clientType := api.GetClientTypeFromEnv()
	fmt.Printf("Provider: %s\n\n", clientType)

	// Sort models alphabetically by model ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	// Identify featured models
	featuredIndices := m.findFeaturedModels(models)

	// Display all models
	for i, model := range models {
		fmt.Printf("%d. %s\n", i+1, model.ID)
		if model.Description != "" {
			fmt.Printf("   Description: %s\n", model.Description)
		}
		if model.Size != "" {
			fmt.Printf("   Size: %s\n", model.Size)
		}
		if model.InputCost > 0 || model.OutputCost > 0 {
			if model.InputCost > 0 && model.OutputCost > 0 {
				fmt.Printf("   Cost: $%.3f/M input, $%.3f/M output tokens\n", model.InputCost, model.OutputCost)
			} else if model.Cost > 0 {
				// Fallback to legacy format
				fmt.Printf("   Cost: ~$%.2f/M tokens\n", model.Cost)
			}
		} else if model.Provider == "Ollama (Local)" {
			fmt.Printf("   Cost: FREE (local)\n")
		} else {
			fmt.Printf("   Cost: N/A\n")
		}
		if model.ContextLength > 0 {
			fmt.Printf("   Context: %d tokens\n", model.ContextLength)
		}
		if len(model.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(model.Tags, ", "))
		}
		fmt.Println()
	}

	// Display featured models section
	if len(featuredIndices) > 0 {
		fmt.Println("⭐ Featured Models (Popular & High Performance):")
		fmt.Println("================================================")
		for _, idx := range featuredIndices {
			model := models[idx]
			fmt.Printf("%d. %s", idx+1, model.ID)
			if model.InputCost > 0 && model.OutputCost > 0 {
				fmt.Printf(" - $%.3f/$%.3f per M tokens", model.InputCost, model.OutputCost)
			} else if model.Cost > 0 {
				fmt.Printf(" - ~$%.2f/M tokens", model.Cost)
			} else if model.Provider == "Ollama (Local)" {
				fmt.Printf(" - FREE")
			} else {
				fmt.Printf(" - N/A")
			}
			if model.ContextLength > 0 {
				fmt.Printf(" - %dK context", model.ContextLength/1000)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("Usage:")
	fmt.Println("  /models select          - Interactive model selection")
	fmt.Println("  /models <model_id>      - Set model directly")
	fmt.Println("  /models                 - Show this list")

	return nil
}

// findFeaturedModels identifies indices of featured models
func (m *ModelsCommand) findFeaturedModels(models []api.ModelInfo) []int {
	featuredPatterns := []string{
		"openai/gpt-oss",
		"deepseek-ai/deepseek-v3.1",
		"qwen/qwen3-coder",
		"qwen/qwen3-235b",
		"mistralai/devstral",
		"moonshotai/kimi-k2",
		"google/gemini-2.5-pro",
	}
	
	var featured []int
	for i, model := range models {
		modelLower := strings.ToLower(model.ID)
		for _, pattern := range featuredPatterns {
			if strings.Contains(modelLower, strings.ToLower(pattern)) {
				featured = append(featured, i)
				break
			}
		}
	}
	
	return featured
}

// selectModel allows interactive model selection
func (m *ModelsCommand) selectModel(chatAgent *agent.Agent) error {
	models, err := api.GetAvailableModels()
	if err != nil {
		return fmt.Errorf("failed to get available models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models available.")
		return nil
	}

	// Sort models alphabetically by model ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	// Identify featured models
	featuredIndices := m.findFeaturedModels(models)

	fmt.Println("\n🎯 Select a Model:")
	fmt.Println("==================")

	fmt.Println("All Models:")
	fmt.Println("===========")
	// Display all models with numbers
	for i, model := range models {
		fmt.Printf("%d. \x1b[34m%s\x1b[0m", i+1, model.ID)
		if model.InputCost > 0 && model.OutputCost > 0 {
			fmt.Printf(" - $%.3f/$%.3f per M tokens", model.InputCost, model.OutputCost)
		} else if model.Cost > 0 {
			fmt.Printf(" - ~$%.2f/M tokens", model.Cost)
		} else if model.Provider == "Ollama (Local)" {
			fmt.Printf(" - FREE")
		} else {
			fmt.Printf(" - N/A")
		}
		fmt.Println()
	}

	// Display featured models at the end if any exist
	if len(featuredIndices) > 0 {
		fmt.Println("\n⭐ Featured Models (Popular & High Performance):")
		fmt.Println("================================================")
		for _, idx := range featuredIndices {
			model := models[idx]
			fmt.Printf("%d. \x1b[34m%s\x1b[0m", idx+1, model.ID)
			if model.InputCost > 0 && model.OutputCost > 0 {
				fmt.Printf(" - $%.3f/$%.3f per M tokens", model.InputCost, model.OutputCost)
			} else if model.Cost > 0 {
				fmt.Printf(" - ~$%.2f/M tokens", model.Cost)
			} else if model.Provider == "Ollama (Local)" {
				fmt.Printf(" - FREE")
			} else {
				fmt.Printf(" - N/A")
			}
			if model.ContextLength > 0 {
				fmt.Printf(" - %dK context", model.ContextLength/1000)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Get user selection
	fmt.Print("\nEnter model number (1-" + strconv.Itoa(len(models)) + ") or 'cancel': ")
	var input string
	fmt.Scanln(&input)

	input = strings.TrimSpace(input)
	if input == "cancel" || input == "" {
		fmt.Println("Model selection cancelled.")
		return nil
	}

	// Parse selection
	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(models) {
		return fmt.Errorf("invalid selection. Please enter a number between 1 and %d", len(models))
	}

	selectedModel := models[selection-1]
	return m.setModel(selectedModel.ID, chatAgent)
}

// setModel sets the specified model for the agent
func (m *ModelsCommand) setModel(modelID string, chatAgent *agent.Agent) error {
	// Validate that the model exists
	models, err := api.GetAvailableModels()
	if err != nil {
		return fmt.Errorf("failed to validate model: %w", err)
	}

	var selectedModel *api.ModelInfo
	for _, model := range models {
		if model.ID == modelID {
			selectedModel = &model
			break
		}
	}

	if selectedModel == nil {
		return fmt.Errorf("model '%s' not found. Use '/models' to see available models", modelID)
	}

	// Update the agent's model
	err = chatAgent.SetModel(modelID)
	if err != nil {
		return fmt.Errorf("failed to set model: %w", err)
	}

	fmt.Printf("✅ Model set to: %s\n", selectedModel.ID)
	if selectedModel.Description != "" {
		fmt.Printf("   %s\n", selectedModel.Description)
	}
	if selectedModel.InputCost > 0 || selectedModel.OutputCost > 0 {
		if selectedModel.InputCost > 0 && selectedModel.OutputCost > 0 {
			fmt.Printf("   Cost: $%.3f/M input, $%.3f/M output tokens\n", selectedModel.InputCost, selectedModel.OutputCost)
		} else if selectedModel.Cost > 0 {
			fmt.Printf("   Cost: ~$%.2f/M tokens\n", selectedModel.Cost)
		}
	} else if selectedModel.Provider == "Ollama (Local)" {
		fmt.Printf("   Cost: FREE (local inference)\n")
	} else {
		fmt.Printf("   Cost: N/A\n")
	}

	return nil
}