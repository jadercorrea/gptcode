package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/merge"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Resolve merge conflicts with AI assistance",
	Long:  `Automatically resolve Git merge conflicts using LLM-powered analysis.`,
}

var mergeResolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve all merge conflicts",
	Long: `Detect and resolve all merge conflicts in the working directory.

Examples:
  gptcode merge resolve
  gptcode merge resolve --model claude-3-5-sonnet-20241022`,
	RunE: runMergeResolve,
}

var mergeModel string

func init() {
	rootCmd.AddCommand(mergeCmd)
	mergeCmd.AddCommand(mergeResolveCmd)

	mergeCmd.PersistentFlags().StringVar(&mergeModel, "model", "", "LLM model to use")
}

func runMergeResolve(cmd *cobra.Command, args []string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getMergeProvider(setup)
	if err != nil {
		return err
	}

	resolver := merge.NewResolver(provider, model)

	conflicts, err := resolver.DetectConflicts()
	if err != nil {
		return fmt.Errorf("failed to detect conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		fmt.Println("✅ No merge conflicts detected")
		return nil
	}

	fmt.Printf("🔍 Found %d file(s) with conflicts\n\n", len(conflicts))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resolved, err := resolver.ResolveAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve conflicts: %w", err)
	}

	for _, cf := range resolved {
		fmt.Printf("📝 Resolving %s...\n", cf.Path)

		if err := resolver.ValidateResolution(cf); err != nil {
			fmt.Printf("   ⚠️  Warning: %v\n", err)
			continue
		}

		if err := resolver.ApplyResolution(cf); err != nil {
			fmt.Printf("   ❌ Failed: %v\n", err)
			continue
		}

		fmt.Printf("   ✅ Resolved and staged\n")
	}

	fmt.Println("\n✅ All conflicts resolved")
	fmt.Println("💡 Review changes with: git diff --cached")
	fmt.Println("💡 Commit with: git commit")

	return nil
}

func getMergeProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := mergeModel
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
