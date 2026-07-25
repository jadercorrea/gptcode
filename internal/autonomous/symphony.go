package autonomous

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gptcode/internal/maestro"
)

// Symphony represents a multi-movement task execution
type Symphony struct {
	ID              string     `json:"id"`
	Task            string     `json:"task"`
	Movements       []Movement `json:"movements"`
	CurrentMovement int        `json:"current_movement"`
	Status          string     `json:"status"` // "pending", "executing", "completed", "failed"
	StartTime       time.Time  `json:"start_time"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// Executor executes tasks using the Symphony pattern
type Executor struct {
	analyzer *TaskAnalyzer
	maestro  *maestro.Conductor
	cwd      string
}

// NewExecutor creates a new symphony executor
func NewExecutor(
	analyzer *TaskAnalyzer,
	maestro *maestro.Conductor,
	cwd string,
) *Executor {
	return &Executor{
		analyzer: analyzer,
		maestro:  maestro,
		cwd:      cwd,
	}
}

// Execute executes a task autonomously
func (e *Executor) Execute(ctx context.Context, task string) error {
	// 1. Analyze task
	fmt.Println("Analyzing task...")
	analysis, err := e.analyzer.Analyze(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to analyze task: %w", err)
	}

	// Override intent if task is obviously a query (ML sometimes misclassifies)
	if isObviousQuery(task) {
		analysis.Intent = "query"
	}

	fmt.Printf("   Intent: %s\n", analysis.Intent)
	fmt.Printf("   Complexity: %d/10\n", analysis.Complexity)

	// 2. If query task with read-only movements, execute directly
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		readOnly := isReadOnlyMovements(analysis.Movements)
		fmt.Fprintf(os.Stderr, "[SYMPHONY] Intent=%s, #movements=%d, isReadOnly=%v\n",
			analysis.Intent, len(analysis.Movements), readOnly)
		if !readOnly && len(analysis.Movements) > 0 {
			fmt.Fprintf(os.Stderr, "[SYMPHONY] Movements NOT read-only:")
			for i, m := range analysis.Movements {
				fmt.Fprintf(os.Stderr, "\n[SYMPHONY]   %d: %s", i+1, m.Goal)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}
	if analysis.Intent == "query" && isReadOnlyMovements(analysis.Movements) {
		fmt.Println("\nQuery task detected! Executing directly (no decomposition)...")
		return e.executeDirect(ctx, task, analysis)
	}

	// Maestro already performs plan, edit, review, and verification. Keep ordinary
	// fixes in that single coherent workflow; Symphony is reserved for genuinely
	// large tasks with multiple independently useful movements.
	if !shouldUseSymphony(analysis) {
		fmt.Println("\nExecuting directly...")
		return e.executeDirect(ctx, task, analysis)
	}

	// 3. Complex task (ML scored >= 7): decompose into Symphony movements
	if len(analysis.Movements) == 0 {
		fmt.Println("\n[WARNING] Model failed to decompose task (returned empty plan).")
		fmt.Println("Falling back to direct execution...")
		return e.executeDirect(ctx, task, analysis)
	}

	fmt.Printf("\nComplex task detected! Creating symphony with %d movements...\n\n", len(analysis.Movements))

	symphony := &Symphony{
		ID:              generateID(),
		Task:            task,
		Movements:       analysis.Movements,
		CurrentMovement: 0,
		Status:          "executing",
		StartTime:       time.Now(),
	}

	// 4. Optimize movements: collapse redundant display/show movements
	symphony.Movements = collapseDisplayMovements(symphony.Movements)
	fmt.Printf("Optimized to %d movements\n\n", len(symphony.Movements))

	// 5. Execute each movement
	for i, movement := range symphony.Movements {
		symphony.CurrentMovement = i

		movementMsg := fmt.Sprintf("Movement %d/%d: %s", i+1, len(symphony.Movements), movement.Name)
		fmt.Printf("Movement %d/%d: %s\n", i+1, len(symphony.Movements), movement.Name)
		fmt.Printf("   Goal: %s\n", movement.Goal)

		// Report progress to Live Dashboard
		e.maestro.ReportProgress("symphony", movementMsg)

		err := e.executeMovement(ctx, &symphony.Movements[i], symphony.Task)
		if err != nil {
			symphony.Status = "failed"
			e.maestro.ReportError("symphony", fmt.Sprintf("Movement %d failed: %v", i+1, err))
			return fmt.Errorf("movement %d failed: %w", i+1, err)
		}

		fmt.Printf("   [OK] Movement %d complete\n\n", i+1)
		e.maestro.ReportProgress("symphony", fmt.Sprintf("Movement %d complete", i+1))

		// Save checkpoint (enable resume)
		if err := e.saveCheckpoint(symphony); err != nil {
			fmt.Printf("   [WARNING] Failed to save checkpoint: %v\n", err)
		}
	}

	now := time.Now()
	symphony.Status = "completed"
	symphony.CompletedAt = &now

	fmt.Println("[OK] Symphony complete!")
	return nil
}

func shouldUseSymphony(analysis *TaskAnalysis) bool {
	return analysis != nil &&
		analysis.Intent != "query" &&
		analysis.Complexity >= 9 &&
		len(analysis.Movements) > 1
}

// executeDirect executes a simple task without decomposition
func (e *Executor) executeDirect(ctx context.Context, task string, analysis *TaskAnalysis) error {
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[SYMPHONY] executeDirect called: complexity=%d\n", analysis.Complexity)
	}

	// Delegate to Maestro with complexity
	complexityStr := "simple"
	if analysis.Complexity >= 7 {
		complexityStr = "complex"
	} else if analysis.Complexity >= 5 {
		complexityStr = "medium"
	}

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[SYMPHONY] Calling maestro.ExecuteTask with complexityStr=%s\n", complexityStr)
	}

	return e.maestro.ExecuteTask(ctx, task, complexityStr)
}

// executeMovement executes a single movement with retry on review failure
func (e *Executor) executeMovement(ctx context.Context, movement *Movement, originalTask string) error {
	movement.Status = "executing"

	// Prepend the original task context so the editor LLM has full context
	// (stacktrace, tool instructions, etc.) instead of just the movement's short goal
	enrichedGoal := fmt.Sprintf(`## Original Task Context
%s

## Current Movement Goal
%s`, originalTask, movement.Goal)

	// Delegate to Maestro (movements are complex by definition)
	err := e.maestro.ExecuteTask(ctx, enrichedGoal, "complex")
	if err != nil {
		movement.Status = "failed"
		return err
	}

	movement.Status = "completed"
	return nil
}

// saveCheckpoint saves symphony state for resume capability
func (e *Executor) saveCheckpoint(symphony *Symphony) error {
	checkpointsDir := filepath.Join(os.Getenv("HOME"), ".gptcode", "symphonies")
	if err := os.MkdirAll(checkpointsDir, 0755); err != nil {
		return err
	}

	checkpointPath := filepath.Join(checkpointsDir, symphony.ID+".json")

	data, err := json.MarshalIndent(symphony, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(checkpointPath, data, 0644)
}

// generateID generates a random symphony ID
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// isReadOnlyMovements checks if all movements are read-only (query tasks)
func isReadOnlyMovements(movements []Movement) bool {
	for _, m := range movements {
		lower := strings.ToLower(m.Goal)

		editPatterns := []string{
			"modify the", "modify file",
			"write to", "write the", "write file",
			"create a", "create the", "create new", "create file",
			"update the", "update file",
			"delete the", "delete file",
			"change the", "change file",
			"edit the", "edit file",
			"add to", "add the", "add new", "add file",
			"remove from", "remove the", "remove file",
			"refactor",
			"implement",
			"fix the code", "fix bug",
		}

		for _, pattern := range editPatterns {
			if strings.Contains(lower, pattern) {
				return false
			}
		}
	}
	return true
}

// isObviousQuery checks if task is clearly a read-only query
func isObviousQuery(task string) bool {
	lower := strings.ToLower(strings.TrimSpace(task))

	// Check for question words at start
	questionStarters := []string{
		"what", "show", "list", "display", "tell", "which",
		"how many", "where", "when", "who",
	}
	for _, starter := range questionStarters {
		if strings.HasPrefix(lower, starter) {
			return true
		}
	}

	// Check for common query patterns
	queryPatterns := []string{
		"git status", "git log", "git diff", "git branch",
		"what changes", "show me", "list all",
	}
	for _, pattern := range queryPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// collapseDisplayMovements merges redundant display/show movements with their execution
func collapseDisplayMovements(movements []Movement) []Movement {
	if len(movements) <= 1 {
		return movements
	}

	var optimized []Movement
	for i, m := range movements {
		lower := strings.ToLower(m.Goal)
		isDisplay := strings.Contains(lower, "display") || strings.Contains(lower, "show")

		// If this is a display movement and previous was execution, skip it
		if i > 0 && isDisplay {
			prev := &optimized[len(optimized)-1]
			prevLower := strings.ToLower(prev.Goal)
			if strings.Contains(prevLower, "run") || strings.Contains(prevLower, "execute") || strings.Contains(prevLower, "retrieve") {
				// Skip redundant display movement - execution handles it
				continue
			}
		}

		optimized = append(optimized, m)
	}

	return optimized
}
