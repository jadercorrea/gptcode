package main

import (
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/testrunner"

	"github.com/spf13/cobra"
)

var (
	profileFlag     string
	interactiveFlag bool
	backendFlag     string
	timeoutFlag     int
	notifyFlag      bool
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests",
	Long:  `Run unit or E2E tests for GPTCode`,
}

var testE2ECmd = &cobra.Command{
	Use:   "e2e [category]",
	Short: "Run E2E tests with Ollama models",
	Long: `Run end-to-end tests using configured profiles.

Categories: run, chat, tdd, integration, all (default)

Examples:
  gptcode test e2e                      # Run all tests with default E2E profile
  gptcode test e2e --interactive        # Select profile interactively
  gptcode test e2e --profile local      # Use specific profile
  gptcode test e2e run                  # Run only REPL tests
  gptcode test e2e chat --profile fast  # Chat tests with fast profile
  gptcode test e2e --timeout 300        # Custom timeout per test`,
	RunE: runE2ETests,
}

func init() {
	testE2ECmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Profile to use for tests")
	testE2ECmd.Flags().BoolVarP(&interactiveFlag, "interactive", "i", false, "Select profile interactively")
	testE2ECmd.Flags().StringVarP(&backendFlag, "backend", "b", "", "Override backend (default: from config)")
	testE2ECmd.Flags().IntVarP(&timeoutFlag, "timeout", "t", 180, "Timeout per test in seconds")
	testE2ECmd.Flags().BoolVar(&notifyFlag, "notify", false, "Desktop notification when complete")

	testCmd.AddCommand(testE2ECmd)
	rootCmd.AddCommand(testCmd)
}

func runE2ETests(cmd *cobra.Command, args []string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	backend := setup.Defaults.Backend
	if backendFlag != "" {
		backend = backendFlag
	}

	category := "all"
	if len(args) > 0 {
		category = args[0]
	}

	profile := profileFlag
	if profile == "" {
		profile = setup.E2E.DefaultProfile
	}

	if profile == "" && interactiveFlag {
		selected, err := promptForProfile(backend, setup)
		if err != nil {
			return err
		}
		profile = selected

		if promptYesNo("Set as default E2E profile?") {
			if err := saveDefaultE2EProfile(profile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not save default profile: %v\n", err)
			} else {
				fmt.Printf("[OK] Saved '%s' as default E2E profile\n", profile)
			}
		}
	}

	if profile == "" {
		return fmt.Errorf(`no E2E profile configured

Run with --interactive to select a profile:
  gptcode test e2e --interactive

Or specify profile directly:
  gptcode test e2e --profile local`)
	}

	if !profileExists(backend, profile, setup) {
		return fmt.Errorf("profile '%s' not found for backend '%s'\n\nAvailable profiles: %v",
			profile, backend, listProfileNames(backend, setup))
	}

	timeout := timeoutFlag
	if timeout == 0 && setup.E2E.Timeout > 0 {
		timeout = setup.E2E.Timeout
	}
	if timeout == 0 {
		timeout = 600
	}

	notify := notifyFlag || setup.E2E.Notify

	fmt.Printf(" GPTCode E2E Tests\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Backend:  %s\n", backend)
	fmt.Printf("Profile:  %s\n", profile)
	fmt.Printf("Category: %s\n", category)
	fmt.Printf("Timeout:  %ds per test\n", timeout)
	if notify {
		fmt.Printf("Notify:   enabled\n")
	}
	fmt.Printf("\n")

	describeProfile(backend, profile, setup)

	return testrunner.RunTestsWithProgress(category, backend, profile, timeout, notify)
}

func promptForProfile(backend string, setup *config.Setup) (string, error) {
	profiles := listProfileNames(backend, setup)
	if len(profiles) == 0 {
		return "", fmt.Errorf("no profiles configured for backend '%s'", backend)
	}

	fmt.Printf("\n GPTCode E2E Tests\n\n")
	fmt.Printf("Available profiles for '%s':\n", backend)
	for i, p := range profiles {
		fmt.Printf("  %d. %s\n", i+1, p)
	}

	fmt.Print("\n? Select profile [1]: ")
	var input string
	fmt.Scanln(&input)

	if input == "" {
		input = "1"
	}

	var idx int
	_, err := fmt.Sscanf(input, "%d", &idx)
	if err != nil || idx < 1 || idx > len(profiles) {
		return "", fmt.Errorf("invalid selection")
	}

	return profiles[idx-1], nil
}

func promptYesNo(prompt string) bool {
	fmt.Printf("\n? %s (y/N): ", prompt)
	var input string
	fmt.Scanln(&input)
	return input == "y" || input == "Y"
}

func profileExists(backend, profile string, setup *config.Setup) bool {
	backendCfg, ok := setup.Backend[backend]
	if !ok {
		return false
	}
	_, exists := backendCfg.Profiles[profile]
	return exists
}

func listProfileNames(backend string, setup *config.Setup) []string {
	backendCfg, ok := setup.Backend[backend]
	if !ok {
		return nil
	}

	profiles := []string{}
	for name := range backendCfg.Profiles {
		profiles = append(profiles, name)
	}
	return profiles
}

func describeProfile(backend, profile string, setup *config.Setup) {
	backendCfg := setup.Backend[backend]
	profileCfg := backendCfg.Profiles[profile]

	fmt.Printf("Agent Models:\n")
	if profileCfg.AgentModels.Router != "" {
		fmt.Printf("  Router:   %s\n", profileCfg.AgentModels.Router)
	}
	if profileCfg.AgentModels.Query != "" {
		fmt.Printf("  Query:    %s\n", profileCfg.AgentModels.Query)
	}
	if profileCfg.AgentModels.Editor != "" {
		fmt.Printf("  Editor:   %s\n", profileCfg.AgentModels.Editor)
	}
	if profileCfg.AgentModels.Research != "" {
		fmt.Printf("  Research: %s\n", profileCfg.AgentModels.Research)
	}
	fmt.Printf("\n")
}

func saveDefaultE2EProfile(profile string) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return err
	}

	setup.E2E.DefaultProfile = profile
	return config.SaveSetup(setup)
}
