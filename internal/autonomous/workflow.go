package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/maestro"
)

type AutonomousWorkflow struct {
	executor   *Executor
	git        *GitManager
	testRunner *TestRunner
	maestro    *maestro.Conductor
	cwd        string
}

func NewAutonomousWorkflow(cwd string) *AutonomousWorkflow {
	git := NewGitManager(cwd)
	testRunner := NewTestRunner(cwd, "")

	return &AutonomousWorkflow{
		git:        git,
		testRunner: testRunner,
		cwd:        cwd,
	}
}

func (w *AutonomousWorkflow) SetExecutor(executor *Executor) {
	w.executor = executor
	w.maestro = executor.maestro
}

func (w *AutonomousWorkflow) ExecuteTask(ctx context.Context, task string) error {
	fmt.Printf("\n=== GT Autonomous Workflow ===\n")
	fmt.Printf("Task: %s\n\n", task)

	fmt.Println("[PHASE 1/4] Executing task...")
	if err := w.executor.Execute(ctx, task); err != nil {
		fmt.Printf("[ERROR] Task execution failed: %v\n", err)
		return w.handleFailure(ctx, task, err)
	}
	fmt.Println("[OK] Task execution completed")

	fmt.Println("\n[PHASE 2/4] Running tests...")
	testResult, err := w.testRunner.RunTests(ctx)
	if err != nil {
		fmt.Printf("[WARNING] Test run failed: %v\n", err)
	} else if !testResult.Passed {
		fmt.Printf("[WARNING] Tests failed (%d failures)\n", testResult.Failures)
		if testResult.SnapshotAware && testResult.CanAutoFix {
			fmt.Println("[INFO] Snapshot tests detected - attempting auto-fix...")
			if err := w.handleSnapshotFailure(ctx, task); err != nil {
				fmt.Printf("[ERROR] Snapshot fix failed: %v\n", err)
			}
		}
	} else {
		fmt.Println("[OK] All tests passed")
	}

	fmt.Println("\n[PHASE 3/4] Committing changes...")
	commitResult, err := w.autoCommit(ctx, task)
	if err != nil {
		fmt.Printf("[WARNING] Auto-commit failed: %v\n", err)
	} else if commitResult.Success && commitResult.Message != "No changes to commit" {
		fmt.Printf("[OK] Committed: %s\n", commitResult.Message)
		if commitResult.CommitSHA != "" {
			fmt.Printf("    Commit: %s\n", commitResult.CommitSHA[:min(8, len(commitResult.CommitSHA))])
		}
	} else {
		fmt.Println("[INFO] No changes to commit")
	}

	fmt.Println("\n[PHASE 4/4] Summary")
	fmt.Println("========================")
	w.printSummary()

	return nil
}

func (w *AutonomousWorkflow) handleFailure(ctx context.Context, task string, err error) error {
	fmt.Printf("\n[RECOVERY] Attempting recovery...\n")

	errMsg := err.Error()

	fmt.Println("[RECOVERY] Attempting retry...")
	time.Sleep(1 * time.Second)

	if retryErr := w.executor.Execute(ctx, task); retryErr != nil {
		fmt.Printf("[RECOVERY] Retry failed: %v\n", retryErr)

		fmt.Println("\n[FEEDBACK] What to do:")
		if strings.Contains(errMsg, "syntax") || strings.Contains(errMsg, "parse") {
			fmt.Println("  1. Check for syntax errors")
			fmt.Println("  2. Run: go build ./... or npm run build")
		} else if strings.Contains(errMsg, "test") || strings.Contains(errMsg, "fail") {
			fmt.Println("  1. Run tests manually: npm test or go test ./...")
			fmt.Println("  2. Check test output for failures")
		} else if strings.Contains(errMsg, "permission") {
			fmt.Println("  1. Check file permissions")
			fmt.Println("  2. Ensure working directory is writable")
		}

		return fmt.Errorf("task failed after recovery attempts: %w", err)
	}

	fmt.Println("[RECOVERY] Retry succeeded!")
	return nil
}

func (w *AutonomousWorkflow) handleSnapshotFailure(ctx context.Context, task string) error {
	fmt.Println("[SNAPSHOT] Running tests with snapshot update...")

	result, err := w.testRunner.RunTestsWithUpdate(ctx)
	if err != nil {
		return err
	}

	if result.Passed {
		fmt.Println("[SNAPSHOT] Snapshots updated successfully!")
		return nil
	}

	return fmt.Errorf("snapshot update resulted in failures: %s", result.Output)
}

func (w *AutonomousWorkflow) autoCommit(ctx context.Context, task string) (*CommitResult, error) {
	committer := NewTaskCommitter(w.cwd)
	return committer.AutoGitWorkflow(ctx, task, true)
}

func (w *AutonomousWorkflow) printSummary() {
	if w.git == nil {
		return
	}

	status, err := w.git.GetStatus()
	if err != nil {
		return
	}

	lines := strings.Split(status, "\n")
	for _, line := range lines {
		if line != "" && !strings.Contains(line, "On branch") && !strings.Contains(line, "Your branch") {
			fmt.Println(" ", line)
		}
	}
}

func (w *AutonomousWorkflow) ResumeFromCheckpoint(ctx context.Context) error {
	fmt.Println("[RESUME] Looking for checkpoint...")

	symphony, err := w.loadCheckpoint()
	if err != nil {
		return fmt.Errorf("no checkpoint found: %w", err)
	}

	if symphony.Status == "completed" {
		fmt.Println("[RESUME] Task was already completed")
		return nil
	}

	fmt.Printf("[RESUME] Resuming from movement %d/%d\n",
		symphony.CurrentMovement+1, len(symphony.Movements))

	for i := symphony.CurrentMovement; i < len(symphony.Movements); i++ {
		movement := symphony.Movements[i]
		fmt.Printf("[RESUME] Executing movement %d: %s\n", i+1, movement.Name)

		err := w.executor.executeMovement(ctx, &movement, symphony.Task)
		if err != nil {
			return fmt.Errorf("movement %d failed: %w", i+1, err)
		}

		symphony.CurrentMovement = i + 1
		w.saveCheckpoint(symphony)
	}

	fmt.Println("[RESUME] All movements completed!")
	return nil
}

func (w *AutonomousWorkflow) loadCheckpoint() (*Symphony, error) {
	checkpointsDir := filepath.Join(os.Getenv("HOME"), ".gptcode", "symphonies")

	files, err := os.ReadDir(checkpointsDir)
	if err != nil {
		return nil, err
	}

	var newest string
	var newestTime time.Time

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			path := filepath.Join(checkpointsDir, f.Name())
			info, _ := f.Info()
			if info.ModTime().After(newestTime) {
				newest = path
				newestTime = info.ModTime()
			}
		}
	}

	if newest == "" {
		return nil, fmt.Errorf("no checkpoint found")
	}

	data, err := os.ReadFile(newest)
	if err != nil {
		return nil, err
	}

	var symphony Symphony
	if err := json.Unmarshal(data, &symphony); err != nil {
		return nil, err
	}

	return &symphony, nil
}

func (w *AutonomousWorkflow) saveCheckpoint(symphony *Symphony) error {
	checkpointsDir := filepath.Join(os.Getenv("HOME"), ".gptcode", "symphonies")
	if err := os.MkdirAll(checkpointsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(checkpointsDir, symphony.ID+".json")
	data, err := json.MarshalIndent(symphony, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
