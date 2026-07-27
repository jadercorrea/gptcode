package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/migration"
	"github.com/spf13/cobra"
)

var evolveCmd = &cobra.Command{
	Use:   "evolve",
	Short: "Zero-downtime database schema evolution",
	Long:  `Generate safe, phased database migrations for production deployments.`,
}

var evolveGenerateCmd = &cobra.Command{
	Use:   "generate <description>",
	Short: "Generate zero-downtime migration strategy",
	Long: `Generate a multi-phase migration strategy that can be safely deployed to production.

Examples:
  gptcode evolve generate "add email column to users table"
  gptcode evolve generate "rename status to state in orders"`,
	Args: cobra.ExactArgs(1),
	RunE: runEvolveGenerate,
}

var evolveModel string
var evolveDir string

func init() {
	rootCmd.AddCommand(evolveCmd)
	evolveCmd.AddCommand(evolveGenerateCmd)

	evolveCmd.PersistentFlags().StringVar(&evolveModel, "model", "", "LLM model to use")
	evolveCmd.PersistentFlags().StringVar(&evolveDir, "dir", "migrations", "Migration directory")
}

func runEvolveGenerate(cmd *cobra.Command, args []string) error {
	description := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getEvolveProvider(setup)
	if err != nil {
		return err
	}

	evolver := migration.NewSchemaEvolution(provider, model, evolveDir)

	fmt.Printf("🔄 Generating zero-downtime migration strategy...\n")
	fmt.Printf("📝 Description: %s\n\n", description)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	evolution, err := evolver.GenerateEvolution(ctx, description)
	if err != nil {
		return fmt.Errorf("failed to generate evolution: %w", err)
	}

	fmt.Printf("✅ Generated %d-phase migration: %s\n\n", len(evolution.Steps), evolution.Name)

	for _, step := range evolution.Steps {
		fmt.Printf("Phase %d: %s\n", step.Phase, step.Description)
	}

	fmt.Println("\n💾 Saving migration files...")
	if err := evolver.SaveEvolution(evolution); err != nil {
		return fmt.Errorf("failed to save evolution: %w", err)
	}

	fmt.Printf("\n✅ Migration saved to %s/\n", evolveDir)
	fmt.Println("📖 Review the README.md for deployment instructions")

	return nil
}

func getEvolveProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := evolveModel
	backendName := setup.Defaults.Backend
	if backendName == "" {
		backendName = "anthropic"
	}

	backendCfg, ok := setup.Backend[backendName]
	if !ok {
		return nil, "", fmt.Errorf("backend %s not configured", backendName)
	}

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	if model == "" {
		model = backendCfg.GetModelForAgent("editor")
		if model == "" {
			model = backendCfg.DefaultModel
		}
	}

	if model == "" {
		return nil, "", fmt.Errorf("no model configured")
	}

	return provider, model, nil
}
