package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/configmgmt"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/spf13/cobra"
)

var configMgmtCmd = &cobra.Command{
	Use:   "cfg",
	Short: "Configuration file management",
	Long:  `Detect and update configuration files across environments.`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected configuration files",
	Long:  `Scan and list all configuration files in the project.`,
	RunE:  runConfigList,
}

var configUpdateCmd = &cobra.Command{
	Use:   "update <key> <value>",
	Short: "Update configuration value",
	Long: `Update a configuration key across environments.

Examples:
  gptcode cfg update DATABASE_URL "postgres://..."        # Update in all configs
  gptcode cfg update --env=production PORT 8080           # Production only
  gptcode cfg update --apply API_KEY "secret"             # Apply immediately`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigUpdate,
}

var configEnv string
var configApply bool
var configModel string

func init() {
	rootCmd.AddCommand(configMgmtCmd)
	configMgmtCmd.AddCommand(configListCmd)
	configMgmtCmd.AddCommand(configUpdateCmd)

	configUpdateCmd.Flags().StringVar(&configEnv, "env", "", "Target environment (production, development, test)")
	configUpdateCmd.Flags().BoolVar(&configApply, "apply", false, "Apply changes immediately")
	configMgmtCmd.PersistentFlags().StringVar(&configModel, "model", "", "LLM model to use")
}

func runConfigList(cmd *cobra.Command, args []string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getConfigProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	mgr := configmgmt.NewManager(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	report, err := mgr.DetectAndUpdate(ctx, "", "", "", false)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	if len(report.Detected) == 0 {
		fmt.Println("No configuration files detected")
		return nil
	}

	fmt.Printf("📋 Found %d configuration file(s):\n\n", len(report.Detected))

	envGroups := make(map[string][]configmgmt.ConfigFile)
	for _, cfg := range report.Detected {
		envGroups[cfg.Environment] = append(envGroups[cfg.Environment], cfg)
	}

	for env, files := range envGroups {
		fmt.Printf("Environment: %s\n", env)
		for _, cfg := range files {
			fmt.Printf("  - %s [%s]\n", cfg.Path, cfg.Format)
		}
		fmt.Println()
	}

	return nil
}

func runConfigUpdate(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getConfigProvider(setup)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	mgr := configmgmt.NewManager(provider, model, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Printf("🔧 Updating %s...\n", key)

	report, err := mgr.DetectAndUpdate(ctx, configEnv, key, value, configApply)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if len(report.Changes) == 0 {
		fmt.Println("⚠️  No matching configuration files found")
		return nil
	}

	fmt.Printf("\n📝 Changes:\n")
	for i, change := range report.Changes {
		fmt.Printf("\n%d. %s [%s]\n", i+1, change.File, change.Environment)
		fmt.Printf("   Key: %s\n", change.Key)
		if change.OldValue != "" {
			fmt.Printf("   Old: %s\n", change.OldValue)
		}
		fmt.Printf("   New: %s\n", change.NewValue)
	}

	if configApply {
		fmt.Printf("\n✅ Applied %d change(s)\n", len(report.Changes))
		if len(report.UpdatedFiles) > 0 {
			fmt.Println("\nUpdated files:")
			for _, file := range report.UpdatedFiles {
				fmt.Printf("  - %s\n", file)
			}
		}
	} else {
		fmt.Println("\n💡 Run with --apply to apply changes")
	}

	if len(report.Errors) > 0 {
		fmt.Printf("\n⚠️  %d error(s) occurred:\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

func getConfigProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := configModel
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
