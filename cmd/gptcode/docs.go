package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/docs"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Manage documentation",
	Long:  `Update and maintain project documentation.`,
}

var docsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update README.md based on recent changes",
	Long: `Analyze recent commits and update README.md automatically.

Examples:
  gptcode docs update           # Analyze and update README
  gptcode docs update --apply   # Apply changes automatically`,
	RunE: runDocsUpdate,
}

var docsAPICmd = &cobra.Command{
	Use:   "api [format]",
	Short: "Generate API documentation from code",
	Long: `Discover API endpoints and generate comprehensive documentation.

Supported formats:
  markdown (default) - Markdown documentation
  openapi           - OpenAPI 3.0 YAML spec
  postman           - Postman Collection JSON

Examples:
  gptcode docs api              # Generate API.md
  gptcode docs api openapi      # Generate api-spec.yaml
  gptcode docs api postman      # Generate api-collection.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDocsAPI,
}

var docsApply bool
var docsModel string

func init() {
	rootCmd.AddCommand(docsCmd)
	docsCmd.AddCommand(docsUpdateCmd)
	docsCmd.AddCommand(docsAPICmd)

	docsUpdateCmd.Flags().BoolVar(&docsApply, "apply", false, "Apply changes automatically")
	docsCmd.PersistentFlags().StringVar(&docsModel, "model", "", "LLM model to use (default: from config)")
}

func runDocsUpdate(cmd *cobra.Command, args []string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getDocsProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	updater := docs.NewReadmeUpdater(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Println("📚 Analyzing recent changes...")

	result, err := updater.UpdateReadme(ctx)
	if err != nil {
		return fmt.Errorf("failed to update README: %w", err)
	}

	if !result.Updated {
		fmt.Println("✅ README is up to date")
		return nil
	}

	fmt.Printf("\n📝 Detected %d change(s):\n", len(result.Changes))
	for _, change := range result.Changes {
		fmt.Printf("  - %s\n", change)
	}

	if docsApply {
		readmePath := filepath.Join(workDir, "README.md")
		if err := updater.ApplyUpdate(readmePath, result.NewText); err != nil {
			return fmt.Errorf("failed to apply update: %w", err)
		}
		fmt.Println("\n✅ README updated successfully")
	} else {
		fmt.Println("\n📋 Preview changes:")
		fmt.Println("Run with --apply to update README.md")

		previewPath := filepath.Join(workDir, "README.new.md")
		if err := os.WriteFile(previewPath, []byte(result.NewText), 0644); err != nil {
			return fmt.Errorf("failed to write preview: %w", err)
		}
		fmt.Printf("\nPreview saved to: %s\n", previewPath)
		fmt.Println("Review with: diff README.md README.new.md")
	}

	return nil
}

func getDocsProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := docsModel
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
		model = backendCfg.GetModelForAgent("query")
		if model == "" {
			model = backendCfg.DefaultModel
		}
	}

	if model == "" {
		return nil, "", fmt.Errorf("no model configured")
	}

	return provider, model, nil
}

func runDocsAPI(cmd *cobra.Command, args []string) error {
	format := "markdown"
	if len(args) > 0 {
		format = args[0]
	}

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getDocsProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator := docs.NewAPIDocGenerator(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Printf("📚 Discovering API endpoints...\n")

	filename, err := generator.Generate(ctx, format)
	if err != nil {
		return fmt.Errorf("failed to generate API docs: %w", err)
	}

	fmt.Printf("✅ Generated: %s\n", filename)
	fmt.Println("\n📚 Documentation includes:")
	fmt.Println("  - Endpoint descriptions")
	fmt.Println("  - Request/response examples")
	fmt.Println("  - Authentication info")
	fmt.Println("  - Error responses")

	return nil
}
