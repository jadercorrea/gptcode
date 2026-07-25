package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gptcode/internal/config"
	"gptcode/internal/intelligence"
	"gptcode/internal/live"
	"gptcode/internal/llm"
	"gptcode/internal/modes"

	"golang.org/x/term"
)

var doCmd = &cobra.Command{
	Use:   "do [task]",
	Short: "Execute a task autonomously",
	Long: `Execute a task autonomously using the agent system.

Examples:
  gptcode do "add error handling to main.go"
  gptcode do "read docs/README.md and create a getting-started guide"
  gptcode do "unify all feature files in /guides"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := strings.Join(args, " ")

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		verbose, _ := cmd.Flags().GetBool("verbose")
		maxAttempts, _ := cmd.Flags().GetInt("max-attempts")
		supervised, _ := cmd.Flags().GetBool("supervised")
		interactive, _ := cmd.Flags().GetBool("interactive")

		if verbose {
			fmt.Fprintf(os.Stderr, "Task: %s\n", task)
			fmt.Fprintf(os.Stderr, "Dry-run: %v\n", dryRun)
			fmt.Fprintf(os.Stderr, "Max attempts: %d\n\n", maxAttempts)
		}

		if dryRun {
			return runDoAnalysis(cmd.Context(), task, verbose)
		}

		return runDoExecutionWithRetry(cmd.Context(), task, verbose, maxAttempts, supervised, interactive)
	},
}

func init() {
	rootCmd.AddCommand(doCmd)

	doCmd.Flags().Bool("dry-run", false, "Show analysis and plan without executing")
	doCmd.Flags().BoolP("verbose", "v", false, "Show detailed progress")
	doCmd.Flags().Int("max-attempts", 3, "Maximum retry attempts with different models")
	doCmd.Flags().Bool("supervised", false, "Require manual approval before implementation")
	doCmd.Flags().BoolP("interactive", "i", false, "Prompt for model selection when multiple options are similar")
}

func runDoAnalysis(ctx context.Context, task string, verbose bool) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	queryModel := backendCfg.GetModelForAgent("query")

	fmt.Println("=== Task Analysis ===")
	fmt.Printf("Task: %s\n\n", task)

	analysisPrompt := fmt.Sprintf(`Analyze this task and determine:
1. Primary intent (create, read, update, refactor, unify, etc.)
2. Files that need to be read (if any)
3. Files that will be created or modified
4. Key steps required
5. Estimated complexity (1-10)

Task: %s

Provide a brief analysis.`, task)

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You analyze tasks to determine requirements and complexity.",
		UserPrompt:   analysisPrompt,
		Model:        queryModel,
	})
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	fmt.Println(resp.Text)
	fmt.Println("\n=== Execution Plan ===")
	fmt.Println("This would create a detailed plan and execute using guided mode.")
	fmt.Println("\nTo execute: run without --dry-run flag")

	return nil
}

func runDoExecutionWithRetry(ctx context.Context, task string, verbose bool, maxAttempts int, supervised bool, interactive bool) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	selection, err := selectDoModels(setup)
	if err != nil {
		return err
	}
	editorBackend := selection.editorBackend
	editorModel := selection.editorModel
	editorReason := selection.editorReason
	queryBackend := selection.queryBackend
	queryModel := selection.queryModel
	queryReason := selection.queryReason

	if verbose {
		fmt.Fprintf(os.Stderr, "Auto-selected models:\n")
		fmt.Fprintf(os.Stderr, "   Editor: %s/%s - %s\n", editorBackend, editorModel, editorReason)
		fmt.Fprintf(os.Stderr, "   Query: %s/%s - %s\n", queryBackend, queryModel, queryReason)
		fmt.Fprintf(os.Stderr, "\n")
	}

	currentBackend := editorBackend
	currentEditorModel := editorModel

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && verbose {
			fmt.Fprintf(os.Stderr, "\n=== Attempt %d/%d ===\n", attempt, maxAttempts)
		}

		startTime := time.Now()
		err := runDoExecution(ctx, task, verbose, supervised, setup, currentBackend, currentEditorModel)
		elapsed := time.Since(startTime).Milliseconds()

		if err == nil {
			_ = intelligence.RecordExecution(intelligence.TaskExecution{
				Task:      task,
				Backend:   currentBackend,
				Model:     currentEditorModel,
				Success:   true,
				LatencyMs: elapsed,
			})

			if verbose {
				fmt.Fprintf(os.Stderr, "\n[OK] Task completed successfully\n")
			}
			return nil
		}

		_ = intelligence.RecordExecution(intelligence.TaskExecution{
			Task:    task,
			Backend: currentBackend,
			Model:   currentEditorModel,
			Success: false,
			Error:   err.Error(),
		})

		errMsg := err.Error()
		looksLikeToolError := strings.Contains(errMsg, "tool") || strings.Contains(errMsg, "function") ||
			strings.Contains(errMsg, "not available") || strings.Contains(errMsg, "not supported")

		looksLikeAPIError := strings.Contains(errMsg, "API error") || strings.Contains(errMsg, "Provider returned error") ||
			strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "429") ||
			strings.Contains(errMsg, "empty movements array") || strings.Contains(errMsg, "decomposition failed")

		if !looksLikeToolError && !looksLikeAPIError {
			return fmt.Errorf("task failed: %w", err)
		}

		if attempt >= maxAttempts {
			// If we hit max attempts with API errors, offer to try a different backend
			if looksLikeAPIError && shouldPromptForRetry(interactive) {
				fmt.Fprintf(os.Stderr, "\n  All %d attempts failed with API/rate limit errors.\n", maxAttempts)
				fmt.Fprintf(os.Stderr, "\nAvailable backends:\n")
				var backends []string
				for backend := range setup.Backend {
					backends = append(backends, backend)
					fmt.Fprintf(os.Stderr, "  - %s\n", backend)
				}
				if len(backends) == 0 {
					return fmt.Errorf("no backends configured")
				}
				fmt.Fprintf(os.Stderr, "\nWould you like to try a different backend?\n")
				fmt.Fprintf(os.Stderr, "Enter backend name (or 'quit' to stop): ")
				var input string
				fmt.Scanln(&input)
				if input == "quit" || input == "q" || input == "" {
					return fmt.Errorf("task failed after %d attempts: %w", maxAttempts, err)
				}
				if _, exists := setup.Backend[input]; !exists {
					return fmt.Errorf("backend '%s' not found", input)
				}
				currentBackend = input
				backendCfg := setup.Backend[currentBackend]
				currentEditorModel = backendCfg.GetModelForAgent("editor")
				if currentEditorModel == "" {
					currentEditorModel = backendCfg.DefaultModel
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "\nSwitching to %s/%s...\n", currentBackend, currentEditorModel)
				}
				// Reset attempt counter to give the new backend a fair chance
				attempt = 0
				continue
			}
			return fmt.Errorf("task failed after %d attempts: %w", maxAttempts, err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "\n[ERROR] Attempt %d failed: %v\n", attempt, err)
			fmt.Fprintf(os.Stderr, "Asking intelligence system for alternative model...\n")
		}

		recommendations, err := intelligence.RecommendModelForRetry(setup, "editor", currentBackend, currentEditorModel, task)
		if err != nil || len(recommendations) == 0 {
			if !shouldPromptForRetry(interactive) {
				return fmt.Errorf("task failed: no automatic retry model available: %w", err)
			}
			// No automatic recommendations - ask user
			fmt.Fprintf(os.Stderr, "\n  No suitable models found automatically.\n")
			fmt.Fprintf(os.Stderr, "\nAvailable backends:\n")
			var backends []string
			for backend := range setup.Backend {
				backends = append(backends, backend)
				fmt.Fprintf(os.Stderr, "  - %s\n", backend)
			}
			if len(backends) == 0 {
				return fmt.Errorf("no backends configured")
			}
			fmt.Fprintf(os.Stderr, "\nWhich backend would you like to try? (or 'quit' to stop): ")
			var input string
			fmt.Scanln(&input)
			if input == "quit" || input == "q" {
				return fmt.Errorf("user cancelled")
			}
			if _, exists := setup.Backend[input]; !exists {
				return fmt.Errorf("backend '%s' not found", input)
			}
			currentBackend = input
			backendCfg := setup.Backend[currentBackend]
			currentEditorModel = backendCfg.GetModelForAgent("editor")
			if currentEditorModel == "" {
				currentEditorModel = backendCfg.DefaultModel
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "Switching to %s/%s...\n", currentBackend, currentEditorModel)
			}
			continue
		}

		rec := recommendations[0]

		if verbose {
			fmt.Fprintf(os.Stderr, "\n[RECOMMENDATION] Intelligence recommends: %s/%s\n", rec.Backend, rec.Model)
			fmt.Fprintf(os.Stderr, "   Overall Score: %.2f\n", rec.Score)
			fmt.Fprintf(os.Stderr, "   Success Rate: %.0f%% | Speed: %d TPS | Cost: $%.3f/1M | Latency: %dms\n",
				rec.Metrics.SuccessRate*100,
				rec.Metrics.SpeedTPS,
				rec.Metrics.CostPer1M,
				rec.Metrics.AvgLatencyMs)
			fmt.Fprintf(os.Stderr, "   Reason: %s\n", rec.Reason)
			fmt.Fprintf(os.Stderr, "\nRetrying with recommended model...\n")
		}

		if rec.Backend != currentBackend {
			if _, exists := setup.Backend[rec.Backend]; exists {
				currentBackend = rec.Backend
			}
		}

		currentEditorModel = rec.Model
	}

	return fmt.Errorf("task failed after %d attempts", maxAttempts)
}

type doModelSelection struct {
	editorBackend string
	editorModel   string
	editorReason  string
	queryBackend  string
	queryModel    string
	queryReason   string
}

func selectDoModels(setup *config.Setup) (doModelSelection, error) {
	backendName := setup.Defaults.Backend
	backendCfg, ok := setup.Backend[backendName]
	if !ok {
		return doModelSelection{}, fmt.Errorf("backend %q is not configured", backendName)
	}

	profileName := setup.Defaults.Profile
	editorModel := backendCfg.GetModelForAgentWithProfile("editor", profileName)
	queryModel := backendCfg.GetModelForAgentWithProfile("query", profileName)
	if editorModel == "" || queryModel == "" {
		return doModelSelection{}, fmt.Errorf("backend %q has incomplete agent model configuration", backendName)
	}

	reason := "Using configured agent model"
	if profileName != "" {
		reason = fmt.Sprintf("Using profile: %s/%s", backendName, profileName)
	}

	return doModelSelection{
		editorBackend: backendName,
		editorModel:   editorModel,
		editorReason:  reason,
		queryBackend:  backendName,
		queryModel:    queryModel,
		queryReason:   reason,
	}, nil
}

func shouldPromptForRetry(interactive bool) bool {
	return interactive && term.IsTerminal(int(os.Stdin.Fd()))
}

func runDoExecution(ctx context.Context, task string, verbose bool, supervised bool, setup *config.Setup, backendName string, editorModel string) error {
	backendCfg := setup.Backend[backendName]

	cwd, _ := os.Getwd()

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	// Use same backend for all agents (query, research, editor) for consistency
	// This ensures retry switches all models together
	profileName := setup.Defaults.Profile
	queryModel := backendCfg.GetModelForAgentWithProfile("query", profileName)
	if queryModel == "" {
		queryModel = backendCfg.DefaultModel
	}

	researchModel := backendCfg.GetModelForAgentWithProfile("research", profileName)
	if researchModel == "" {
		researchModel = backendCfg.DefaultModel
	}

	orchestrator := llm.NewOrchestrator(backendCfg.BaseURL, backendName, provider, researchModel)

	queryProvider := provider

	if verbose {
		fmt.Fprintf(os.Stderr, "Query: %s/%s\n", backendName, queryModel)
		fmt.Fprintf(os.Stderr, "Research: %s/%s\n\n", backendName, researchModel)
	}

	// ALWAYS use Symphony executor for non-supervised mode
	// Symphony internally decides: complexity < 7 = direct, >= 7 = decompose
	if !supervised {
		if verbose {
			fmt.Fprintf(os.Stderr, "Analyzing task complexity...\n")
		}
		// Detect language
		language := setup.Defaults.Lang
		if language == "" {
			language = "go" // default
		}

		liveClient := live.GetClient()
		var reportConfig *live.ReportConfig
		if liveURL := os.Getenv("GPTCODE_LIVE_URL"); liveURL != "" {
			reportConfig = live.DefaultReportConfig()
			reportConfig.SetBaseURL(liveURL)
			reportConfig.AgentID = live.GetAgentID()
		} else if liveClient != nil {
			reportConfig = live.DefaultReportConfig()
			reportConfig.AgentID = live.GetAgentID()
		}

		executor := modes.NewAutonomousExecutorWithLive(queryProvider, cwd, queryModel, language, liveClient, reportConfig, backendName)
		return executor.Execute(ctx, task)
	}

	// Supervised mode: use guided workflow
	if supervised {
		guided := modes.NewGuidedMode(orchestrator, cwd, queryModel)

		if verbose {
			fmt.Fprintf(os.Stderr, "Creating plan...\n")
		}

		planContent, err := guided.ExecuteAndReturnPlan(ctx, task)
		if err != nil {
			return fmt.Errorf("plan creation failed: %w", err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Plan created. Starting implementation...\n")
		}

		guidedWithCustomEditor := modes.NewGuidedModeWithCustomModel(orchestrator, provider, cwd, queryModel, editorModel)

		if err := guidedWithCustomEditor.Implement(ctx, planContent); err != nil {
			return fmt.Errorf("implementation failed: %w", err)
		}
	} else {
		orchestrated := modes.NewOrchestratedMode(orchestrator, provider, cwd, queryModel, editorModel)

		if verbose {
			fmt.Fprintf(os.Stderr, "Using orchestrated mode with decomposed agents...\n")
		}

		if err := orchestrated.Execute(ctx, task); err != nil {
			return fmt.Errorf("orchestrated execution failed: %w", err)
		}
	}

	return nil
}
