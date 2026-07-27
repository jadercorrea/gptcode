package maestro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	"gptcode/internal/agents"
	"gptcode/internal/config"
	"gptcode/internal/events"
	"gptcode/internal/live"
	"gptcode/internal/llm"
	"gptcode/internal/observability"
	"gptcode/internal/telemetry"
)

// Maestro orchestrates autonomous execution with verification and recovery
type Maestro struct {
	Provider       llm.Provider
	CWD            string
	Model          string
	Events         *events.Emitter
	Verifiers      []Verifier
	Recovery       *RecoveryStrategy
	Checkpoints    *CheckpointSystem
	MaxRetries     int
	ModifiedFiles  []string
	CurrentStepIdx int
	Tracer         observability.Tracer
	UsageTracker   *telemetry.UsageTracker
}

// NewMaestro creates a new Maestro orchestrator
func NewMaestro(provider llm.Provider, cwd, model string) *Maestro {
	checkpoints := NewCheckpointSystem(cwd)
	recovery := NewRecoveryStrategy(3, checkpoints)
	recovery.Verbose = os.Getenv("GPTCODE_DEBUG") == "1" // Enable verbose logging in debug mode
	tracer := observability.NewTracer()
	usageTracker := telemetry.NewUsageTracker()

	return &Maestro{
		Provider:     provider,
		CWD:          cwd,
		Model:        model,
		Events:       events.NewEmitter(os.Stderr),
		Verifiers:    []Verifier{},
		Recovery:     recovery,
		Checkpoints:  checkpoints,
		MaxRetries:   3,
		Tracer:       tracer,
		UsageTracker: usageTracker,
	}
}

// checkPause delegates to the global ControlManager
func (m *Maestro) checkPause() {
	live.GetControlManager().CheckPause()
}

// ExecutePlan runs the autonomous execution loop
func (m *Maestro) ExecutePlan(ctx context.Context, planContent string) error {
	m.CurrentStepIdx = 0
	m.ModifiedFiles = nil
	_ = m.Events.Status("\u001b[36mStarting autonomous execution...\u001b[0m")

	// Load setup to access budget settings
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if budget is exceeded before starting
	if setup.Defaults.BudgetMode {
		exceeded, remaining := m.UsageTracker.CheckBudget(setup.Defaults.MonthlyBudget)
		if exceeded {
			return fmt.Errorf("monthly budget exceeded: $%.2f spent, $%.2f budget, -$%.2f remaining",
				m.UsageTracker.GetTotalCost(), setup.Defaults.MonthlyBudget, -remaining)
		}
		if remaining < setup.Defaults.MaxCostPerTask {
			_ = m.Events.Notify(fmt.Sprintf("\u001b[33mWarning: Budget running low ($%.2f remaining)\u001b[0m", remaining), "warn")
		}
	}

	// Begin tracing session
	sessionID := uuid.New().String()
	if m.Tracer != nil {
		_ = m.Tracer.Begin(sessionID, planContent)
		defer func() { _ = m.Tracer.End(true) }() // End with success status (will be updated on error)
	}

	// Parse plan into steps (simple version: split by phases)
	steps := m.parsePlan(planContent)

	for stepIdx, step := range steps {
		_ = m.Events.Status(fmt.Sprintf("\u001b[34mStep %d/%d\u001b[0m: %s", stepIdx+1, len(steps), step.Title))

		var lastCheckpoint *Checkpoint
		var err error

		// Track history across attempts for context
		var history []llm.ChatMessage

		// Try execution with retries
		for attempt := 0; attempt < m.MaxRetries; attempt++ {
			if attempt > 0 {
				_ = m.Events.Status(fmt.Sprintf("Retry %d/%d", attempt, m.MaxRetries))
			}

			// Execute the step
			m.CurrentStepIdx = stepIdx
			m.checkPause()

			result, modifiedFiles, err := m.executeStepWithHistory(ctx, step, history)
			m.ModifiedFiles = modifiedFiles // Use the actual modified files returned by the agent
			_ = result                      // Use the result to avoid unused variable error, could be used for additional processing later

			if err != nil {
				_ = m.Events.Notify(fmt.Sprintf("\u001b[31mExecution failed\u001b[0m: %v", err), "error")
				continue
			}

			// Verify the changes
			verifyResult, verifyErr := m.verify(ctx)
			if verifyErr != nil {
				_ = m.Events.Notify(fmt.Sprintf("\u001b[31mVerification error\u001b[0m: %v", verifyErr), "error")
				if m.Tracer != nil {
					_ = m.Tracer.RecordMetrics("Verification", observability.Metrics{ErrorMessage: verifyErr.Error()})
				}
				continue
			}

			if !verifyResult.Success {
				_ = m.Events.Notify(fmt.Sprintf("\u001b[33mVerification failed\u001b[0m: %s", verifyResult.Output), "warn")

				// Classify error and decide recovery strategy
				errorType := ClassifyError(verifyResult.Output)
				_ = m.Events.Status(fmt.Sprintf("Error type: %s, attempting recovery...", errorType))

				// Create recovery context with more information
				recoveryCtx := &RecoveryContext{
					ErrorType:     errorType,
					ErrorOutput:   verifyResult.Output,
					ModifiedFiles: m.ModifiedFiles,
					StepIndex:     stepIdx,
					Attempts:      attempt,
					MaxAttempts:   m.MaxRetries,
				}

				// Try advanced recovery first
				advancedPrompt, found := m.Recovery.AdvancedRecovery(recoveryCtx)
				if !found {
					// Fall back to basic error formatting
					advancedPrompt = m.Recovery.GenerateFixPromptWithContext(recoveryCtx)
				}

				// For build errors, rollback if we have a checkpoint
				if errorType == ErrorBuild && lastCheckpoint != nil {
					_ = m.Events.Status("\u001b[35mRolling back to last checkpoint...\u001b[0m")
					if rollbackErr := m.Recovery.Rollback(lastCheckpoint.ID); rollbackErr != nil {
						_ = m.Events.Notify(fmt.Sprintf("Rollback failed: %v", rollbackErr), "error")
					}
				}

				// Add recovery prompt to history for next attempt
				recoveryMessage := llm.ChatMessage{
					Role:    "user",
					Content: advancedPrompt,
				}
				history = append(history, recoveryMessage)

				verificationErr := fmt.Errorf("verification failed: %s", verifyResult.Error)
				if m.Tracer != nil {
					_ = m.Tracer.RecordMetrics("Verification", observability.Metrics{ErrorMessage: verificationErr.Error()})
				}
				continue
			}

			// Success! Save checkpoint
			_ = m.Events.Status("\u001b[32mVerification passed\u001b[0m, saving checkpoint...")
			_, err = m.Checkpoints.Save(stepIdx, m.ModifiedFiles)
			if err != nil {
				_ = m.Events.Notify(fmt.Sprintf("Checkpoint save failed: %v", err), "warn")
			}

			_ = m.Events.Complete()
			break
		}

		if err != nil {
			// Update tracer with failure status
			if m.Tracer != nil {
				// Override the success status set in defer
				_ = m.Tracer.End(false)
			}
			return fmt.Errorf("step %d failed after %d retries: %w", stepIdx, m.MaxRetries, err)
		}
	}

	_ = m.Events.Message("\u001b[32mAutonomous execution completed successfully!\u001b[0m")
	// On successful completion, update tracer status
	if m.Tracer != nil {
		_ = m.Tracer.End(true)
	}
	return nil
}

// ResumeExecution continues from the last successful checkpoint
func (m *Maestro) ResumeExecution(ctx context.Context, planContent string) error {
	steps := m.parsePlan(planContent)

	// Load latest checkpoint directory and infer step index
	// Simple approach: scan .gptcode/checkpoints and pick the latest, then parse step index from id
	ckptDir := m.Checkpoints.RootDir
	dirs, err := os.ReadDir(ckptDir)
	if err != nil || len(dirs) == 0 {
		return fmt.Errorf("no checkpoints to resume")
	}

	latest := dirs[len(dirs)-1].Name()
	var step int
	_, err = fmt.Sscanf(latest, "ckpt_%d_", &step)
	if err != nil {
		return fmt.Errorf("failed to parse checkpoint step: %w", err)
	}

	m.CurrentStepIdx = step + 1
	if m.CurrentStepIdx >= len(steps) {
		return fmt.Errorf("nothing to resume: all steps completed")
	}

	return m.ExecutePlan(ctx, planContent)
}

// executeStep runs a single step of the plan
func (m *Maestro) ExecuteStep(ctx context.Context, step PlanStep) (string, []string, error) {
	return m.executeStep(ctx, step)
}

func (m *Maestro) executeStepWithHistory(ctx context.Context, step PlanStep, history []llm.ChatMessage) (string, []string, error) {
	editorAgent := agents.NewEditor(m.Provider, m.CWD, m.Model)

	statusCallback := func(status string) {
		_ = m.Events.Status(status)
	}

	// If no history provided, create initial history
	if len(history) == 0 {
		history = []llm.ChatMessage{
			{Role: "user", Content: fmt.Sprintf("Implement this step:\n\n# %s\n\n%s", step.Title, step.Content)},
		}
	}

	start := time.Now()
	result, modifiedFiles, err := editorAgent.Execute(ctx, history, statusCallback)
	elapsed := time.Since(start)

	// Record metrics for the execution step
	if m.Tracer != nil {
		metrics := observability.Metrics{
			DurationMs:   elapsed.Milliseconds(),
			ErrorMessage: "",
		}
		if err != nil {
			metrics.ErrorMessage = err.Error()
		}
		_ = m.Tracer.RecordMetrics("EditorAgent", metrics)
	}

	return result, modifiedFiles, err
}

func (m *Maestro) executeStep(ctx context.Context, step PlanStep) (string, []string, error) {
	editorAgent := agents.NewEditor(m.Provider, m.CWD, m.Model)

	statusCallback := func(status string) {
		_ = m.Events.Status(status)
	}

	history := []llm.ChatMessage{
		{Role: "user", Content: fmt.Sprintf("Implement this step:\n\n# %s\n\n%s", step.Title, step.Content)},
	}

	result, modifiedFiles, err := editorAgent.Execute(ctx, history, statusCallback)
	// Track modified files in the maestro state if needed, but for now we rely on git diff in the outer loop
	// or we could return them here.
	// The outer loop uses gitChangedFiles(), but we should probably use these modifiedFiles too.
	// However, executeStep returns error only.
	// Let's just update the signature call for now.
	return result, modifiedFiles, err
}

// verify runs all verifiers
func (m *Maestro) verify(ctx context.Context) (*VerificationResult, error) {
	// Dynamically select verifiers based on modified files
	verifiers := m.selectVerifiers()

	for _, verifier := range verifiers {
		start := time.Now()
		result, err := verifier.Verify(ctx)
		elapsed := time.Since(start)

		// Record metrics for the verification step
		if m.Tracer != nil {
			metrics := observability.Metrics{
				DurationMs:   elapsed.Milliseconds(),
				ErrorMessage: "",
			}
			if err != nil {
				metrics.ErrorMessage = err.Error()
			}
			_ = m.Tracer.RecordMetrics("Verifier", metrics)
		}

		if err != nil {
			return nil, err
		}
		if !result.Success {
			return result, nil
		}
	}

	return &VerificationResult{Success: true}, nil
}

// selectVerifiers dynamically selects which verifiers to run based on modified files
func (m *Maestro) selectVerifiers() []Verifier {
	// Get current modified files (including added, modified, deleted, renamed, copied)
	gitCmd := exec.Command("git", "--no-pager", "diff", "--name-only", "--diff-filter=ACMR")
	gitCmd.Dir = m.CWD
	gitCmd.Env = cleanGitEnvironment(os.Environ())
	out, err := gitCmd.CombinedOutput()
	if err != nil {
		// If git fails, return default verifiers
		return []Verifier{NewBuildVerifier(m.CWD), NewTestVerifier(m.CWD)}
	}

	modifiedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Check if any modified file is a code file
	hasCodeFiles := false
	codeExtensions := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".java": true, ".c": true,
		".cpp": true, ".rs": true, ".rb": true, ".ex": true,
		".exs": true,
	}

	for _, file := range modifiedFiles {
		if file == "" {
			continue
		}
		for ext := range codeExtensions {
			if strings.HasSuffix(file, ext) {
				hasCodeFiles = true
				break
			}
		}
		if hasCodeFiles {
			break
		}
	}

	// Only add verifiers if code files were modified
	var verifiers []Verifier
	if hasCodeFiles {
		verifiers = append(verifiers, NewBuildVerifier(m.CWD))
		verifiers = append(verifiers, NewTestVerifier(m.CWD))
	}

	// Add lint verifier if it was specifically requested
	for _, originalVerifier := range m.Verifiers {
		// Check if it's a LintVerifier by type assertion
		if _, ok := originalVerifier.(*LintVerifier); ok {
			verifiers = append(verifiers, originalVerifier)
		}
	}

	return verifiers
}

func cleanGitEnvironment(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, variable := range environment {
		if strings.HasPrefix(variable, "GIT_INDEX_FILE=") ||
			strings.HasPrefix(variable, "GIT_DIR=") ||
			strings.HasPrefix(variable, "GIT_WORK_TREE=") {
			continue
		}
		clean = append(clean, variable)
	}
	return clean
}

type PlanStep struct {
	Title    string
	Content  string
	SubSteps []PlanStep
}

func (m *Maestro) ParsePlan(plan string) []PlanStep {
	return m.parsePlan(plan)
}

func (m *Maestro) parsePlan(plan string) []PlanStep {
	var steps []PlanStep
	lines := strings.Split(plan, "\n")

	var currentStep *PlanStep
	var currentSubStep *PlanStep

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if currentStep != nil {
				if currentSubStep != nil {
					currentStep.SubSteps = append(currentStep.SubSteps, *currentSubStep)
					currentSubStep = nil
				}
				steps = append(steps, *currentStep)
			}
			currentStep = &PlanStep{
				Title:    strings.TrimPrefix(line, "## "),
				Content:  "",
				SubSteps: []PlanStep{},
			}
			currentSubStep = nil
		} else if strings.HasPrefix(line, "### ") {
			if currentSubStep != nil && currentStep != nil {
				currentStep.SubSteps = append(currentStep.SubSteps, *currentSubStep)
			}
			currentSubStep = &PlanStep{
				Title:    strings.TrimPrefix(line, "### "),
				Content:  "",
				SubSteps: []PlanStep{},
			}
		} else if currentSubStep != nil {
			currentSubStep.Content += line + "\n"
		} else if currentStep != nil {
			currentStep.Content += line + "\n"
		}
	}

	if currentStep != nil {
		if currentSubStep != nil {
			currentStep.SubSteps = append(currentStep.SubSteps, *currentSubStep)
		}
		steps = append(steps, *currentStep)
	}

	return flattenSteps(steps)
}

func flattenSteps(steps []PlanStep) []PlanStep {
	var flat []PlanStep
	for _, step := range steps {
		if len(step.SubSteps) > 0 {
			for _, sub := range step.SubSteps {
				flat = append(flat, PlanStep{
					Title:   step.Title + " / " + sub.Title,
					Content: step.Content + "\n" + sub.Content,
				})
			}
		} else {
			flat = append(flat, step)
		}
	}
	return flat
}
