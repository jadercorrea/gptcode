package modes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/live"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/prompt"
)

// WatchConfig holds configuration for the watch loop
type WatchConfig struct {
	RoadmapFile string
	Interval    time.Duration
	MaxTasks    int // 0 = unlimited
}

// WatchExecute runs in a continuous loop, picking tasks from a roadmap file
func WatchExecute(builder *prompt.Builder, provider llm.Provider, model string, cfg WatchConfig) error {
	// Find roadmap file
	roadmapPath := cfg.RoadmapFile
	if roadmapPath == "" {
		roadmapPath = findRoadmapFile()
	}
	if roadmapPath == "" {
		return fmt.Errorf("no roadmap file found. Create one of: _roadmap.md, .gptcode/roadmap.md, ROADMAP.md")
	}

	fmt.Printf("Watching roadmap: %s\n", roadmapPath)
	fmt.Printf("Interval between tasks: %s\n", cfg.Interval)
	if cfg.MaxTasks > 0 {
		fmt.Printf("Max tasks: %d\n", cfg.MaxTasks)
	}

	// Connect to Live Dashboard
	dashboardURL := live.GetDashboardURL()
	var liveClient *live.Client
	if dashboardURL != "" {
		agentType := live.AgentTypeBuilder
		agentID := live.GetAgentIDWithType(agentType)
		liveClient = live.NewClient(dashboardURL, agentID)
		liveClient.SetTask("watch: " + filepath.Base(roadmapPath))

		if err := liveClient.Connect(); err != nil {
			fmt.Printf("Live connection error: %v\n", err)
			liveClient = nil
		} else {
			live.SetGlobalClient(liveClient)
			fmt.Printf("Connected to Live Dashboard: %s\n", dashboardURL)
		}
	}

	tasksCompleted := 0

	for {
		// Read roadmap and find next unchecked task
		task, lineNum, err := findNextTask(roadmapPath)
		if err != nil {
			fmt.Printf("Error reading roadmap: %v\n", err)
			break
		}

		if task == "" {
			fmt.Println("All tasks completed. Watching for changes...")

			if liveClient != nil {
				liveClient.SendExecutionStep("complete", "All roadmap tasks completed", nil)
			}

			// Wait and check again
			time.Sleep(cfg.Interval)
			continue
		}

		tasksCompleted++
		fmt.Printf("\n--- Task %d: %s ---\n", tasksCompleted, task)

		// Update Live Dashboard
		if liveClient != nil {
			agentType := live.AgentTypeFromInput(task)
			liveClient.SetAgentType(agentType)
			liveClient.SetTask(task)
			liveClient.SendExecutionStep("start", task, map[string]interface{}{
				"task_number": tasksCompleted,
				"source":      filepath.Base(roadmapPath),
			})
		}

		// Execute the task
		err = RunExecute(builder, provider, model, []string{task}, liveClient, nil)

		if err != nil {
			fmt.Printf("Task failed: %v\n", err)
			if liveClient != nil {
				liveClient.SendExecutionStep("error", fmt.Sprintf("Task failed: %v", err), nil)
			}
		} else {
			// Mark task as done in roadmap
			if markErr := markTaskDone(roadmapPath, lineNum); markErr != nil {
				fmt.Printf("Warning: could not mark task done: %v\n", markErr)
			} else {
				fmt.Printf("Marked task as done: %s\n", task)
			}

			if liveClient != nil {
				liveClient.SendExecutionStep("complete", "Task completed: "+task, nil)
			}
		}

		// Check max tasks
		if cfg.MaxTasks > 0 && tasksCompleted >= cfg.MaxTasks {
			fmt.Printf("\nReached max tasks (%d). Stopping.\n", cfg.MaxTasks)
			break
		}

		// Wait before next task
		fmt.Printf("Waiting %s before next task...\n", cfg.Interval)
		time.Sleep(cfg.Interval)
	}

	// Disconnect
	if liveClient != nil {
		liveClient.Close()
	}

	return nil
}

// findRoadmapFile looks for a roadmap file in standard locations
func findRoadmapFile() string {
	candidates := []string{
		"_roadmap.md",
		".gptcode/roadmap.md",
		"ROADMAP.md",
		"roadmap.md",
		"TODO.md",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

var taskPattern = regexp.MustCompile(`^(\s*)-\s*\[\s*\]\s+(.+)$`)

// findNextTask reads the roadmap and returns the first unchecked task
func findNextTask(path string) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := taskPattern.FindStringSubmatch(line)
		if matches != nil {
			task := strings.TrimSpace(matches[2])
			return task, lineNum, nil
		}
	}

	return "", 0, scanner.Err()
}

// markTaskDone replaces - [ ] with - [x] at the given line number
func markTaskDone(path string, lineNum int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return fmt.Errorf("line %d out of range", lineNum)
	}

	idx := lineNum - 1
	lines[idx] = strings.Replace(lines[idx], "- [ ]", "- [x]", 1)

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
