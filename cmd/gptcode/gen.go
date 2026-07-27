package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jadercorrea/gptcode/internal/changelog"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/migration"
	"github.com/jadercorrea/gptcode/internal/mockgen"
	"github.com/jadercorrea/gptcode/internal/testgen"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate code, tests, and more",
	Long:  `Generate code artifacts using LLM.`,
}

var genTestCmd = &cobra.Command{
	Use:   "test <file>",
	Short: "Generate unit tests for a source file",
	Args:  cobra.ExactArgs(1),
	RunE:  runGenTest,
}

var genChangelogCmd = &cobra.Command{
	Use:   "changelog [from-tag] [to-tag]",
	Short: "Generate CHANGELOG entry from git commits",
	Long: `Generate a CHANGELOG entry using conventional commits.

Examples:
  gptcode gen changelog           # All commits since last tag
  gptcode gen changelog v1.0.0    # From v1.0.0 to HEAD
  gptcode gen changelog v1.0.0 v1.1.0  # Between two tags`,
	Args: cobra.MaximumNArgs(2),
	RunE: runGenChangelog,
}

var genMockCmd = &cobra.Command{
	Use:   "mock <file>",
	Short: "Generate mock implementation for interfaces in a file",
	Long: `Generate mock objects for Go interfaces.

Examples:
  gptcode gen mock internal/storage/storage.go
  gptcode gen mock pkg/service/service.go`,
	Args: cobra.ExactArgs(1),
	RunE: runGenMock,
}

var genIntegrationCmd = &cobra.Command{
	Use:   "integration <package>",
	Short: "Generate integration tests for a package",
	Long: `Generate integration tests that test component interactions.

Examples:
  gptcode gen integration ./internal/api
  gptcode gen integration ./pkg/service`,
	Args: cobra.ExactArgs(1),
	RunE: runGenIntegration,
}

var genMigrationCmd = &cobra.Command{
	Use:   "migration <name>",
	Short: "Generate database migration from model changes",
	Long: `Detect changes in model structs and generate SQL migration.

Examples:
  gptcode gen migration "add user email"
  gptcode gen migration "update product schema"`,
	Args: cobra.ExactArgs(1),
	RunE: runGenMigration,
}

var genSnapshotCmd = &cobra.Command{
	Use:   "snapshot <file>",
	Short: "Generate snapshot tests for a source file",
	Long: `Generate snapshot tests that capture output for regression testing.

Supports: Go, TypeScript/React, Python, Ruby

Examples:
  gptcode gen snapshot components/Button.tsx
  gptcode gen snapshot pkg/formatter/formatter.go`,
	Args: cobra.ExactArgs(1),
	RunE: runGenSnapshot,
}

var genModel string

func init() {
	rootCmd.AddCommand(genCmd)
	genCmd.AddCommand(genTestCmd)
	genCmd.AddCommand(genChangelogCmd)
	genCmd.AddCommand(genMockCmd)
	genCmd.AddCommand(genIntegrationCmd)
	genCmd.AddCommand(genMigrationCmd)
	genCmd.AddCommand(genSnapshotCmd)

	genCmd.PersistentFlags().StringVar(&genModel, "model", "", "LLM model to use (default: from config)")
}

func runGenTest(cmd *cobra.Command, args []string) error {
	sourceFile := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator, err := testgen.NewTestGenerator(provider, model, workDir)
	if err != nil {
		return fmt.Errorf("failed to create test generator: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Printf("🧪 Generating unit tests for: %s\n", sourceFile)

	result, err := generator.GenerateUnitTests(ctx, sourceFile)
	if err != nil && result == nil {
		return fmt.Errorf("failed to generate tests: %w", err)
	}

	if result.Valid {
		fmt.Printf("✅ Generated %s (valid)\n", result.TestFile)
	} else {
		fmt.Printf("⚠️  Generated %s (may have compilation issues)\n", result.TestFile)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}

	return nil
}

func runGenIntegration(cmd *cobra.Command, args []string) error {
	packagePath := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator := testgen.NewIntegrationTestGenerator(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Printf("🔗 Generating integration tests for: %s\n", packagePath)

	result, err := generator.GenerateIntegrationTests(ctx, packagePath)
	if err != nil && result == nil {
		return fmt.Errorf("failed to generate integration tests: %w", err)
	}

	if result.Valid {
		fmt.Printf("✅ Generated %s (valid)\n", result.TestFile)
	} else {
		fmt.Printf("⚠️  Generated %s (may have issues)\n", result.TestFile)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}

	fmt.Println("\nRun with: go test -tags=integration")
	return nil
}

func runGenMigration(cmd *cobra.Command, args []string) error {
	migrationName := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator := migration.NewMigrationGenerator(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Printf("💾 Analyzing model changes for: %s\n", migrationName)

	result, err := generator.GenerateMigration(ctx, migrationName)
	if err != nil && result == nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	if len(result.Changes) == 0 {
		fmt.Println("ℹ️  No model changes detected")
		return nil
	}

	fmt.Printf("\n🔍 Detected %d change(s):\n", len(result.Changes))
	for _, change := range result.Changes {
		switch change.Type {
		case "added":
			if change.Field == "" {
				fmt.Printf("  + Model: %s\n", change.ModelName)
			} else {
				fmt.Printf("  + %s.%s (%s)\n", change.ModelName, change.Field, change.NewType)
			}
		case "modified":
			fmt.Printf("  ~ %s.%s: %s -> %s\n", change.ModelName, change.Field, change.OldType, change.NewType)
		case "removed":
			if change.Field == "" {
				fmt.Printf("  - Model: %s\n", change.ModelName)
			} else {
				fmt.Printf("  - %s.%s\n", change.ModelName, change.Field)
			}
		}
	}

	if result.Valid {
		fmt.Printf("\n✅ Generated migration: %s\n", result.MigrationFile)
	} else {
		fmt.Printf("\n⚠️  Generated migration with issues: %s\n", result.MigrationFile)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}

	return nil
}

func getGenProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := genModel
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

func runGenChangelog(cmd *cobra.Command, args []string) error {
	var fromTag, toTag string

	if len(args) == 0 {
		fromTag = ""
		toTag = "HEAD"
	} else if len(args) == 1 {
		fromTag = args[0]
		toTag = "HEAD"
	} else {
		fromTag = args[0]
		toTag = args[1]
	}

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator := changelog.NewChangelogGenerator(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("📝 Generating CHANGELOG from %s to %s...\n", fromTag, toTag)

	entry, err := generator.Generate(ctx, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("failed to generate changelog: %w", err)
	}

	fmt.Println("\n" + entry)
	return nil
}

func runGenMock(cmd *cobra.Command, args []string) error {
	sourceFile := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	generator := mockgen.NewMockGenerator(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Printf("🎭 Generating mocks for: %s\n", sourceFile)

	result, err := generator.GenerateMock(ctx, sourceFile)
	if err != nil && result == nil {
		return fmt.Errorf("failed to generate mock: %w", err)
	}

	if result.Valid {
		fmt.Printf("✅ Generated %s (valid)\n", result.MockFile)
	} else {
		fmt.Printf("⚠️  Generated %s (may have issues)\n", result.MockFile)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}

	return nil
}

func runGenSnapshot(cmd *cobra.Command, args []string) error {
	sourceFile := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGenProvider(setup)
	if err != nil {
		return err
	}

	generator := testgen.NewSnapshotGenerator(provider, model)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Printf("📸 Generating snapshot tests for: %s\n", sourceFile)

	testFile, err := generator.Generate(ctx, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to generate snapshot tests: %w", err)
	}

	fmt.Printf("✅ Generated: %s\n", testFile)
	fmt.Println("\n💡 Next steps:")
	fmt.Println("  1. Run the tests to generate initial snapshots")
	fmt.Println("  2. Review the snapshots in __snapshots__/")
	fmt.Println("  3. Commit both test files and snapshots")

	return nil
}
