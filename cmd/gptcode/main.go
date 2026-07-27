package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jadercorrea/gptcode/internal/repl"

	"github.com/jadercorrea/gptcode/internal/catalog"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/elixir"
	"github.com/jadercorrea/gptcode/internal/feedback"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/live"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/memory"
	"github.com/jadercorrea/gptcode/internal/ml"
	"github.com/jadercorrea/gptcode/internal/modes"
	"github.com/jadercorrea/gptcode/internal/ollama"
	"github.com/jadercorrea/gptcode/internal/prompt"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GPTCode version %s\n", version)
	},
}

func init() {
	version = resolveVersion(version, moduleVersion())
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("GPTCode version {{.Version}}\n")
}

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

func resolveVersion(injected, module string) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if module != "" && module != "(devel)" {
		return module
	}
	return "dev"
}

var rootCmd = &cobra.Command{
	Use:   "gptcode",
	Short: "Verifiable AI coding workflows from the terminal",
	Long: `GPTCode – Verifiable AI coding workflows

AI coding agents should produce evidence, not just answers.
Research → Plan → Implement → Review → Verify.

## AGENTIC EXECUTION
  gptcode do "task" [--supervised] [--interactive]  - Autonomous execution with agent orchestration

## INTERACTIVE (Conversational)
  gptcode chat                - Code-focused conversation (CLI or Neovim)
  gptcode run "task"          - Execute tasks with follow-up

## WORKFLOW (Manual Control)
  gptcode research "question" - Document codebase and architecture
  gptcode plan "task"         - Create implementation plan
  gptcode implement plan.md   - Execute plan step-by-step

## SPECIALIZED TOOLS
  gptcode gen test <file>        - Generate unit tests for a file
  gptcode gen integration <pkg>  - Generate integration tests for a package
  gptcode gen snapshot <file>    - Generate snapshot tests for regression
  gptcode gen mock <file>        - Generate mocks for interfaces
  gptcode gen migration <name>   - Generate DB migration from model changes
  gptcode gen changelog          - Generate CHANGELOG from git commits
  gptcode docs update            - Update README based on changes
  gptcode docs api               - Generate API docs (Markdown/OpenAPI/Postman)
  gptcode coverage [pkg]         - Analyze test coverage gaps
  gptcode tdd                    - Test-driven development mode
  gptcode feature "desc"      - Generate tests + implementation
  gptcode review [target]     - Code review for bugs, security, improvements

## MODEL MANAGEMENT
  gptcode model list [--recommended]   - List models from catalog
  gptcode model recommend [agent]      - Get model recommendations
  gptcode model install <model>        - Install Ollama model
  gptcode model update [--all]         - Update catalog from providers

## CONFIGURATION
  gptcode setup                - Initialize ~/.gptcode configuration
  gptcode key [backend]        - Add/update API key
  gptcode backend              - Show current backend
  gptcode backend list         - List all backends
  gptcode backend use <name>   - Switch backend
  gptcode profile              - Show current profile
  gptcode profile list         - List all profiles
  gptcode profile use <backend>.<profile> - Switch profile

## REFACTORING
  gptcode refactor api              - Coordinate API changes (routes, handlers, tests)
  gptcode refactor signature <func> - Change function signature and update all call sites
  gptcode refactor breaking         - Detect breaking changes and update all consumers
  gptcode refactor type <name>      - Refactor type definition and propagate changes
  gptcode refactor compat <old> <new> - Add backward compatibility wrapper

## SECURITY
  gptcode security scan             - Scan for vulnerabilities
  gptcode security scan --fix       - Scan and auto-fix vulnerabilities

## CONFIGURATION
  gptcode cfg list                  - List configuration files
  gptcode cfg update KEY VALUE      - Update config value across environments

## PERFORMANCE
  gptcode perf profile [target]     - Profile CPU/memory performance
  gptcode perf bench [pattern]      - Run benchmarks with optimization tips

## ADVANCED
  gptcode config get/set       - Direct config manipulation (advanced)
  gptcode ml list|train|test|eval|predict - Machine learning features
  gptcode graph build|query    - Dependency graph analysis
  gptcode feedback good|bad    - User feedback tracking
  gptcode detect-language      - Detect project language`,
}

func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Skip for setup and key commands
		if cmd.Name() == "setup" || cmd.Name() == "key" || cmd.Name() == "completion" {
			return nil
		}

		// Check if setup exists and show welcome message if needed
		setup, err := config.LoadSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, `
╔════════════════════════════════════════════════════════════╗
║                    Welcome to GPTCode!                     ║
╠════════════════════════════════════════════════════════════╣
║                                                            ║
║  To get started, run:                                     ║
║                                                            ║
║    gt setup                                               ║
║                                                            ║
║  This will guide you through the initial setup.            ║
║                                                            ║
║  Quick start (free, no API key needed):                    ║
║    1. Get key: https://openrouter.ai/keys                ║
║    2. Run: gt key openrouter                              ║
║    3. Run: gt run "your task"                             ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝
`)
			return nil
		}

		// Check if backend is configured
		if setup.Defaults.Backend == "" {
			fmt.Fprintf(os.Stderr, `
⚠️  No backend configured. Run: gptcode setup
`)
		}

		// Initialize Live dashboard connection (with short timeout)
		// Must be synchronous so GetClient() works during command execution
		done := make(chan struct{})
		go func() {
			defer close(done)
			agentID := live.GetAgentID()
			client := live.NewClient(live.GetDashboardURL(), agentID)
			if err := client.Connect(); err != nil {
				return
			}
			live.SetGlobalClient(client)

			// Register global control manager for pause/resume/kill from dashboard
			cm := live.GetControlManager()
			cm.RegisterWithClient(client)
		}()
		select {
		case <-done:
			// Connected (or failed silently)
		case <-time.After(2 * time.Second):
			// Timeout — proceed without live dashboard
		}

		return nil
	}

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(versionCmd)
	setupCmd.Flags().BoolP("yes", "y", false, "Use Quick Start (non-interactive)")
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(releaseCmd)
	rootCmd.AddCommand(backendCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(detectLanguageCmd)
	rootCmd.AddCommand(mlCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(goCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(tddCmd)
	rootCmd.AddCommand(researchCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(implementCmd)
	rootCmd.AddCommand(featureCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(prCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(trainingCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(gainCmd)
	rootCmd.AddCommand(cavemanCmd)
}

// monitorCmd scans local AI agent logs and reports real API usage to the Live Dashboard
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Scan local AI agents and report real API usage to Live Dashboard",
	Long: `Discovers installed AI coding agents (Antigravity, Cursor, Windsurf, etc.),
reads their log files, counts today's API calls per model, and reports real
usage data to the gptcode Live Dashboard.

This is the 'fuel gauge' — shows how much of your daily model quota is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔍 Scanning local AI agents...")
		fmt.Println()

		result, err := live.Monitor()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if len(result.Agents) == 0 {
			fmt.Println("No AI agent activity found today.")
			return nil
		}

		// Display results
		fmt.Printf("📊 Found %d API calls across %d agent(s) today:\n\n", result.TotalAPICalls, len(result.Agents))
		for _, u := range result.Agents {
			pct := int(u.QuotaUsed * 100)
			bar := renderBar(u.QuotaUsed)
			exhaustedTag := ""
			if u.Exhausted {
				exhaustedTag = " ⚠️  HIT RATE LIMIT"
			}
			fmt.Printf("  %s (%s)\n", u.DisplayName(), u.Agent)
			fmt.Printf("  %s %d%% · %d calls · %s → %s%s\n\n", bar, pct, u.APICalls, u.FirstCall, u.LastCall, exhaustedTag)
		}

		// Report to Live Dashboard
		liveURL := os.Getenv("GPTCODE_LIVE_URL")
		if liveURL == "" {
			liveURL = "https://gptcode.live"
		}

		reportConfig := live.DefaultReportConfig()
		reportConfig.SetBaseURL(liveURL)

		fmt.Println("📡 Reporting to Live Dashboard...")
		if err := result.ReportToLive(reportConfig); err != nil {
			fmt.Printf("⚠️  Report error: %v\n", err)
		} else {
			fmt.Printf("✅ Reported to %s\n", liveURL)
		}

		return nil
	},
}

func renderBar(quota float64) string {
	filled := int(quota * 10)
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "●"
	}
	for i := 0; i < empty; i++ {
		bar += "○"
	}
	return bar
}

func newBuilderAndLLM(lang, mode, hint string) (*prompt.Builder, llm.Provider, string, error) {
	setup, err := config.LoadSetup()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load setup: %w", err)
	}

	store, _ := memory.LoadStore()
	builder := prompt.NewDefaultBuilder(store)

	backendName := setup.Defaults.Backend
	modelAlias := setup.Defaults.Model

	backendCfg := setup.Backend[backendName]
	model := backendCfg.DefaultModel
	if alias, ok := backendCfg.Models[modelAlias]; ok {
		model = alias
	} else if modelAlias != "" && modelAlias != backendCfg.DefaultModel {
		model = modelAlias
	}

	var provider llm.Provider
	if strings.Contains(model, "compound") {
		var customExec llm.Provider
		customModel := backendCfg.DefaultModel

		if backendCfg.Type == "ollama" {
			customExec = llm.NewOllama(backendCfg.BaseURL)
		} else {
			customExec = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}

		provider = llm.NewOrchestrator(backendCfg.BaseURL, backendName, customExec, customModel)
	} else {
		if backendCfg.Type == "ollama" {
			provider = llm.NewOllama(backendCfg.BaseURL)
		} else {
			provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}
	}

	return builder, provider, model, nil
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize ~/.gptcode with default profile and system prompt",
	Run: func(cmd *cobra.Command, args []string) {
		quickStart, _ := cmd.Flags().GetBool("yes")
		if quickStart {
			config.RunSetupQuickStart()
		} else {
			config.RunSetup()
		}
	},
}

var keyCmd = &cobra.Command{
	Use:   "key [backend]",
	Short: "Add or update API key for a backend (e.g., gptcode key openrouter)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backendName := args[0]
		return config.UpdateAPIKey(backendName)
	},
}

var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Show and manage backends",
	RunE: func(cmd *cobra.Command, args []string) error {
		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backendName := setup.Defaults.Backend
		backendCfg, ok := setup.Backend[backendName]
		if !ok {
			return fmt.Errorf("backend %s not found", backendName)
		}

		fmt.Printf("Current: %s\n", backendName)
		fmt.Printf("  Type: %s\n", backendCfg.Type)
		fmt.Printf("  URL: %s\n", backendCfg.BaseURL)
		if backendCfg.DefaultModel != "" {
			fmt.Printf("  Default model: %s\n", backendCfg.DefaultModel)
		}
		return nil
	},
}

var backendListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured backends",
	RunE: func(cmd *cobra.Command, args []string) error {
		backends, err := config.ListBackends()
		if err != nil {
			return fmt.Errorf("failed to list backends: %w", err)
		}

		setup, _ := config.LoadSetup()
		defaultBackend := setup.Defaults.Backend

		for _, name := range backends {
			if name == defaultBackend {
				fmt.Printf("%s (default)\n", name)
			} else {
				fmt.Println(name)
			}
		}
		return nil
	},
}

var backendCreateCmd = &cobra.Command{
	Use:   "create <name> <type> <base-url>",
	Short: "Create new backend",
	Long: `Create a new backend configuration.

Type must be: openai, ollama

Examples:
  gptcode backend create mygroq openai https://api.groq.com/openai/v1
  gptcode backend create local ollama http://localhost:11434`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		backendType := args[1]
		baseURL := args[2]

		if backendType != "openai" && backendType != "ollama" {
			return fmt.Errorf("type must be 'openai' or 'ollama'")
		}

		if err := config.CreateBackend(name, backendType, baseURL); err != nil {
			return err
		}

		fmt.Printf("[OK] Created backend: %s\n", name)
		fmt.Println("\nNext steps:")
		if backendType == "openai" {
			fmt.Printf("  gptcode key %s                    # Set API key\n", name)
		}
		fmt.Printf("  gptcode config set backend.%s.default_model <model>\n", name)
		fmt.Printf("  gptcode backend use %s            # Switch to this backend\n", name)
		return nil
	},
}

var backendDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete backend",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := config.DeleteBackend(name); err != nil {
			return err
		}

		fmt.Printf("[OK] Deleted backend: %s\n", name)
		return nil
	},
}

var backendShowCmd = &cobra.Command{
	Use:   "show [backend]",
	Short: "Show backend configuration (current if not specified)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backendName := setup.Defaults.Backend
		if len(args) > 0 {
			backendName = args[0]
		}

		backendCfg, ok := setup.Backend[backendName]
		if !ok {
			return fmt.Errorf("backend %s not found", backendName)
		}

		fmt.Printf("%s\n", backendName)
		fmt.Printf("  Type: %s\n", backendCfg.Type)
		fmt.Printf("  URL: %s\n", backendCfg.BaseURL)
		if backendCfg.DefaultModel != "" {
			fmt.Printf("  Default model: %s\n", backendCfg.DefaultModel)
		}
		if len(backendCfg.Models) > 0 {
			fmt.Println("  Models:")
			for alias, model := range backendCfg.Models {
				fmt.Printf("    %s: %s\n", alias, model)
			}
		}
		return nil
	},
}

var backendUseCmd = &cobra.Command{
	Use:   "use <backend>",
	Short: "Switch to a backend",
	Long: `Switch to a specific backend.

Examples:
  gptcode backend use groq
  gptcode backend use openrouter
  gptcode backend use ollama`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backendName := args[0]

		if err := config.SetConfig("defaults.backend", backendName); err != nil {
			return fmt.Errorf("failed to set backend: %w", err)
		}

		fmt.Printf("[OK] Switched to %s\n", backendName)
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get configuration value",
	Long: `Get a configuration value from ~/.gptcode/setup.yaml

Examples:
  gptcode config get defaults.backend
  gptcode config get defaults.profile
  gptcode config get backend.groq.default_model`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value, err := config.GetConfig(key)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set configuration value",
	Long: `Set a configuration value in ~/.gptcode/setup.yaml

Examples:
  gptcode config set defaults.backend groq
  gptcode config set defaults.profile speed
  gptcode config set backend.groq.default_model llama-3.3-70b-versatile`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		if err := config.SetConfig(key, value); err != nil {
			return err
		}
		fmt.Printf("[OK] Set %s = %s\n", key, value)
		return nil
	},
}

var detectLanguageCmd = &cobra.Command{
	Use:     "detect-language [path]",
	Aliases: []string{"detect"},
	Short:   "Detect project language distribution using GitHub Linguist",
	Long: `Analyze the project and show language breakdown.

Uses go-enry (GitHub Linguist port) for accurate multi-language detection.
Automatically excludes vendored code, generated files, and documentation.

Examples:
  gptcode detect language
  gptcode detect language /path/to/project
  gptcode detect language .`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		detector := langdetect.NewDetector(path)
		breakdown, err := detector.Detect()
		if err != nil {
			return fmt.Errorf("failed to detect languages: %w", err)
		}

		fmt.Print(langdetect.FormatBreakdown(breakdown))
		return nil
	},
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage model catalog",
}

var modelsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update model catalog from multiple sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Fetching models from available sources...")

		apiKeys := map[string]string{
			"groq":      os.Getenv("GROQ_API_KEY"),
			"openai":    os.Getenv("OPENAI_API_KEY"),
			"anthropic": os.Getenv("ANTHROPIC_API_KEY"),
			"cohere":    os.Getenv("COHERE_API_KEY"),
		}

		catalogPath := catalog.GetCatalogPath()
		if err := catalog.FetchAndSave(catalogPath, apiKeys); err != nil {
			return fmt.Errorf("failed to update catalog: %w", err)
		}
		fmt.Printf("[OK] Model catalog updated: %s\n", catalogPath)
		return nil
	},
}

var modelsSearchCmd = &cobra.Command{
	Use:   "search [term1] [term2] ...",
	Short: "Search models in catalog with filtering and sorting",
	Long: `Search models from a backend with multi-term filtering.
Models are automatically sorted by price (lowest first), then by context window (largest first).

Single term: filters across all backends
  gptcode models search gemini

Multiple terms: ANDs all terms (must match all)
  gptcode models search groq llama     # groq models with "llama" in name
  gptcode models search free coding     # free models tagged as coding

Flags override positional backend:
  gptcode models search gemini --backend openrouter`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backendFlag, _ := cmd.Flags().GetString("backend")
		agentFlag, _ := cmd.Flags().GetString("agent")

		var queryTerms []string
		if len(args) > 0 {
			queryTerms = args
		}

		models, err := catalog.SearchModelsMulti(backendFlag, queryTerms, agentFlag)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		type ModelJSON struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Tags          []string `json:"tags"`
			Recommended   bool     `json:"recommended"`
			ContextWindow int      `json:"context_window"`
			PricePrompt   float64  `json:"pricing_prompt_per_m_tokens"`
			PriceComp     float64  `json:"pricing_completion_per_m_tokens"`
			Installed     bool     `json:"installed"`
			FeedbackScore float64  `json:"feedback_score"`
		}

		result := make([]ModelJSON, 0, len(models))
		for _, m := range models {
			recommended := false
			if agentFlag != "" {
				for _, rec := range m.RecommendedFor {
					if rec == agentFlag {
						recommended = true
						break
					}
				}
			}

			result = append(result, ModelJSON{
				ID:            m.ID,
				Name:          m.Name,
				Tags:          m.Tags,
				Recommended:   recommended,
				ContextWindow: m.ContextWindow,
				PricePrompt:   m.PricingPrompt,
				PriceComp:     m.PricingComp,
				Installed:     m.Installed,
				FeedbackScore: m.FeedbackScore,
			})
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}

		return nil
	},
}

var modelsInstallCmd = &cobra.Command{
	Use:   "install <model>",
	Short: "Install Ollama model if not present",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]

		installed, err := ollama.IsInstalled(modelName)
		if err != nil {
			return fmt.Errorf("failed to check model status: %w", err)
		}

		if installed {
			fmt.Printf("[OK] Model %s already installed\n", modelName)
			return nil
		}

		fmt.Printf("Installing model %s...\n", modelName)
		progressCallback := func(status string) {
			fmt.Println(status)
		}

		if err := ollama.Install(modelName, progressCallback); err != nil {
			return fmt.Errorf("failed to install model: %w", err)
		}

		fmt.Printf("[OK] Model %s installed successfully\n", modelName)
		return nil
	},
}

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage backend profiles",
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show and manage current profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backend := setup.Defaults.Backend
		profileName := setup.Defaults.Profile
		if profileName == "" {
			profileName = "default"
		}

		profile, err := config.GetBackendProfile(backend, profileName)
		if err != nil {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		fmt.Printf("Current: %s/%s\n", backend, profileName)
		if len(profile.AgentModels) > 0 {
			for agent, model := range profile.AgentModels {
				fmt.Printf("  %s: %s\n", agent, model)
			}
		}
		return nil
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list [backend]",
	Short: "List all profiles (or for specific backend)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		var backendFilter string
		if len(args) > 0 {
			backendFilter = args[0]
		}

		for backendName := range setup.Backend {
			if backendFilter != "" && backendName != backendFilter {
				continue
			}

			profiles, err := config.ListBackendProfiles(backendName)
			if err != nil {
				continue
			}

			for _, p := range profiles {
				if backendName == setup.Defaults.Backend && p == setup.Defaults.Profile {
					fmt.Printf("%s.%s (current)\n", backendName, p)
				} else {
					fmt.Printf("%s.%s\n", backendName, p)
				}
			}
		}
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [backend.profile]",
	Short: "Show profile configuration (current if not specified)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backend := setup.Defaults.Backend
		profileName := setup.Defaults.Profile
		if profileName == "" {
			profileName = "default"
		}

		if len(args) > 0 {
			parts := strings.Split(args[0], ".")
			if len(parts) != 2 {
				return fmt.Errorf("format must be <backend>.<profile> (e.g., openrouter.free)")
			}
			backend = parts[0]
			profileName = parts[1]
		}

		profile, err := config.GetBackendProfile(backend, profileName)
		if err != nil {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		fmt.Printf("%s/%s\n", backend, profileName)
		if len(profile.AgentModels) > 0 {
			for agent, model := range profile.AgentModels {
				fmt.Printf("  %s: %s\n", agent, model)
			}
		}
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <backend>.<profile>",
	Short: "Switch to a backend and profile",
	Long: `Switch to a specific backend and profile in one command.

Examples:
  gptcode profile use openrouter.free
  gptcode profile use groq.speed
  gptcode profile use ollama.local`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.Split(args[0], ".")
		if len(parts) != 2 {
			return fmt.Errorf("format must be <backend>.<profile> (e.g., openrouter.free)")
		}

		backend := parts[0]
		profile := parts[1]

		if err := config.SetConfig("defaults.backend", backend); err != nil {
			return fmt.Errorf("failed to set backend: %w", err)
		}

		if err := config.SetConfig("defaults.profile", profile); err != nil {
			return fmt.Errorf("failed to set profile: %w", err)
		}

		fmt.Printf("[OK] Switched to %s/%s\n", backend, profile)
		return nil
	},
}

var profilesListCmd = &cobra.Command{
	Use:   "list <backend>",
	Short: "List profiles for a backend",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := args[0]
		profiles, err := config.ListBackendProfiles(backend)
		if err != nil {
			return fmt.Errorf("failed to list profiles: %w", err)
		}

		encoder := json.NewEncoder(os.Stdout)
		if err := encoder.Encode(profiles); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
		return nil
	},
}

var profilesShowCmd = &cobra.Command{
	Use:   "show <backend> <profile>",
	Short: "Show profile configuration",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := args[0]
		profileName := args[1]

		profile, err := config.GetBackendProfile(backend, profileName)
		if err != nil {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		fmt.Printf("Profile: %s/%s\n", backend, profile.Name)
		if len(profile.AgentModels) == 0 {
			fmt.Println("  (no agent models configured)")
		} else {
			for agent, model := range profile.AgentModels {
				fmt.Printf("  %s: %s\n", agent, model)
			}
		}
		return nil
	},
}

var profilesCreateCmd = &cobra.Command{
	Use:   "create <backend> <profile-name>",
	Short: "Create new profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := args[0]
		name := args[1]

		if err := config.CreateBackendProfile(backend, name); err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}

		fmt.Printf("[OK] Created profile: %s/%s\n", backend, name)
		fmt.Println("\nConfigure agent models using:")
		fmt.Printf("  gptcode profiles set-agent %s %s router <model>\n", backend, name)
		fmt.Printf("  gptcode profiles set-agent %s %s query <model>\n", backend, name)
		fmt.Printf("  gptcode profiles set-agent %s %s editor <model>\n", backend, name)
		fmt.Printf("  gptcode profiles set-agent %s %s research <model>\n", backend, name)
		return nil
	},
}

var profilesSetAgentCmd = &cobra.Command{
	Use:   "set-agent <backend> <profile> <agent> <model>",
	Short: "Set agent model in profile",
	Long: `Set the model for a specific agent in a profile.

Agent types: router, query, editor, research

Example:
  gptcode profiles set-agent openrouter free router google/gemini-2.0-flash-exp:free`,
	Args: cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := args[0]
		profile := args[1]
		agent := args[2]
		model := args[3]

		if err := config.SetProfileAgentModel(backend, profile, agent, model); err != nil {
			return fmt.Errorf("failed to set agent model: %w", err)
		}

		fmt.Printf("[OK] Set %s/%s %s = %s\n", backend, profile, agent, model)
		return nil
	},
}

var profilesDeleteCmd = &cobra.Command{
	Use:   "delete <backend> <profile>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := args[0]
		profile := args[1]

		if err := config.DeleteBackendProfile(backend, profile); err != nil {
			return fmt.Errorf("failed to delete profile: %w", err)
		}

		fmt.Printf("[OK] Deleted profile: %s/%s\n", backend, profile)
		return nil
	},
}

var profilesUseCmd = &cobra.Command{
	Use:   "use <backend>.<profile>",
	Short: "Switch to a backend and profile",
	Long: `Switch to a specific backend and profile in one command.

Examples:
  gptcode profiles use openrouter.free
  gptcode profiles use groq.speed
  gptcode profiles use ollama.local`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.Split(args[0], ".")
		if len(parts) != 2 {
			return fmt.Errorf("format must be <backend>.<profile> (e.g., openrouter.free)")
		}

		backend := parts[0]
		profile := parts[1]

		if err := config.SetConfig("defaults.backend", backend); err != nil {
			return fmt.Errorf("failed to set backend: %w", err)
		}

		if err := config.SetConfig("defaults.profile", profile); err != nil {
			return fmt.Errorf("failed to set profile: %w", err)
		}

		fmt.Printf("[OK] Switched to %s/%s\n", backend, profile)
		return nil
	},
}

var feedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Record and analyze feedback for model performance",
}

var feedbackGoodCmd = &cobra.Command{
	Use:   "good",
	Short: "Record positive feedback",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, _ := cmd.Flags().GetString("backend")
		model, _ := cmd.Flags().GetString("model")
		agent, _ := cmd.Flags().GetString("agent")
		context, _ := cmd.Flags().GetString("context")
		task, _ := cmd.Flags().GetString("task")
		wrong, _ := cmd.Flags().GetString("wrong")
		correct, _ := cmd.Flags().GetString("correct")
		source, _ := cmd.Flags().GetString("source")
		kind, _ := cmd.Flags().GetString("kind")
		files, _ := cmd.Flags().GetStringSlice("files")

		event := feedback.Event{
			Sentiment:       feedback.SentimentGood,
			Backend:         backend,
			Model:           model,
			Agent:           agent,
			Context:         context,
			Task:            task,
			WrongResponse:   wrong,
			CorrectResponse: correct,
			Source:          source,
			Kind:            feedback.EventKind(kind),
			Files:           files,
		}

		if err := feedback.Record(event); err != nil {
			return fmt.Errorf("failed to record feedback: %w", err)
		}

		fmt.Println("[OK] Positive feedback recorded")
		return nil
	},
}

var feedbackBadCmd = &cobra.Command{
	Use:   "bad",
	Short: "Record negative feedback",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, _ := cmd.Flags().GetString("backend")
		model, _ := cmd.Flags().GetString("model")
		agent, _ := cmd.Flags().GetString("agent")
		context, _ := cmd.Flags().GetString("context")
		task, _ := cmd.Flags().GetString("task")
		wrong, _ := cmd.Flags().GetString("wrong")
		correct, _ := cmd.Flags().GetString("correct")
		source, _ := cmd.Flags().GetString("source")
		kind, _ := cmd.Flags().GetString("kind")
		files, _ := cmd.Flags().GetStringSlice("files")

		event := feedback.Event{
			Sentiment:       feedback.SentimentBad,
			Backend:         backend,
			Model:           model,
			Agent:           agent,
			Context:         context,
			Task:            task,
			WrongResponse:   wrong,
			CorrectResponse: correct,
			Source:          source,
			Kind:            feedback.EventKind(kind),
			Files:           files,
		}

		if err := feedback.Record(event); err != nil {
			return fmt.Errorf("failed to record feedback: %w", err)
		}

		fmt.Println("[OK] Negative feedback recorded")
		return nil
	},
}

var feedbackStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View feedback statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		events, err := feedback.LoadAll()
		if err != nil {
			return fmt.Errorf("failed to load feedback: %w", err)
		}

		if len(events) == 0 {
			fmt.Println("No feedback recorded yet")
			return nil
		}

		stats := feedback.Analyze(events)

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			return fmt.Errorf("failed to encode stats: %w", err)
		}

		return nil
	},
}

var feedbackExportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export anonymized feedback for sharing",
	Long: `Export feedback events with sensitive information removed.

Removed fields:
- Task descriptions
- Context/code snippets  
- File paths
- Responses (wrong/correct)

Kept fields (safe for sharing):
- Model ID
- Action (edit/review/plan/research)
- Language
- Complexity
- Success/failure
- Backend
- Date (without time)

Example:
  gptcode feedback export feedback-export.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath := "feedback-export.json"
		if len(args) > 0 {
			outputPath = args[0]
		}

		events, err := feedback.LoadAll()
		if err != nil {
			return fmt.Errorf("failed to load feedback: %w", err)
		}

		if len(events) == 0 {
			return fmt.Errorf("no feedback events found")
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			anonymized := feedback.Anonymize(events)
			fmt.Printf("\n Preview of anonymized data (%d events):\n\n", len(anonymized))
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(anonymized[:min(5, len(anonymized))]); err != nil {
				return fmt.Errorf("failed to encode preview: %w", err)
			}
			fmt.Printf("\n...and %d more events\n", max(0, len(anonymized)-5))
			fmt.Printf("\nRun without --dry-run to export to %s\n", outputPath)
			return nil
		}

		if err := feedback.ExportAnonymized(outputPath); err != nil {
			return fmt.Errorf("failed to export feedback: %w", err)
		}

		fmt.Printf("[OK] Exported %d anonymized feedback events to %s\n", len(events), outputPath)
		return nil
	},
}

var feedbackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit feedback event via flags or JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonStr, _ := cmd.Flags().GetString("json")
		if jsonStr != "" {
			var e feedback.Event
			if jsonStr == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				jsonStr = string(data)
			}
			if err := json.Unmarshal([]byte(jsonStr), &e); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			if err := feedback.Record(e); err != nil {
				return fmt.Errorf("failed to record feedback: %w", err)
			}
			fmt.Println("[OK] Feedback submitted")
			return nil
		}

		backend, _ := cmd.Flags().GetString("backend")
		model, _ := cmd.Flags().GetString("model")
		agent, _ := cmd.Flags().GetString("agent")
		context, _ := cmd.Flags().GetString("context")
		task, _ := cmd.Flags().GetString("task")
		wrong, _ := cmd.Flags().GetString("wrong")
		correct, _ := cmd.Flags().GetString("correct")
		source, _ := cmd.Flags().GetString("source")
		kind, _ := cmd.Flags().GetString("kind")
		files, _ := cmd.Flags().GetStringSlice("files")
		captureDiff, _ := cmd.Flags().GetBool("capture-diff")
		sent, _ := cmd.Flags().GetString("sentiment")

		e := feedback.Event{
			Backend:         backend,
			Model:           model,
			Agent:           agent,
			Context:         context,
			Task:            task,
			WrongResponse:   wrong,
			CorrectResponse: correct,
			Source:          source,
			Kind:            feedback.EventKind(kind),
			Files:           files,
		}
		if sent != "" {
			e.Sentiment = feedback.Sentiment(sent)
		}
		if captureDiff {
			if _, err := exec.LookPath("git"); err == nil {
				cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
				if err := cmd.Run(); err == nil {
					diffCmd := exec.Command("git", "diff")
					diffBytes, _ := diffCmd.Output()
					if len(diffBytes) > 0 {
						dir := filepath.Join(os.Getenv("HOME"), ".gptcode", "diffs")
						_ = os.MkdirAll(dir, 0755)
						name := time.Now().Format("20060102-150405") + ".patch"
						path := filepath.Join(dir, name)
						_ = os.WriteFile(path, diffBytes, 0644)
						e.DiffPath = path
					}
				}
			}
		}
		if err := feedback.Record(e); err != nil {
			return fmt.Errorf("failed to record feedback: %w", err)
		}
		fmt.Println("[OK] Feedback submitted")
		return nil
	},
}

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Demos and recordings",
}

var demoFeedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Feedback capture demos",
}

var demoFeedbackCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"feedback:create", "feedback.create"},
	Short:   "Generate feedback demos (casts + GIFs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		tries, _ := cmd.Flags().GetInt("tries")
		if repo == "" {
			repo = "."
		}
		sh := exec.Command("bash", "docs/scripts/build_demos.sh")
		sh.Dir = repo
		sh.Env = append(os.Environ(), fmt.Sprintf("TRIES=%d", tries))
		sh.Stdout = os.Stdout
		sh.Stderr = os.Stderr
		if err := sh.Run(); err != nil {
			return fmt.Errorf("failed to build demos: %w", err)
		}
		fmt.Println("[OK] Demos built in docs/assets")
		return nil
	},
}

var feedbackHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Install shell hooks for automatic feedback capture",
}

func init() {
	backendCmd.AddCommand(backendListCmd)
	backendCmd.AddCommand(backendShowCmd)
	backendCmd.AddCommand(backendUseCmd)
	backendCmd.AddCommand(backendCreateCmd)
	backendCmd.AddCommand(backendDeleteCmd)

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)

	rootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesShowCmd)
	profilesCmd.AddCommand(profilesCreateCmd)
	profilesCmd.AddCommand(profilesSetAgentCmd)
	profilesCmd.AddCommand(profilesDeleteCmd)
	profilesCmd.AddCommand(profilesUseCmd)

	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)

	rootCmd.AddCommand(feedbackCmd)
	feedbackCmd.AddCommand(feedbackGoodCmd)
	feedbackCmd.AddCommand(feedbackBadCmd)
	feedbackCmd.AddCommand(feedbackStatsCmd)
	feedbackCmd.AddCommand(feedbackExportCmd)
	feedbackCmd.AddCommand(feedbackSubmitCmd)
	feedbackCmd.AddCommand(feedbackHookCmd)

	rootCmd.AddCommand(demoCmd)
	demoCmd.AddCommand(demoFeedbackCmd)
	demoFeedbackCmd.AddCommand(demoFeedbackCreateCmd)
	demoFeedbackCreateCmd.Flags().String("repo", ".", "Repository root containing docs/scripts/build_demos.sh")
	demoFeedbackCreateCmd.Flags().Int("tries", 3, "Max attempts to capture good demos")

	feedbackGoodCmd.Flags().String("backend", "", "Backend used")
	feedbackGoodCmd.Flags().String("model", "", "Model used")
	feedbackGoodCmd.Flags().String("agent", "", "Agent type (router, query, editor, research)")
	feedbackGoodCmd.Flags().String("context", "", "Additional context")
	feedbackGoodCmd.Flags().String("task", "", "Task/query from user")
	feedbackGoodCmd.Flags().String("wrong", "", "Wrong response to task")
	feedbackGoodCmd.Flags().String("correct", "", "Correct response for task")
	feedbackGoodCmd.Flags().String("source", "", "Source of feedback (cli/tool)")
	feedbackGoodCmd.Flags().String("kind", "", "Event kind (command,text,file_edit,review_note)")
	feedbackGoodCmd.Flags().StringSlice("files", nil, "Related files")

	feedbackBadCmd.Flags().String("backend", "", "Backend used")
	feedbackBadCmd.Flags().String("model", "", "Model used")
	feedbackBadCmd.Flags().String("agent", "", "Agent type (router, query, editor, research)")
	feedbackBadCmd.Flags().String("context", "", "Additional context")
	feedbackBadCmd.Flags().String("task", "", "Task/query from user")
	feedbackBadCmd.Flags().String("wrong", "", "Wrong response to task")
	feedbackBadCmd.Flags().String("correct", "", "Correct response for task")
	feedbackBadCmd.Flags().String("source", "", "Source of feedback (cli/tool)")
	feedbackBadCmd.Flags().String("kind", "", "Event kind (command,text,file_edit,review_note)")
	feedbackBadCmd.Flags().StringSlice("files", nil, "Related files")

	feedbackSubmitCmd.Flags().String("json", "", "JSON payload or '-' to read from stdin")
	feedbackSubmitCmd.Flags().String("backend", "", "Backend used")
	feedbackSubmitCmd.Flags().String("model", "", "Model used")
	feedbackSubmitCmd.Flags().String("agent", "", "Agent type (router, query, editor, research)")
	feedbackSubmitCmd.Flags().String("context", "", "Additional context")
	feedbackSubmitCmd.Flags().String("task", "", "Task/query from user")
	feedbackSubmitCmd.Flags().String("wrong", "", "Wrong response to task")
	feedbackSubmitCmd.Flags().String("correct", "", "Correct response for task")
	feedbackSubmitCmd.Flags().String("source", "", "Source of feedback (cli/tool)")
	feedbackSubmitCmd.Flags().String("kind", "", "Event kind (command,text,file_edit,review_note)")
	feedbackSubmitCmd.Flags().StringSlice("files", nil, "Related files")
	feedbackSubmitCmd.Flags().Bool("capture-diff", false, "Also capture git diff to file and link it")
	feedbackSubmitCmd.Flags().String("sentiment", "", "good|bad")

	feedbackExportCmd.Flags().Bool("dry-run", false, "Preview anonymized data without exporting")

	modelsCmd.AddCommand(modelsUpdateCmd)
	modelsCmd.AddCommand(modelsSearchCmd)
	modelsCmd.AddCommand(modelsInstallCmd)

	modelsSearchCmd.Flags().StringP("backend", "b", "openrouter", "Backend to search (openrouter, groq, ollama, etc)")
	modelsSearchCmd.Flags().StringP("agent", "a", "", "Agent type (router, query, editor, research)")
}

var feedbackHookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell hook",
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, _ := cmd.Flags().GetString("shell")
		withDiff, _ := cmd.Flags().GetBool("with-diff")
		andSource, _ := cmd.Flags().GetBool("and-source")
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		hookDir := filepath.Join(home, ".gptcode")
		if err := os.MkdirAll(hookDir, 0755); err != nil {
			return err
		}
		switch shell {
		case "zsh":
			hookPath := filepath.Join(hookDir, "feedback_hook.zsh")
			hook := `chu_mark_suggestion_widget() {
	local f="$HOME/.gptcode/last_suggestion_cmd"
	print -r -- "$BUFFER" > "$f"
zle -M "Suggestion captured"
}

zle -N chu_mark_suggestion_widget
bindkey -M emacs "^G" chu_mark_suggestion_widget
bindkey -M viins "^G" chu_mark_suggestion_widget

preexec_chu_feedback() {
	local cmd="$1"
	local sfile="$HOME/.gptcode/last_suggestion_cmd"
	if [[ -f "$sfile" ]]; then
		print -r -- "$(<"$sfile")" > "$HOME/.gptcode/.pending_wrong"
		print -r -- "$cmd" > "$HOME/.gptcode/.pending_correct"
	fi
}

precmd_chu_feedback() {
	local wrongf="$HOME/.gptcode/.pending_wrong"
	local correctf="$HOME/.gptcode/.pending_correct"
	if [[ -f "$wrongf" && -f "$correctf" ]]; then
		local wrong="$(<"$wrongf")"
		local correct="$(<"$correctf")"
		local files=""
		if command -v git >/dev/null 2>&1; then
			if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
				files=$(git diff --name-only)
			fi
		fi
		local -a args
		args=(feedback submit --sentiment=bad --kind=command --source=shell --agent=editor --wrong="$wrong" --correct="$correct")
		
		if [[ -n "$files" ]]; then
			local f
			for f in ${(f)files}; do
				args+=(--files "$f")
			done
		fi
		if [[ %WITH_DIFF% == 1 ]]; then args+=(--capture-diff); fi
		gptcode $args >/dev/null 2>&1
		rm -f "$wrongf" "$correctf" "$HOME/.gptcode/last_suggestion_cmd"
	fi
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec preexec_chu_feedback
add-zsh-hook precmd precmd_chu_feedback
`
			if withDiff {
				hook = strings.ReplaceAll(hook, "%WITH_DIFF%", "1")
				hook = strings.ReplaceAll(hook, "%FISH_DIFF%", "set args $args --capture-diff")
			} else {
				hook = strings.ReplaceAll(hook, "%WITH_DIFF%", "0")
				hook = strings.ReplaceAll(hook, "%FISH_DIFF%", "")
			}
			if err := os.WriteFile(hookPath, []byte(hook), 0644); err != nil {
				return err
			}
			rcPath := filepath.Join(home, ".zshrc")
			var rc string
			if data, err := os.ReadFile(rcPath); err == nil {
				rc = string(data)
			}
			line := "source $HOME/.gptcode/feedback_hook.zsh"
			if !strings.Contains(rc, line) {
				rc += "\n" + line + "\n"
				if err := os.WriteFile(rcPath, []byte(rc), 0644); err != nil {
					return err
				}
			}
			fmt.Println("[OK] Installed zsh hook. Restart your shell or run: source ~/.zshrc")
			if andSource {
				_ = exec.Command("zsh", "-ic", "source ~/.zshrc").Run()
			}
			return nil
		case "bash":
			hookPath := filepath.Join(hookDir, "feedback_hook.bash")
			hook := `chu_mark_suggestion_bash() {
	local f="$HOME/.gptcode/last_suggestion_cmd"
	printf "%s" "$READLINE_LINE" > "$f"
}

bind -x '"\C-g":"chu_mark_suggestion_bash"'

chu_preexec() {
	local cmd="$1"
	local sfile="$HOME/.gptcode/last_suggestion_cmd"
	if [[ -f "$sfile" ]]; then
		cat "$sfile" > "$HOME/.gptcode/.pending_wrong"
		printf "%s" "$cmd" > "$HOME/.gptcode/.pending_correct"
	fi
}
trap 'chu_preexec "$BASH_COMMAND"' DEBUG

chu_precmd() {
	local wrongf="$HOME/.gptcode/.pending_wrong"
	local correctf="$HOME/.gptcode/.pending_correct"
	if [[ -f "$wrongf" && -f "$correctf" ]]; then
		local wrong
		wrong="$(cat "$wrongf")"
		local correct
		correct="$(cat "$correctf")"
		local files=""
		if command -v git >/dev/null 2>&1; then
			if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
				files="$(git diff --name-only)"
			fi
		fi
		local -a args=(feedback submit --sentiment=bad --kind=command --source=shell --agent=editor --wrong="$wrong" --correct="$correct")
		if [[ %WITH_DIFF% == 1 ]]; then args+=(--capture-diff); fi
		if [[ -n "$files" ]]; then
			while IFS= read -r f; do args+=(--files "$f"); done <<< "$files"
		fi
		gptcode "${args[@]}" >/dev/null 2>&1
		rm -f "$wrongf" "$correctf" "$HOME/.gptcode/last_suggestion_cmd"
	fi
}

PROMPT_COMMAND="chu_precmd; $PROMPT_COMMAND"
`
			if err := os.WriteFile(hookPath, []byte(hook), 0644); err != nil {
				return err
			}
			rcPath := filepath.Join(home, ".bashrc")
			var rc string
			if data, err := os.ReadFile(rcPath); err == nil {
				rc = string(data)
			}
			line := ". \"$HOME/.gptcode/feedback_hook.bash\""
			if !strings.Contains(rc, line) {
				rc += "\n" + line + "\n"
				if err := os.WriteFile(rcPath, []byte(rc), 0644); err != nil {
					return err
				}
			}
			fmt.Println("[OK] Installed bash hook. Restart your shell or run: source ~/.bashrc")
			if andSource {
				_ = exec.Command("bash", "-ic", "source ~/.bashrc").Run()
			}
			return nil
		case "fish":
			confDir := filepath.Join(home, ".config", "fish", "conf.d")
			if err := os.MkdirAll(confDir, 0755); err != nil {
				return err
			}
			hookPath := filepath.Join(confDir, "chu_feedback.fish")
			hook := `function chufb_mark_suggestion
	set -l f "$HOME/.gptcode/last_suggestion_cmd"
	commandline -b > $f
end
bind \cg chufb_mark_suggestion

function chufb_preexec --on-event fish_preexec
	set -l cmd $argv
	set -l sfile "$HOME/.gptcode/last_suggestion_cmd"
	if test -f $sfile
		cat $sfile > "$HOME/.gptcode/.pending_wrong"
		printf "%s" "$cmd" > "$HOME/.gptcode/.pending_correct"
	end
end

function chufb_postexec --on-event fish_postexec
	set -l wrongf "$HOME/.gptcode/.pending_wrong"
	set -l correctf "$HOME/.gptcode/.pending_correct"
	if test -f $wrongf; and test -f $correctf
		set -l wrong (cat $wrongf)
		set -l correct (cat $correctf)
		set -l files
		if type -q git
			if git rev-parse --is-inside-work-tree >/dev/null 2>&1
				set files (git diff --name-only)
			end
		end
		set -l args feedback submit --sentiment=bad --kind=command --source=shell --agent=editor --wrong="$wrong" --correct="$correct"
		%FISH_DIFF%
		for f in $files
			set args $args --files $f
		end
		gptcode $args >/dev/null 2>&1
		rm -f $wrongf $correctf "$HOME/.gptcode/last_suggestion_cmd"
	end
end
`
			if err := os.WriteFile(hookPath, []byte(hook), 0644); err != nil {
				return err
			}
			fmt.Println("[OK] Installed fish hook. Restart fish or open a new session")
			if andSource {
				_ = exec.Command("fish", "-ic", "source ~/.config/fish/conf.d/chu_feedback.fish").Run()
			}
			return nil
		default:
			return fmt.Errorf("unsupported shell: %s", shell)
		}
	},
}

func init() {
	feedbackHookInstallCmd.Flags().String("shell", "zsh", "Shell to install hook for")
	feedbackHookInstallCmd.Flags().Bool("with-diff", false, "Also capture git diff patch to file")
	feedbackHookInstallCmd.Flags().Bool("and-source", false, "Attempt to source shell rc after install")
	feedbackHookCmd.AddCommand(feedbackHookInstallCmd)
}

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Interactive chat with optional initial message",
	Long: `Interactive chat mode - always stays open for follow-up questions.

With initial message:
   gptcode chat "investigate 700GB system data"
   # Processes message and stays open for follow-up
   
Without message:
   gptcode chat
   # Starts interactive session

REPL Commands:
  /exit, /quit   - Exit chat
  /clear         - Clear conversation history
  /save <file>   - Save conversation
  /load <file>   - Load conversation
  /context       - Show context stats
  /files         - List files in context
  /history       - Show history
  /help          - Show help`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we have a message argument or stdin input
		var initialMessage string
		if len(args) > 0 && args[0] != "" {
			initialMessage = args[0]
		} else if !isInteractiveTTY() {
			// Check for piped input
			stdinBytes, _ := io.ReadAll(os.Stdin)
			initialMessage = string(stdinBytes)
		}

		// Always start REPL, with optional initial message
		replInstance, err := repl.NewChatREPL(8000, 50) // 8k tokens, 50 messages
		if err != nil {
			return fmt.Errorf("failed to initialize chat REPL: %w", err)
		}
		return replInstance.RunWithInitialMessage(initialMessage)
	},
}

// isInteractiveTTY returns true if we're running in an interactive terminal
func isInteractiveTTY() bool {
	cmd := exec.Command("tty", "-s")
	return cmd.Run() == nil
}

var tddCmd = &cobra.Command{
	Use:   "tdd",
	Short: "TDD mode (tests → implementation → refine)",
	RunE: func(cmd *cobra.Command, args []string) error {
		builder, provider, model, err := newBuilderAndLLM("general", "tdd", "")
		if err != nil {
			return err
		}
		desc := ""
		if len(args) > 0 {
			desc = args[0]
		}
		return modes.RunTDD(builder, provider, model, desc)
	},
}

var researchCmd = &cobra.Command{
	Use:   "research [question]",
	Short: "Research mode - document codebase and understand architecture",
	Long: `Research mode uses subagents to explore the codebase and document findings.
Provide a research question or area to investigate.

Example: gptcode research "How does authentication work?"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return modes.RunResearch(args)
	},
}

var planCmd = &cobra.Command{
	Use:   "plan [task]",
	Short: "Plan mode - create detailed implementation plan with phases",
	Long: `Plan mode creates a detailed implementation plan through interactive research.
Provide a task description or path to a ticket/spec file.

Example: gptcode plan "Add user authentication"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return modes.RunPlan(args)
	},
}

var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Execute tasks with follow-up support",
	Long: `Execute mode for general operational tasks with AI assistance or direct command execution.

Two modes available:

1. AI-assisted mode (new, default when no args provided):
   gptcode run
   gptcode run --help  # Show REPL commands
   gptcode run --once   # Force single-shot AI mode

2. Direct REPL mode with command history:
   gptcode run "command and args" --raw
   gptcode run "curl https://api.github.com" --raw

AI-assisted mode provides:
- Command suggestions and execution
- Follow-up support with context preservation
- Command history and output reference ($1, $2, $last)
- Directory and environment variable management

REPL Commands:
  /exit, /quit   - Exit run session
  /help          - Show help
  /history       - Show command history
  /output <id>   - Show output of previous command
  /cd <dir>      - Change directory
  /env           - Show/set environment variables

Examples:
  gptcode run                # Start AI-assisted mode
  gptcode run "deploy to staging" --once  # Single AI execution
  gptcode run "docker ps"    --raw     # Direct command REPL`,
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, _ := cmd.Flags().GetBool("raw")
		once, _ := cmd.Flags().GetBool("once")

		// Raw mode with command references
		if raw {
			// If we have a task, we want to run it and exit
			if len(args) > 0 && args[0] != "" {
				task := strings.Join(args, " ")
				return repl.RunSingleShotCommand(task)
			}
			// Start raw REPL mode (no AI, just command execution)
			repl := repl.NewRunREPL(20) // Track last 20 commands
			return repl.Run()
		}

		// Check if we have a task argument or stdin input, or if we're in an interactive TTY
		var input string
		if len(args) > 0 && args[0] != "" {
			input = strings.Join(args, " ")
		} else if !isInteractiveTTY() {
			// Check for piped input
			stdinBytes, _ := io.ReadAll(os.Stdin)
			input = string(stdinBytes)
		}

		// If we have input or --once flag, use single-shot AI mode
		if input != "" || once {
			// Auto-detect agent type from input text
			agentType := live.AgentTypeFromInput(input)
			agentID := live.GetAgentIDWithType(agentType)

			// Resolve model first so we can send it in all connect payloads
			builder, provider, model, err := newBuilderAndLLM("general", "run", "")
			if err != nil {
				return err
			}

			// Try to connect to Live Dashboard via HTTP API
			reportConfig := live.DefaultReportConfig()
			reportConfig.Model = model
			liveURL := os.Getenv("GPTCODE_LIVE_URL")
			if liveURL != "" {
				reportConfig.SetBaseURL(liveURL)
			}

			// Report connect to Live
			if err := reportConfig.Connect(agentID, agentType, input); err != nil {
				fmt.Printf("⚠️  Live connect error: %v\n", err)
			} else {
				fmt.Printf("🔗 Reported to Live Dashboard: %s\n", reportConfig.BaseURL)
			}

			// Report first step
			reportConfig.Step("Starting: "+input, "start")

			// WebSocket connection for commands (optional)
			var liveClient *live.Client
			wsURL := os.Getenv("GPTCODE_LIVE_URL")
			if wsURL != "" && strings.HasPrefix(wsURL, "ws") {
				liveClient = live.NewClient(wsURL, agentID)
				liveClient.SetAgentType(agentType)
				liveClient.SetTask(input)
				liveClient.SetModel(model)
				if err := liveClient.Connect(); err != nil {
					fmt.Printf("⚠️  Live WS error: %v\n", err)
				} else {
					live.SetGlobalClient(liveClient)
				}
			}

			err = modes.RunExecute(builder, provider, model, strings.Fields(input), liveClient, reportConfig)

			// Report completion to Live
			if err != nil {
				reportConfig.Step("Error: "+err.Error(), "error")
			} else {
				reportConfig.Step("Completed: "+input, "complete")
			}
			reportConfig.Disconnect()

			// Send end event via WebSocket
			if liveClient != nil {
				liveClient.SendExecutionStep("complete", "Task completed", map[string]interface{}{
					"task":  input,
					"error": err != nil,
				})
			}

			return err
		}

		// Start AI-assisted REPL mode - combine run REPL with AI processing
		repl := repl.NewRunREPL(20)
		return repl.Run()
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously execute tasks from a roadmap file",
	Long: `Watch mode reads a roadmap file and executes tasks one by one.

Looks for roadmap files in this order:
  _roadmap.md, .gptcode/roadmap.md, ROADMAP.md, roadmap.md, TODO.md

Tasks are markdown checkboxes: - [ ] task description
Completed tasks are marked: - [x] task description

The agent stays connected to the Live Dashboard between tasks,
allowing real-time monitoring from the /live page.

Examples:
  gt watch
  gt watch --file my-tasks.md
  gt watch --interval 60s
  gt watch --max 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		roadmapFile, _ := cmd.Flags().GetString("file")
		intervalStr, _ := cmd.Flags().GetString("interval")
		maxTasks, _ := cmd.Flags().GetInt("max")

		interval := 30 * time.Second
		if intervalStr != "" {
			parsed, err := time.ParseDuration(intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}
			interval = parsed
		}

		builder, provider, model, err := newBuilderAndLLM("general", "run", "")
		if err != nil {
			return err
		}

		return modes.WatchExecute(builder, provider, model, modes.WatchConfig{
			RoadmapFile: roadmapFile,
			Interval:    interval,
			MaxTasks:    maxTasks,
		})
	},
}

var goCmd = &cobra.Command{
	Use:   "go [task]",
	Short: "Direct execution - ask AI and get immediate answer (no tools)",
	Long: `Direct execution mode - gets AI response without tool calling loop.

Examples:
  gt go hello world
  gt go "what is Go programming language"
  gt go "write a simple HTTP server in Python"
  gt go "explain this error: cannot find package"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		builder, provider, model, err := newBuilderAndLLM("general", "go", "")
		if err != nil {
			return err
		}
		return modes.RunGo(builder, provider, model, args)
	},
}

func init() {
	runCmd.Flags().Bool("raw", false, "Run direct command REPL mode (no AI)")
	runCmd.Flags().Bool("once", false, "Run single-shot mode")

	watchCmd.Flags().String("file", "", "Path to roadmap file (default: auto-detect)")
	watchCmd.Flags().String("interval", "30s", "Wait duration between tasks")
	watchCmd.Flags().Int("max", 0, "Maximum number of tasks to execute (0 = unlimited)")
}

var featureCmd = &cobra.Command{
	Use:   "feature [description]",
	Short: "Generate tests + implementation for a feature (auto-detects language)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := detectLanguage()
		fmt.Printf("Detected language: %s\n", lang)

		builder, provider, model, err := newBuilderAndLLM(lang, "tdd", args[0])
		if err != nil {
			return err
		}

		if lang == "elixir" {
			return elixir.RunFeatureElixir(builder, provider, model)
		}

		// Default to generic TDD for other languages
		return modes.RunTDD(builder, provider, model, args[0])
	},
}

var mlCmd = &cobra.Command{
	Use:   "ml",
	Short: "Machine learning model management",
	Long: `Manage machine learning models for GPTCode.

Available commands:
  list  - List available models
  train - Train a model
  test  - Test a trained model

Examples:
  gptcode ml list
  gptcode ml train complexity_detection
  gptcode ml test complexity_detection "implement oauth2"`,
}

var mlListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available ML models",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		trainer := ml.NewTrainer(cwd)
		trainer.ListModels()
		return nil
	},
}

var mlTrainCmd = &cobra.Command{
	Use:   "train <model-name>",
	Short: "Train an ML model",
	Long: `Train a machine learning model.

Examples:
  gptcode ml train complexity_detection`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		trainer := ml.NewTrainer(cwd)
		modelName := args[0]

		if err := trainer.Train(modelName); err != nil {
			return fmt.Errorf("training failed: %w", err)
		}

		return nil
	},
}

var mlTestCmd = &cobra.Command{
	Use:   "test <model-name> [query]",
	Short: "Test a trained ML model",
	Long: `Test a trained model with example queries.

Without a query, runs pre-defined test examples.
With a query, tests that specific input.

Examples:
  gptcode ml test complexity_detection
  gptcode ml test complexity_detection "fix typo in readme"
  gptcode ml test complexity_detection "implement oauth2 with google"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		trainer := ml.NewTrainer(cwd)
		modelName := args[0]

		var query string
		if len(args) > 1 {
			query = args[1]
		}

		if err := trainer.Test(modelName, query); err != nil {
			return fmt.Errorf("test failed: %w", err)
		}

		return nil
	},
}

var mlEvalCmd = &cobra.Command{
	Use:   "eval <model-name>",
	Short: "Evaluate a trained ML model on a dataset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		trainer := ml.NewTrainer(cwd)
		file, _ := cmd.Flags().GetString("file")
		if err := trainer.Eval(args[0], file); err != nil {
			return fmt.Errorf("eval failed: %w", err)
		}
		return nil
	},
}

var mlPredictCmd = &cobra.Command{
	Use:   "predict [model-name] <text>",
	Short: "Predict using embedded Go model (no Python)",
	Long: `Predict using embedded Go model (no Python).

If model-name is omitted, defaults to 'complexity_detection'.

Examples:
  gptcode ml predict "fix typo in readme"
  gptcode ml predict complexity_detection "implement oauth"
  gptcode ml predict router_agent "explain this code"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var modelName, text string

		if len(args) == 1 {
			modelName = "complexity_detection"
			text = args[0]
		} else {
			modelName = args[0]
			text = strings.Join(args[1:], " ")
		}

		p, err := ml.LoadEmbedded(modelName)
		if err != nil {
			return err
		}

		label, probs := p.Predict(text)
		fmt.Printf("Model: %s\n", modelName)
		fmt.Printf("Prediction: %s\n", label)
		fmt.Println("Probabilities:")

		for _, pair := range ml.SortedProbs(probs) {
			fmt.Printf("  %-12s %s\n", pair[0]+":", pair[1])
		}
		return nil
	},
}

func init() {
	mlCmd.AddCommand(mlListCmd)
	mlCmd.AddCommand(mlTrainCmd)
	mlCmd.AddCommand(mlTestCmd)
	mlCmd.AddCommand(mlEvalCmd)
	mlCmd.AddCommand(mlPredictCmd)
	mlEvalCmd.Flags().StringP("file", "f", "", "Path to eval CSV with columns: message,label")
}

var reviewCmd = &cobra.Command{
	Use:   "review [file or directory]",
	Short: "Review code for bugs, security issues, and improvements",
	Long: `Review mode performs detailed code analysis.

Review a specific file:
  gptcode review main.go
  gptcode review src/auth.go

Review a directory:
  gptcode review .
  gptcode review ./src

Focus on specific aspects:
  gptcode review main.go --focus security
  gptcode review . --focus performance
  gptcode review src/ --focus "error handling"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}

		focus, _ := cmd.Flags().GetString("focus")

		return modes.RunReview(modes.ReviewOptions{
			Target: target,
			Focus:  focus,
		})
	},
}

func init() {
	reviewCmd.Flags().StringP("focus", "f", "", "Focus area for review (e.g., security, performance, error handling)")
}

func detectLanguage() string {
	if _, err := os.Stat("mix.exs"); err == nil {
		return "elixir"
	}
	if _, err := os.Stat("Gemfile"); err == nil {
		return "ruby"
	}
	if _, err := os.Stat("go.mod"); err == nil {
		return "go"
	}
	if _, err := os.Stat("package.json"); err == nil {
		return "typescript"
	}
	if _, err := os.Stat("requirements.txt"); err == nil {
		return "python"
	}
	if _, err := os.Stat("Cargo.toml"); err == nil {
		return "rust"
	}
	return "unknown"
}
