package ci

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/recovery"
)

type CIStatus struct {
	State      string
	Conclusion string
	Name       string
	URL        string
	LogURL     string
}

type CIFailure struct {
	JobName    string
	Step       string
	Error      string
	LogSnippet string
	FullLog    string
}

type Handler struct {
	repo     string
	workDir  string
	provider llm.Provider
	model    string
}

func NewHandler(repo, workDir string, provider llm.Provider, model string) *Handler {
	return &Handler{
		repo:     repo,
		workDir:  workDir,
		provider: provider,
		model:    model,
	}
}

func (h *Handler) CheckPRStatus(prNumber int) ([]CIStatus, error) {
	cmd := exec.Command("gh", "pr", "checks", strconv.Itoa(prNumber),
		"--repo", h.repo)
	cmd.Dir = h.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to check PR status: %w\nOutput: %s", err, string(output))
	}

	var statuses []CIStatus
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := CIStatus{
			State: parts[0],
			Name:  strings.Join(parts[1:], " "),
		}

		if strings.Contains(strings.ToLower(status.State), "fail") ||
			strings.Contains(strings.ToLower(status.State), "error") {
			status.Conclusion = "failure"
		} else if strings.Contains(strings.ToLower(status.State), "success") ||
			strings.Contains(strings.ToLower(status.State), "pass") {
			status.Conclusion = "success"
		} else {
			status.Conclusion = "pending"
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (h *Handler) GetFailedChecks(prNumber int) ([]CIStatus, error) {
	statuses, err := h.CheckPRStatus(prNumber)
	if err != nil {
		return nil, err
	}

	var failed []CIStatus
	for _, status := range statuses {
		if status.Conclusion == "failure" {
			failed = append(failed, status)
		}
	}

	return failed, nil
}

func (h *Handler) FetchCILogs(prNumber int, checkName string) (string, error) {
	cmd := exec.Command("gh", "run", "view",
		"--repo", h.repo,
		"--log")
	cmd.Dir = h.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to fetch CI logs: %w", err)
	}

	return string(output), nil
}

func (h *Handler) AnalyzeFailure(failure CIFailure) (*recovery.FixResult, error) {
	fixer := recovery.NewErrorFixer(h.provider, h.model, h.workDir)

	prompt := fmt.Sprintf(`CI/CD Failure Analysis:

Job: %s
Step: %s
Error: %s

Log snippet:
%s

Please analyze the failure and identify:
1. Root cause
2. Files that need to be modified
3. Specific changes needed

Focus on common CI issues:
- Test failures
- Build errors
- Linting issues
- Dependency problems
- Environment issues`, failure.JobName, failure.Step, failure.Error, failure.LogSnippet)

	fixResult, err := fixer.FixGenericError(context.Background(), prompt, failure.LogSnippet, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze failure: %w", err)
	}

	return fixResult, nil
}

func (h *Handler) ParseCIFailure(log string) *CIFailure {
	failure := &CIFailure{
		FullLog: log,
	}

	lines := strings.Split(log, "\n")

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(strings.ToLower(line), "fail") ||
			strings.Contains(strings.ToLower(line), "fatal") {

			failure.Error = strings.TrimSpace(line)

			start := i - 5
			if start < 0 {
				start = 0
			}
			end := i + 10
			if end > len(lines) {
				end = len(lines)
			}

			snippet := lines[start:end]
			failure.LogSnippet = strings.Join(snippet, "\n")
			break
		}
	}

	for _, line := range lines {
		if strings.Contains(line, "##[group]") {
			failure.Step = strings.TrimSpace(strings.TrimPrefix(line, "##[group]"))
		}
		if strings.Contains(line, "Job:") {
			failure.JobName = strings.TrimSpace(strings.TrimPrefix(line, "Job:"))
		}
	}

	if failure.Error == "" {
		failure.Error = "Unknown CI failure"
		if len(lines) > 50 {
			failure.LogSnippet = strings.Join(lines[len(lines)-50:], "\n")
		} else {
			failure.LogSnippet = log
		}
	}

	return failure
}

func (h *Handler) WaitForCI(prNumber int, maxWaitMinutes int) error {
	fmt.Printf("⏳ Waiting for CI checks (max %d minutes)...\n", maxWaitMinutes)

	for i := 0; i < maxWaitMinutes; i++ {
		statuses, err := h.CheckPRStatus(prNumber)
		if err != nil {
			return err
		}

		allComplete := true
		anyFailed := false

		for _, status := range statuses {
			if status.Conclusion == "pending" {
				allComplete = false
			}
			if status.Conclusion == "failure" {
				anyFailed = true
			}
		}

		if allComplete {
			if anyFailed {
				return fmt.Errorf("CI checks failed")
			}
			fmt.Println("✅ All CI checks passed")
			return nil
		}

		if i < maxWaitMinutes-1 {
			fmt.Print(".")
		}
	}

	return fmt.Errorf("CI checks timed out after %d minutes", maxWaitMinutes)
}
