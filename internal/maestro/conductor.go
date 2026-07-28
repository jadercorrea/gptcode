package maestro

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/feedback"
	"github.com/jadercorrea/gptcode/internal/live"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/observability"
)

// ProgressCallback is called during execution to report progress
type ProgressCallback func(phase string, details string)

// Conductor is the central coordinator (Maestro) that orchestrates all agents
type Conductor struct {
	selector         *config.ModelSelector
	setup            *config.Setup
	cwd              string
	language         string
	Recovery         *RecoveryStrategy
	Tracer           observability.Tracer
	Observer         *observability.AgentObserver // For tracking and summary
	loopDetector     *llm.LoopDetector            // Centralized Claude Code-style loop detection
	liveReportConfig *live.ReportConfig           // For Live Dashboard HTTP reporting
	liveClient       *live.Client                 // For Live Dashboard WebSocket reporting
	progressCallback ProgressCallback             // For real-time progress updates
	maxAttempts      int

	// Telemetry
	mu           sync.Mutex
	apiCalls     int
	totalTokens  int
	currentModel string
}

func (c *Conductor) SetMaxAttempts(max int) {
	c.maxAttempts = max
}

// NewConductor creates a new Maestro conductor
func NewConductor(
	selector *config.ModelSelector,
	setup *config.Setup,
	cwd string,
	language string,
) *Conductor {
	// Create a recovery strategy with a temporary checkpoint system
	// The conductor doesn't use checkpoints like the Maestro orchestrator does
	tempCheckpoints := NewCheckpointSystem(cwd)
	recovery := NewRecoveryStrategy(3, tempCheckpoints)
	recovery.Verbose = os.Getenv("GPTCODE_DEBUG") == "1"
	tracer := observability.NewTracer()
	observer := observability.NewObserver()
	observer.SetVerbose(os.Getenv("GPTCODE_DEBUG") == "1")

	return &Conductor{
		selector: selector,
		setup:    setup,
		cwd:      cwd,
		language: language,
		Recovery: recovery,
		Tracer:   tracer,
		Observer: observer,
	}
}

// SetLiveReportConfig sets the Live Dashboard HTTP reporting configuration
func (c *Conductor) SetLiveReportConfig(reportConfig *live.ReportConfig) {
	c.liveReportConfig = reportConfig
}

// SetLiveClient sets the Live Dashboard WebSocket client for real-time updates
func (c *Conductor) SetLiveClient(client *live.Client) {
	c.liveClient = client
}

// SetProgressCallback sets the callback for real-time progress updates
func (c *Conductor) SetProgressCallback(callback ProgressCallback) {
	c.progressCallback = callback
}

// ReportProgress sends progress to Live Dashboard via HTTP and WebSocket, and calls progress callback
func (c *Conductor) ReportProgress(phase string, details string) {
	c.sendProgress(phase, details)
}

// ReportError sends error to Live Dashboard
func (c *Conductor) ReportError(phase string, errMsg string) {
	c.sendError(phase, errMsg)
}

// ReportComplete sends completion to Live Dashboard
func (c *Conductor) ReportComplete(success bool, summary string) {
	c.sendComplete(success, summary)
}

// sendProgress internal helper
func (c *Conductor) sendProgress(phase string, details string) {
	msg := phase + ": " + details

	c.mu.Lock()
	quotaUsed := c.estimateQuota()
	c.mu.Unlock()

	progressPct := 50
	switch phase {
	case "planning":
		progressPct = 15
	case "editing":
		progressPct = 45
	case "validation":
		progressPct = 75
	case "retry":
		progressPct = 60
	}

	// Send via HTTP API (backup/reliability)
	if c.liveReportConfig != nil {
		c.liveReportConfig.Step(msg, "step", map[string]interface{}{
			"progress":   progressPct,
			"quota_used": quotaUsed,
		})
	}

	// Send via WebSocket (real-time)
	if c.liveClient != nil {
		meta := map[string]interface{}{
			"phase":      phase,
			"details":    details,
			"progress":   progressPct,
			"quota_used": quotaUsed,
		}
		if c.Observer != nil {
			sum := c.Observer.Summary()
			meta["tokens_saved"] = sum.TokensSaved
			meta["cost_saved"] = sum.CostSaved
		}
		c.liveClient.SendExecutionStep(phase, details, meta)
	}

	// Call progress callback
	if c.progressCallback != nil {
		c.progressCallback(phase, details)
	}
}

// sendError internal helper
func (c *Conductor) sendError(phase string, errMsg string) {
	c.mu.Lock()
	quotaUsed := c.estimateQuota()
	c.mu.Unlock()

	// Send via HTTP API
	if c.liveReportConfig != nil {
		c.liveReportConfig.Step(phase+": ERROR - "+errMsg, "error", map[string]interface{}{
			"progress":   100,
			"quota_used": quotaUsed,
		})
	}

	// Send via WebSocket
	if c.liveClient != nil {
		meta := map[string]interface{}{
			"phase":      phase,
			"error":      errMsg,
			"quota_used": quotaUsed,
		}
		if c.Observer != nil {
			sum := c.Observer.Summary()
			meta["tokens_saved"] = sum.TokensSaved
			meta["cost_saved"] = sum.CostSaved
		}
		c.liveClient.SendExecutionStep("error", errMsg, meta)
	}
}

// sendComplete internal helper
func (c *Conductor) sendComplete(success bool, summary string) {
	stepType := "complete"
	if !success {
		stepType = "failed"
	}

	c.mu.Lock()
	quotaUsed := c.estimateQuota()
	c.mu.Unlock()

	// Send via HTTP API
	if c.liveReportConfig != nil {
		if success {
			c.liveReportConfig.Step("Completed: "+summary, "complete")
		} else {
			c.liveReportConfig.Step("Failed: "+summary, "error")
		}
		c.liveReportConfig.Disconnect()
	}

	// Send via WebSocket
	if c.liveClient != nil {
		meta := map[string]interface{}{
			"success":    success,
			"summary":    summary,
			"progress":   100,
			"quota_used": quotaUsed,
		}
		if c.Observer != nil {
			sum := c.Observer.Summary()
			meta["tokens_saved"] = sum.TokensSaved
			meta["cost_saved"] = sum.CostSaved
		}
		c.liveClient.SendExecutionStep(stepType, summary, meta)
	}
}

// ReportCompleteWithPR sends completion with a PR URL to the Live Dashboard
func (c *Conductor) ReportCompleteWithPR(success bool, summary string, prURL string) {
	c.sendComplete(success, summary)
	if prURL != "" && c.liveClient != nil {
		c.liveClient.SendExecutionStep("pr_created", summary, map[string]interface{}{
			"pr_url":   prURL,
			"progress": 100,
		})
	}
	if prURL != "" && c.liveReportConfig != nil {
		c.liveReportConfig.Step("PR Created: "+prURL, "complete")
	}
}

// ExecuteTask orchestrates the execution of a task
func (c *Conductor) ExecuteTask(ctx context.Context, task string, complexity string) (taskErr error) {
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MAESTRO] ExecuteTask called: task=%s complexity=%s lang=%s\n", task, complexity, c.language)
	}

	var publicAPIBaseline map[string]string
	if requiresStablePublicAPI(task) && strings.EqualFold(c.language, "go") {
		var snapshotErr error
		publicAPIBaseline, snapshotErr = snapshotGoPublicAPI(c.cwd)
		if snapshotErr != nil {
			return fmt.Errorf("snapshot public API: %w", snapshotErr)
		}
	}

	// Begin tracing session
	sessionID := uuid.New().String()
	if c.Tracer != nil {
		_ = c.Tracer.Begin(sessionID, task)
		defer func() { _ = c.Tracer.End(taskErr == nil) }()
	}

	// Select model for planning
	planBackend, planModel, err := c.selector.SelectModel(config.ActionPlan, c.language, complexity)
	if err != nil {
		return fmt.Errorf("failed to select planner model: %w", err)
	}

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MAESTRO] Planner: %s/%s\n", planBackend, planModel)
	}

	// Record model selection decision
	if c.Tracer != nil {
		decision := observability.Decision{
			Type:         "model_selection",
			Chosen:       fmt.Sprintf("%s/%s", planBackend, planModel),
			Alternatives: []string{},                          // Would populate with alternatives in real implementation
			Attribution:  map[string]float64{"language": 1.0}, // Simplified attribution
			Reasoning:    "Selected based on language and complexity",
		}
		_ = c.Tracer.RecordDecision("ModelSelector", decision)
	}

	// Create planner with selected model
	planProvider := c.createProvider(planBackend)
	planner := agents.NewPlanner(planProvider, planModel)
	repositoryContext, err := buildPlanningRepositoryContext(c.cwd, c.language, 32*1024)
	if err != nil {
		return fmt.Errorf("build planning repository context: %w", err)
	}

	fmt.Println("Creating plan...")
	c.ReportProgress("planning", "Creating plan")
	start := time.Now()
	plan, err := planner.CreatePlan(ctx, task, repositoryContext, nil)
	elapsed := time.Since(start)
	c.ReportProgress("planning", "Plan created")
	c.selector.RecordUsage(planBackend, planModel, err == nil, errorMsg(err))
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	// Record planning metrics
	if c.Tracer != nil {
		metrics := observability.Metrics{
			DurationMs:   elapsed.Milliseconds(),
			ErrorMessage: "",
		}
		_ = c.Tracer.RecordMetrics("PlannerAgent", metrics)
	}

	// Build conversation history
	history := []llm.ChatMessage{
		{
			Role: "user",
			Content: fmt.Sprintf(`## Original task
%s

## Implementation plan
%s

## Current repository evidence
%s

Follow the original task when the plan conflicts with repository evidence.
Do not add, remove, or rename exported symbols when API stability is required.`, task, plan, repositoryContext),
		},
	}

	// Initialize centralized Claude Code-style loop detector
	// Intent is derived from complexity: complex tasks are "edit", simple are "query"
	intent := "edit"
	if c.isQueryTask(task, plan, nil) {
		intent = "query"
	}
	c.loopDetector = llm.NewLoopDetector(intent)
	c.loopDetector.SetMaxIterations(c.maxAttempts)

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MAESTRO] LoopDetector initialized with intent=%s\n", intent)
	}

	consecutiveErrors := 0
	verificationProgress := newVerificationProgress(3)

	for {
		// Check if we should continue (intent-aware limits + loop detection)
		shouldContinue, stopReason := c.loopDetector.ShouldContinue()
		if !shouldContinue {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MAESTRO] Stopping: %s\n", stopReason)
			}
			return fmt.Errorf("task stopped: %s (stats: %s)", stopReason, c.loopDetector.GetStats())
		}

		attempt := c.loopDetector.Iteration
		if attempt > 1 {
			fmt.Printf("Retrying (attempt %d)...\n", attempt)
			c.ReportProgress("retry", fmt.Sprintf("Attempt %d", attempt))
		} else {
			c.ReportProgress("editing", "Starting code changes")
		}

		// Select model for editing
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MAESTRO] About to select editor model for lang=%s complexity=%s\n", c.language, complexity)
		}
		editBackend, editModel, err := c.selector.SelectModel(config.ActionEdit, c.language, complexity)
		if err != nil {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MAESTRO] SelectModel failed: %v\n", err)
			}
			return fmt.Errorf("failed to select editor model: %w", err)
		}

		if os.Getenv("GPTCODE_DEBUG") == "1" && attempt == 1 {
			fmt.Fprintf(os.Stderr, "[MAESTRO] Editor: %s/%s\n", editBackend, editModel)
		}

		// Create editor with selected model and observer
		editProvider := c.createProvider(editBackend)
		editor := agents.NewEditorWithObserver(editProvider, c.cwd, editModel, c.Observer)
		editor.SetExpectedFiles(plannedFiles(plan))

		// Execute with editor
		fmt.Println("Executing changes...")
		c.ReportProgress("editing", "Executing code changes")
		start = time.Now()
		result, modifiedFiles, err := editor.Execute(ctx, history, nil)
		elapsed = time.Since(start)
		c.ReportProgress("editing", fmt.Sprintf("Completed - %d files changed", len(modifiedFiles)))
		c.selector.RecordUsage(editBackend, editModel, err == nil, errorMsg(err))
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= 5 {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[MAESTRO] Aborting after %d consecutive execution errors\n", consecutiveErrors)
				}
				c.ReportError("execution", fmt.Sprintf("Aborted: %d consecutive execution errors. Last error: %v", consecutiveErrors, err))
				return fmt.Errorf("task aborted: %d consecutive execution errors: %w", consecutiveErrors, err)
			}

			// LoopDetector will handle max iterations check on next iteration
			fmt.Printf("[WARNING] Execution error (%d/5): %v\n", consecutiveErrors, err)

			// Use enhanced recovery system
			recoveryCtx := &RecoveryContext{
				ErrorType:     ErrorUnknown, // Will be classified by formatExecutionError
				ErrorOutput:   err.Error(),
				ModifiedFiles: modifiedFiles,
				StepIndex:     -1, // Not applicable in conductor
				Attempts:      attempt,
				MaxAttempts:   c.loopDetector.Iteration,
			}

			// Try advanced recovery first
			advancedPrompt, found := c.Recovery.AdvancedRecovery(recoveryCtx)
			if !found {
				// Fall back to basic error formatting
				advancedPrompt = c.formatExecutionError(err)
			}

			// Record recovery decision
			if c.Tracer != nil {
				decision := observability.Decision{
					Type:         "recovery_strategy",
					Chosen:       "retry_with_error_fix",
					Alternatives: []string{"skip", "abort"},
					Attribution:  map[string]float64{"attempt": float64(attempt), "error_type": 1.0},
					Reasoning:    fmt.Sprintf("Retrying attempt %d after error", attempt),
				}
				_ = c.Tracer.RecordDecision("RecoverySystem", decision)
			}

			retryMessage, retryErr := buildRetryMessage(c.cwd, c.language, advancedPrompt)
			if retryErr != nil {
				return fmt.Errorf("build execution retry context: %w", retryErr)
			}
			history = append(history, llm.ChatMessage{
				Role:    "user",
				Content: retryMessage,
			})
			continue
		}

		consecutiveErrors = 0

		// Record execution metrics
		if c.Tracer != nil {
			metrics := observability.Metrics{
				DurationMs:   elapsed.Milliseconds(),
				ErrorMessage: "",
			}
			_ = c.Tracer.RecordMetrics("EditorAgent", metrics)
		}

		if strings.EqualFold(c.language, "go") && len(modifiedFiles) > 0 {
			formatOutput, formatErr := formatModifiedGoFiles(ctx, c.cwd, modifiedFiles)
			if formatErr != nil {
				feedback := fmt.Sprintf(
					"Deterministic Go formatting failed. Fix the current implementation.\nOutput:\n%s\nError: %v",
					formatOutput,
					formatErr,
				)
				retryMessage, retryErr := buildRetryMessage(c.cwd, c.language, feedback)
				if retryErr != nil {
					return fmt.Errorf("build formatter retry context: %w", retryErr)
				}
				history = append(history, llm.ChatMessage{
					Role:    "user",
					Content: retryMessage,
				})
				continue
			}
		}

		// Check if this is a query-only task (no validation needed)
		if c.isQueryTask(task, plan, modifiedFiles) {
			c.recordFeedback(editBackend, editModel, "editor", task, true, "")

			fmt.Printf("\n[OK] Task complete!\n")
			if result != "" {
				fmt.Printf("   %s\n", result)
			}

			// Print detailed execution summary
			if c.Observer != nil {
				c.Observer.SetOutcome(true)
				c.Observer.PrintSummary()
			}

			return nil
		}

		if publicAPIBaseline != nil {
			currentPublicAPI, snapshotErr := snapshotGoPublicAPI(c.cwd)
			if snapshotErr != nil {
				return fmt.Errorf("validate public API: %w", snapshotErr)
			}
			if changes := publicAPIChanges(publicAPIBaseline, currentPublicAPI); len(changes) > 0 {
				issues := strings.Join(changes, "\n")
				fmt.Printf("[WARNING] Public API contract changed:\n%s\n", issues)
				retryMessage, retryErr := buildRetryMessage(
					c.cwd,
					c.language,
					"Deterministic public API validation failed. Restore the original exported API while preserving the requested internal fix:\n"+issues,
				)
				if retryErr != nil {
					return fmt.Errorf("build public API retry context: %w", retryErr)
				}
				history = append(history, llm.ChatMessage{
					Role:    "user",
					Content: retryMessage,
				})
				continue
			}
		}

		verificationCommand := requestedVerificationCommand(task)
		if len(verificationCommand) == 0 {
			verificationCommand = repositoryVerificationCommand(c.cwd)
		}
		if len(verificationCommand) == 0 {
			return fmt.Errorf("cannot declare task complete: no deterministic repository verification command was found")
		}

		fmt.Printf("Verifying: %s\n", strings.Join(verificationCommand, " "))
		verificationOutput, verificationErr := runRequestedVerification(ctx, c.cwd, verificationCommand)
		if verificationErr != nil {
			fmt.Printf("[WARNING] Deterministic verification failed:\n%s\n", verificationOutput)
			if verificationProgress.Observe(verificationOutput) {
				c.recordFeedback(
					editBackend,
					editModel,
					"editor",
					task,
					false,
					"verification plateau: "+verificationOutput,
				)
				return fmt.Errorf(
					"verification plateau after %d equivalent failures; stopping without spending the remaining attempt budget",
					verificationProgress.Consecutive(),
				)
			}
			retryMessage, retryErr := buildRetryMessage(
				c.cwd,
				c.language,
				fmt.Sprintf("Deterministic verification failed. Fix the implementation and rerun the required check.\nCommand: %s\nOutput:\n%s",
					strings.Join(verificationCommand, " "), verificationOutput),
			)
			if retryErr != nil {
				return fmt.Errorf("build verification retry context: %w", retryErr)
			}
			history = append(history, llm.ChatMessage{
				Role:    "user",
				Content: retryMessage,
			})
			continue
		}
		fmt.Printf("[OK] Deterministic verification passed\n")

		// Select model for review
		reviewBackend, reviewModel, err := c.selector.SelectModel(config.ActionReview, c.language, complexity)
		if err != nil {
			return fmt.Errorf("failed to select reviewer model: %w", err)
		}

		if os.Getenv("GPTCODE_DEBUG") == "1" && attempt == 1 {
			fmt.Fprintf(os.Stderr, "[MAESTRO] Reviewer: %s/%s\n", reviewBackend, reviewModel)
		}

		// Create reviewer with selected model
		reviewProvider := c.createProvider(reviewBackend)
		reviewer := agents.NewReviewer(reviewProvider, c.cwd, reviewModel)

		// Allow callers to skip the model review, but never the deterministic gate above.
		if os.Getenv("SKIP_VALIDATION") == "1" {
			fmt.Println("Skipping model review (SKIP_VALIDATION=1)...")
			c.ReportProgress("validation", "Deterministic verification passed; model review skipped")
			c.recordFeedback(editBackend, editModel, "editor", task, true, "")
			fmt.Printf("\n[OK] Task complete!\n")
			if result != "" {
				fmt.Printf("   %s\n", result)
			}
			return nil
		}

		// Validate
		fmt.Println("Validating...")
		c.ReportProgress("validation", "Running tests and checks")
		start = time.Now()
		reviewPlan := fmt.Sprintf(`%s

DETERMINISTIC VERIFICATION EVIDENCE (authoritative):
Command: %s
Exit status: 0
Output:
%s`, plan, strings.Join(verificationCommand, " "), verificationOutput)
		review, err := reviewer.Review(ctx, reviewPlan, modifiedFiles, nil)
		elapsed = time.Since(start)
		c.ReportProgress("validation", "Validation complete")
		c.selector.RecordUsage(reviewBackend, reviewModel, err == nil, errorMsg(err))
		if err != nil {
			// LoopDetector will handle max iterations check on next iteration
			fmt.Printf("[WARNING] Validation error: %v\n", err)

			// Use enhanced recovery system
			recoveryCtx := &RecoveryContext{
				ErrorType:     ErrorUnknown, // Will be classified by formatValidationError
				ErrorOutput:   err.Error(),
				ModifiedFiles: modifiedFiles,
				StepIndex:     -1, // Not applicable in conductor
				Attempts:      attempt,
				MaxAttempts:   c.loopDetector.Iteration,
			}

			// Try advanced recovery first
			advancedPrompt, found := c.Recovery.AdvancedRecovery(recoveryCtx)
			if !found {
				// Fall back to basic error formatting
				advancedPrompt = c.formatValidationError(err)
			}

			// Record recovery decision
			if c.Tracer != nil {
				decision := observability.Decision{
					Type:         "recovery_strategy",
					Chosen:       "retry_with_validation_fix",
					Alternatives: []string{"skip", "abort"},
					Attribution:  map[string]float64{"attempt": float64(attempt), "error_type": 1.0},
					Reasoning:    fmt.Sprintf("Retrying attempt %d after validation error", attempt),
				}
				_ = c.Tracer.RecordDecision("RecoverySystem", decision)
			}

			retryMessage, retryErr := buildRetryMessage(c.cwd, c.language, advancedPrompt)
			if retryErr != nil {
				return fmt.Errorf("build validation retry context: %w", retryErr)
			}
			history = append(history, llm.ChatMessage{
				Role:    "user",
				Content: retryMessage,
			})
			continue
		}

		if !review.Success {
			// LoopDetector will handle max iterations check on next iteration
			issuesStr := strings.Join(review.Issues, "\n")
			fmt.Printf("[WARNING] Validation failed:\n%s\n", issuesStr)

			// Use enhanced recovery system
			recoveryCtx := &RecoveryContext{
				ErrorType:     ErrorUnknown, // Will be classified by formatValidationIssues
				ErrorOutput:   issuesStr,
				ModifiedFiles: modifiedFiles,
				StepIndex:     -1, // Not applicable in conductor
				Attempts:      attempt,
				MaxAttempts:   c.loopDetector.Iteration,
			}

			// Try advanced recovery first
			advancedPrompt, found := c.Recovery.AdvancedRecovery(recoveryCtx)
			if !found {
				// Fall back to basic error formatting
				advancedPrompt = c.formatValidationIssues(review.Issues)
			}

			// Record recovery decision
			if c.Tracer != nil {
				decision := observability.Decision{
					Type:         "recovery_strategy",
					Chosen:       "retry_with_issues_fix",
					Alternatives: []string{"skip", "abort"},
					Attribution:  map[string]float64{"attempt": float64(attempt), "error_count": float64(len(review.Issues))},
					Reasoning:    fmt.Sprintf("Retrying attempt %d after %d validation issues", attempt, len(review.Issues)),
				}
				_ = c.Tracer.RecordDecision("RecoverySystem", decision)
			}

			retryMessage, retryErr := buildRetryMessage(c.cwd, c.language, advancedPrompt)
			if retryErr != nil {
				return fmt.Errorf("build review retry context: %w", retryErr)
			}
			history = append(history, llm.ChatMessage{
				Role:    "user",
				Content: retryMessage,
			})
			continue
		}

		// Record validation success metrics
		if c.Tracer != nil {
			metrics := observability.Metrics{
				DurationMs:   elapsed.Milliseconds(),
				ErrorMessage: "",
			}
			_ = c.Tracer.RecordMetrics("ReviewerAgent", metrics)
		}

		// Success! Record positive feedback
		c.recordFeedback(editBackend, editModel, "editor", task, true, "")
		c.recordFeedback(reviewBackend, reviewModel, "reviewer", task, true, "")

		fmt.Printf("\n[OK] Task complete!\n")
		if result != "" {
			fmt.Printf("   %s\n", result)
		}

		// Print detailed execution summary
		if c.Observer != nil {
			c.Observer.SetOutcome(true)
			c.Observer.PrintSummary()
		}

		return nil
	}

}

func errorMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (c *Conductor) recordFeedback(backend, model, agent, task string, success bool, failureReason string) {
	sentiment := feedback.SentimentBad
	if success {
		sentiment = feedback.SentimentGood
	}

	event := feedback.Event{
		Sentiment: sentiment,
		Backend:   backend,
		Model:     model,
		Agent:     agent,
		Task:      task,
		Context:   fmt.Sprintf("language=%s", c.language),
	}

	// Add failure reason to metadata so we can learn from specific failure types
	if !success && failureReason != "" {
		event.Metadata = map[string]string{
			"failure_reason": failureReason,
		}
	}

	if err := feedback.Record(event); err != nil {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to record feedback: %v\n", err)
		}
	}
}

// createProvider creates an LLM provider for the given backend
func (c *Conductor) createProvider(backendName string) llm.Provider {
	backendCfg, ok := c.setup.Backend[backendName]
	if !ok {
		// Fallback to default
		backendName = c.setup.Defaults.Backend
		backendCfg = c.setup.Backend[backendName]
	}

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	return &trackingProvider{inner: provider, conductor: c}
}

type trackingProvider struct {
	inner     llm.Provider
	conductor *Conductor
}

func (t *trackingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	t.conductor.mu.Lock()
	t.conductor.apiCalls++
	t.conductor.currentModel = req.Model
	t.conductor.mu.Unlock()

	resp, err := t.inner.Chat(ctx, req)

	if resp != nil && resp.TokenUsage != nil {
		t.conductor.mu.Lock()
		t.conductor.totalTokens += resp.TokenUsage.TotalTokens
		t.conductor.mu.Unlock()
	}
	return resp, err
}

func (c *Conductor) estimateQuota() float64 {
	knownRPD := map[string]int{
		"claude-opus-4-6-thinking": 800,
		"claude-sonnet-4":          2000,
		"gemini-2.5-pro":           25,
		"gemini-2.5-flash":         500,
		"gemini-2.0-flash":         1500,
		"gpt-4o":                   10000,
	}

	rpd, ok := knownRPD[c.currentModel]
	if !ok {
		for key, val := range knownRPD {
			if strings.HasPrefix(c.currentModel, key) {
				rpd = val
				ok = true
				break
			}
		}
	}
	if !ok || rpd == 0 {
		return 0
	}

	quota := float64(c.apiCalls) / float64(rpd)
	if quota > 1.0 {
		quota = 1.0
	}
	return quota
}

// formatExecutionError creates clear feedback for execution errors
func (c *Conductor) formatExecutionError(err error) string {
	return fmt.Sprintf(`EXECUTION FAILED

Error: %v

INSTRUCTIONS:
1. Read the error message carefully
2. Identify what went wrong
3. Fix the specific issue mentioned
4. Try again

Be precise and fix only what's broken.`, err)
}

// formatValidationError creates clear feedback for review errors
func (c *Conductor) formatValidationError(err error) string {
	return fmt.Sprintf(`VALIDATION SYSTEM ERROR

Error: %v

INSTRUCTIONS:
The review process itself failed. This might mean:
- A required tool is not available
- Syntax errors prevent running build/test commands
- File read errors

Please ensure your code is syntactically correct and try again.`, err)
}

// formatValidationIssues creates clear feedback for review failures
func (c *Conductor) formatValidationIssues(issues []string) string {
	var sb strings.Builder
	sb.WriteString("VALIDATION FAILED\n\n")
	sb.WriteString("The following issues were found:\n")
	for i, issue := range issues {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, issue)
	}
	sb.WriteString("\nINSTRUCTIONS:\n")
	sb.WriteString("1. Fix each issue listed above\n")
	sb.WriteString("2. Pay special attention to:\n")

	// Check if there's a Go package mismatch error
	hasPackageError := false
	hasSnapshotFailure := false
	for _, issue := range issues {
		lowerIssue := strings.ToLower(issue)
		if strings.Contains(issue, "found packages") {
			hasPackageError = true
		}
		if strings.Contains(lowerIssue, "snapshot") {
			hasSnapshotFailure = true
		}
	}

	if hasPackageError {
		sb.WriteString("   - **CRITICAL**: Package name mismatch! Read ALL .go files in the directory to see the correct package name, then fix ONLY the wrong file(s)\n")
	}

	if hasSnapshotFailure {
		sb.WriteString("   - **SNAPSHOT FAILURES**: The new output may be CORRECT - review the diff\n")
		sb.WriteString("   - If new output is correct, run test with update flag (e.g., 'npm test -- -u', 'go test -update')\n")
		sb.WriteString("   - If new output is WRONG, fix the implementation\n")
	}

	sb.WriteString("   - Correct package names (all .go files in same directory must have same package)\n")
	sb.WriteString("   - Missing imports\n")
	sb.WriteString("   - Compilation errors\n")
	sb.WriteString("3. Do NOT change what's already correct\n")
	sb.WriteString("4. Only fix the specific problems mentioned\n")
	return sb.String()
}

// isQueryTask checks if task is query-only (no validation needed)
func (c *Conductor) isQueryTask(task, plan string, modifiedFiles []string) bool {
	if len(modifiedFiles) > 0 {
		return false
	}

	taskLower := strings.ToLower(strings.TrimSpace(task))
	editIndicators := []string{
		"fix ", "implement ", "change ", "modify ", "update ", "add ",
		"remove ", "delete ", "refactor ", "create ",
		"corrija ", "corrigir ", "implemente ", "implementar ", "mude ",
		"modifique ", "atualize ", "adicione ", "remova ", "delete ",
		"refatore ", "crie ",
	}
	for _, indicator := range editIndicators {
		if strings.Contains(taskLower, indicator) {
			return false
		}
	}

	lower := strings.ToLower(plan)
	queryIndicators := []string{
		"run command",
		"execute command",
		"git status",
		"git log",
		"gh pr list",
		"read file",
		"show",
		"display",
		"explain",
		"what is",
		"what does",
		"what means",
		"tell me about",
		"describe",
		"files to modify\nnone",
		"files to modify: none",
		"files to create\nnone",
		"files to create: none",
		"no files to modify",
		"no files to create",
	}

	for _, indicator := range queryIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	return false
}

func plannedFiles(plan string) []string {
	var files []string
	inFilesSection := false

	for _, rawLine := range strings.Split(plan, "\n") {
		line := strings.TrimSpace(rawLine)
		lower := strings.ToLower(line)

		if strings.HasPrefix(lower, "## files to modify") ||
			strings.HasPrefix(lower, "## files to create") {
			inFilesSection = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inFilesSection = false
			continue
		}
		if !inFilesSection || !strings.HasPrefix(line, "- ") {
			continue
		}

		item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if strings.EqualFold(item, "none") {
			continue
		}
		if start := strings.Index(item, "`"); start >= 0 {
			if end := strings.Index(item[start+1:], "`"); end >= 0 {
				item = item[start+1 : start+1+end]
			}
		} else if description := strings.Index(item, " ("); description >= 0 {
			item = item[:description]
		}

		item = strings.TrimSpace(item)
		if item != "" {
			files = append(files, item)
		}
	}

	return files
}
