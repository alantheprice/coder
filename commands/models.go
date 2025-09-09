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

	for i, model := range models {
		fmt.Printf("%d. %s\n", i+1, model.ID)
		if model.Description != "" {
			fmt.Printf("   Description: %s\n", model.Description)
		}
		if model.Size != "" {
			fmt.Printf("   Size: %s\n", model.Size)
		}
		if model.Cost > 0 {
			if strings.Contains(model.Description, "$") {
				// Description already contains detailed pricing info
				fmt.Printf("   Cost: See description for rates\n")
			} else {
				// Legacy format fallback
				fmt.Printf("   Cost: ~$%.2f/M tokens\n", model.Cost)
			}
		} else {
			fmt.Printf("   Cost: FREE (local)\n")
		}
		fmt.Println()
	}

	fmt.Println("Usage:")
	fmt.Println("  /models select          - Interactive model selection")
	fmt.Println("  /models <model_id>      - Set model directly")
	fmt.Println("  /models                 - Show this list")

	return nil
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

	fmt.Println("\n🎯 Select a Model:")
	fmt.Println("==================")

	// Display models with numbers
	for i, model := range models {
		fmt.Printf("%d. %s", i+1, model.ID)
		if model.Description != "" {
			fmt.Printf(" - %s", model.Description)
		}
		if model.Cost > 0 {
			if strings.Contains(model.Description, "$") {
				fmt.Printf(" (pay per use)")
			} else {
				fmt.Printf(" (~$%.2f/M)", model.Cost)
			}
		} else {
			fmt.Printf(" (FREE)")
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
	if selectedModel.Cost > 0 {
		if strings.Contains(selectedModel.Description, "$") {
			fmt.Printf("   Cost: See description above for detailed rates\n")
		} else {
			fmt.Printf("   Cost: ~$%.2f/M tokens\n", selectedModel.Cost)
		}
	} else {
		fmt.Printf("   Cost: FREE (local inference)\n")
	}

	return nil
}