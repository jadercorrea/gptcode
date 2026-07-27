package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jadercorrea/gptcode/internal/catalog"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/intelligence"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage and get recommendations for LLM models",
	Long: `Manage LLM models, view catalog, and get intelligent recommendations.

The model system uses machine learning and historical data to suggest the best models
for your specific use case, considering factors like success rate, speed, cost, and performance.`,
}

var modelRecommendCmd = &cobra.Command{
	Use:   "recommend [agent-type]",
	Short: "Get model recommendations (all agents by default)",
	Long: `Get intelligent model recommendations for agents.

The system analyzes:
- Historical performance data
- Model capabilities from catalog
- Cost and speed trade-offs
- Current backend configuration

Examples:
  gptcode model recommend           # All agents (default)
  gptcode model recommend editor    # Specific agent
  gptcode model recommend query`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Se nenhum argumento, mostrar todos (comportamento default)
		if len(args) == 0 {
			return showAllRecommendations()
		}

		agentType := args[0]

		validAgents := map[string]bool{
			"editor":   true,
			"query":    true,
			"research": true,
			"router":   true,
		}

		if !validAgents[agentType] {
			return fmt.Errorf("invalid agent type '%s'. Must be one of: editor, query, research, router", agentType)
		}

		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backend, model, reason, err := intelligence.SelectBestModelForAgent(setup, agentType)
		if err != nil {
			return fmt.Errorf("failed to get recommendation: %w", err)
		}

		fmt.Printf("Recommended model for %s agent:\n", agentType)
		fmt.Printf("  Backend: %s\n", backend)
		fmt.Printf("  Model:   %s\n", model)
		fmt.Printf("Reason: %s\n", reason)

		return nil
	},
}

func showAllRecommendations() error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	agents := []string{"editor", "query", "research", "router"}

	fmt.Println("Recommended Models for All Agents:")

	for _, agentType := range agents {
		backend, model, reason, err := intelligence.SelectBestModelForAgent(setup, agentType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error %s: %v\n", agentType, err)
			continue
		}

		fmt.Printf("  %s:\n", agentType)
		fmt.Printf("    Model:  %s/%s\n", backend, model)
		fmt.Printf("    Reason: %s\n\n", reason)
	}

	return nil
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available models from catalog",
	Long: `List all models in the catalog with their capabilities, cost, and recommended usage.

This shows models from all backends and their metadata including:
- Cost per 1M tokens
- Speed (tokens per second)
- Supported agents
- Backend availability

Use --recommended flag to show only recommended models for current setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		recommendedOnly, _ := cmd.Flags().GetBool("recommended")

		if recommendedOnly {
			return showAllRecommendations()
		}

		backendFilter := ""
		if len(args) > 0 {
			backendFilter = args[0]
		}

		catalogData, err := catalog.Load()
		if err != nil {
			return fmt.Errorf("failed to load catalog: %w\nRun 'gptcode model update --all' to create catalog", err)
		}

		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		var allModels []catalog.ModelOutput
		if backendFilter == "" {
			allModels = append(allModels, catalogData.Groq.Models...)
			allModels = append(allModels, catalogData.OpenRouter.Models...)
			allModels = append(allModels, catalogData.Ollama.Models...)
			allModels = append(allModels, catalogData.OpenAI.Models...)
			allModels = append(allModels, catalogData.DeepSeek.Models...)
		} else {
			backendModels, err := catalog.GetModelsForBackend(backendFilter)
			if err != nil {
				return fmt.Errorf("failed to get models for backend %s: %w", backendFilter, err)
			}
			allModels = backendModels
		}

		fmt.Println("Available Models:")

		for _, model := range allModels {
			fmt.Printf("  %s\n", model.ID)
			fmt.Printf("    Name:      %s\n", model.Name)
			fmt.Printf("    Context:   %d\n", model.ContextWindow)
			fmt.Printf("    Pricing:   $%.4f input / $%.4f output per 1M tokens\n",
				model.PricingPrompt, model.PricingComp)

			if len(model.Tags) > 0 {
				fmt.Printf("    Tags:      %v\n", model.Tags)
			}
			if len(model.RecommendedFor) > 0 {
				fmt.Printf("    Best for:  %v\n", model.RecommendedFor)
			}

			backendName := ""
			slashIdx := strings.Index(model.ID, "/")
			if slashIdx > 0 {
				backendName = model.ID[:slashIdx]
			}
			if backendName == "" {
				for name := range setup.Backend {
					if strings.Contains(strings.ToLower(model.ID), strings.ToLower(name)) {
						backendName = name
						break
					}
				}
			}

			if backendName != "" {
				if _, configured := setup.Backend[backendName]; configured {
					fmt.Printf("    Status:    Backend %s configured\n", backendName)
				} else {
					fmt.Printf("    Status:    Backend %s not configured\n", backendName)
				}
			}

			fmt.Println("")
		}

		return nil
	},
}

var modelInstallCmd = &cobra.Command{
	Use:   "install <model-name>",
	Short: "Install Ollama model if not present",
	Long: `Install an Ollama model locally if it's not already installed.

This checks if the model exists and downloads it if needed.
Only works with Ollama backend.

Examples:
  gptcode model install qwen3-coder
  gptcode model install llama3.3:70b
  gptcode model install deepseek-coder-v2:latest`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		return installOllamaModel(modelName)
	},
}

var modelUpdateCmd = &cobra.Command{
	Use:   "update [model-name]",
	Short: "Update model information from providers",
	Long: `Update model information from provider APIs (OpenRouter, Ollama, etc).

Without arguments: updates only the specified model's information.
With --all flag: updates the entire catalog from all providers.

Examples:
  gptcode model update claude-3.5-sonnet    # Update specific model
  gptcode model update --all                # Update entire catalog`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		updateAll, _ := cmd.Flags().GetBool("all")

		if updateAll {
			return updateCatalogFromAllProviders()
		}

		if len(args) == 0 {
			return fmt.Errorf("specify a model name or use --all to update entire catalog")
		}

		modelName := args[0]
		return updateSingleModel(modelName)
	},
}

var modelSetCmd = &cobra.Command{
	Use:   "set <model-name>",
	Short: "Set default model for your backend",
	Long: `Set the default model for your configured backend.

This updates your ~/.gptcode/config.yaml with the specified model.
The model must be available in the catalog for your backend.

Recommended models by backend:
  groq:       groq/compound         (128k context, tool calling)
  groq:       llama-3.3-70b-versatile (128k context, fast)
  openrouter: openrouter/auto       (auto-routing across models)
  ollama:     qwen2.5-coder:32b     (local, tool calling)
  ollama:     llama3.3:70b          (local, large context)

Examples:
  gptcode model set groq/compound
  gptcode model set llama-3.3-70b-versatile
  gptcode model set openrouter/auto`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		return setDefaultModel(modelName)
	},
}

func setDefaultModel(modelName string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	// Verify model exists in catalog
	catalogData, err := catalog.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Could not load catalog: %v\n", err)
		fmt.Println("Proceeding anyway - model may not be validated")
	} else {
		found := false
		var allModels []catalog.ModelOutput
		allModels = append(allModels, catalogData.Groq.Models...)
		allModels = append(allModels, catalogData.OpenRouter.Models...)
		allModels = append(allModels, catalogData.Ollama.Models...)
		allModels = append(allModels, catalogData.OpenAI.Models...)
		allModels = append(allModels, catalogData.DeepSeek.Models...)

		for _, m := range allModels {
			if m.ID == modelName || m.Name == modelName {
				found = true
				break
			}
		}

		if !found {
			fmt.Fprintf(os.Stderr, "[WARN] Model '%s' not found in catalog\n", modelName)
			fmt.Println("Run 'gptcode model list' to see available models")
			fmt.Println("Proceeding anyway - model may work if backend supports it")
		}
	}

	// Update config
	setup.Defaults.Model = modelName

	// Also update backend-specific default if applicable
	backendName := setup.Defaults.Backend
	if backendConfig, ok := setup.Backend[backendName]; ok {
		backendConfig.DefaultModel = modelName
		setup.Backend[backendName] = backendConfig
	}

	// Save config
	if err := config.SaveSetup(setup); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Default model set to: %s\n", modelName)
	fmt.Printf("   Backend: %s\n", backendName)
	fmt.Println("\nYou can now use 'gptcode do \"task\"' and it will use this model.")

	return nil
}

func updateSingleModel(modelName string) error {
	fmt.Printf("Updating model: %s\n", modelName)

	catalogPath := filepath.Join(os.Getenv("HOME"), ".gptcode", "models_catalog.json")

	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		fmt.Println("No catalog found. Running full update first...")
		return updateCatalogFromAllProviders()
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %w", err)
	}

	var catalogData map[string]interface{}
	if err := json.Unmarshal(data, &catalogData); err != nil {
		return fmt.Errorf("failed to parse catalog: %w", err)
	}

	found := false
	for provider, providerData := range catalogData {
		if providerMap, ok := providerData.(map[string]interface{}); ok {
			if models, ok := providerMap["models"].([]interface{}); ok {
				for _, model := range models {
					if modelMap, ok := model.(map[string]interface{}); ok {
						if id, ok := modelMap["id"].(string); ok && id == modelName {
							found = true
							fmt.Printf("Found in provider: %s\n", provider)
							fmt.Printf("   Name: %v\n", modelMap["name"])
							fmt.Printf("   Context: %v\n", modelMap["context_window"])
							fmt.Printf("   Tags: %v\n", modelMap["tags"])
							break
						}
					}
				}
			}
		}
		if found {
			break
		}
	}

	if !found {
		fmt.Printf("Model '%s' not found in catalog.\n", modelName)
		fmt.Println("Try one of these commands:")
		fmt.Println("  gptcode model list              # See all available models")
		fmt.Println("  gptcode model update --all      # Update full catalog")
		return fmt.Errorf("model not found")
	}

	fmt.Println("Note: This shows cached data. Use 'gptcode model update --all' to refresh.")

	return nil
}

func installOllamaModel(modelName string) error {
	fmt.Printf("Checking Ollama status...\n")

	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		return fmt.Errorf("ollama is not running, start it with: ollama serve")
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama API returned status %d, is ollama running?", resp.StatusCode)
	}

	fmt.Printf("Ollama is running\n")
	fmt.Printf("Installing model: %s\n", modelName)
	fmt.Println("This may take several minutes depending on model size...")

	cmd := exec.Command("ollama", "pull", modelName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install model: %w", err)
	}

	fmt.Printf("\nModel '%s' installed successfully!\n", modelName)
	fmt.Println("You can now use it in your gptcode configuration:")
	fmt.Printf("  gptcode config set defaults.backend ollama\n")
	fmt.Printf("  gptcode config set defaults.model %s\n", modelName)

	return nil
}

func updateCatalogFromAllProviders() error {
	fmt.Println("Fetching models from all providers...")

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	apiKeys := make(map[string]string)
	// Get API keys using GetAPIKey which checks:
	// 1. Environment variables (BACKEND_API_KEY)
	// 2. keys.yaml
	for backendName := range setup.Backend {
		if key := config.GetAPIKey(backendName); key != "" {
			apiKeys[backendName] = key
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	outputPath := filepath.Join(home, ".gptcode", "models_catalog.json")

	fmt.Println("  OpenRouter (public API - no key needed)")
	if _, ok := apiKeys["groq"]; ok {
		fmt.Println("  Groq (using API key + pricing scrape)")
	} else {
		fmt.Println("  Groq (skipped - no API key)")
	}

	if _, ok := apiKeys["openai"]; ok {
		fmt.Println("  OpenAI (using API key)")
	} else {
		fmt.Println("  OpenAI (skipped - no API key)")
	}

	fmt.Println("  Ollama (local + scraping ollama.com)")

	if err := catalog.FetchAndSave(outputPath, apiKeys); err != nil {
		return fmt.Errorf("failed to fetch catalog: %w", err)
	}

	fmt.Printf("\nCatalog updated successfully!\n")
	fmt.Printf("   Saved to: %s\n", outputPath)
	fmt.Println("Use 'gptcode model list' to see all available models.")

	return nil
}

func init() {
	modelListCmd.Flags().Bool("recommended", false, "Show only recommended models for your setup")
	modelUpdateCmd.Flags().Bool("all", false, "Update entire catalog from all providers")

	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelRecommendCmd)
	modelCmd.AddCommand(modelInstallCmd)
	modelCmd.AddCommand(modelUpdateCmd)
	modelCmd.AddCommand(modelSetCmd)
	rootCmd.AddCommand(modelCmd)
}
