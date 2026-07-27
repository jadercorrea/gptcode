package autonomous

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type TestFailure struct {
	Type       string // "snapshot", "assertion", "compilation", "runtime"
	Summary    string // Brief summary
	Details    string // Full error output
	IsExpected bool   // True if new behavior is correct
	FixNeeded  string // What needs to be done
}

type TestAnalyzer struct {
	llm   llm.Provider
	model string
	cwd   string
}

func NewTestAnalyzer(llm llm.Provider, cwd string, model string) *TestAnalyzer {
	return &TestAnalyzer{
		llm:   llm,
		model: model,
		cwd:   cwd,
	}
}

func (ta *TestAnalyzer) AnalyzeSnapshotFailure(failureOutput string) (*TestFailure, error) {
	snapshotMatch := regexp.MustCompile(`(\d+)\s+snapshots?\s+(?:failed|updated)`)
	matches := snapshotMatch.FindStringSubmatch(failureOutput)

	issue := &TestFailure{
		Type:    "snapshot",
		Details: failureOutput,
	}

	if len(matches) >= 2 {
		count := matches[1]
		issue.Summary = fmt.Sprintf("%s snapshot(s) need updating", count)
	}

	hasDiff := strings.Contains(failureOutput, "+ Received") || strings.Contains(failureOutput, "- Snapshot")
	if hasDiff {
		issue.FixNeeded = "Review the diff and update snapshots if new behavior is correct"
	}

	if strings.Contains(failureOutput, "Snapshot Summary") &&
		(strings.Contains(failureOutput, "updated") || strings.Contains(failureOutput, "updated from")) {
		issue.IsExpected = true
	}

	return issue, nil
}

func (ta *TestAnalyzer) AnalyzeTestOutput(ctx context.Context, testOutput string, context string) (*TestFailure, error) {
	if strings.Contains(testOutput, "snapshots failed") || strings.Contains(testOutput, "snapshots updated") {
		return ta.AnalyzeSnapshotFailure(testOutput)
	}

	if strings.Contains(testOutput, "error:") || strings.Contains(testOutput, "Error:") {
		if strings.Contains(testOutput, "compilation") || strings.Contains(testOutput, "cannot find") {
			return &TestFailure{
				Type:       "compilation",
				Summary:    "Compilation error",
				Details:    testOutput,
				IsExpected: false,
				FixNeeded:  "Fix compilation errors before proceeding",
			}, nil
		}
	}

	if strings.Contains(testOutput, "FAIL") || strings.Contains(testOutput, "failed") {
		return &TestFailure{
			Type:       "assertion",
			Summary:    "Test failure",
			Details:    testOutput,
			IsExpected: false,
			FixNeeded:  "Fix failing tests",
		}, nil
	}

	return &TestFailure{
		Type:       "unknown",
		Summary:    "Unknown test issue",
		Details:    testOutput,
		IsExpected: false,
		FixNeeded:  "Investigate test output",
	}, nil
}

func (ta *TestAnalyzer) ShouldAutoUpdateSnapshots(ctx context.Context, testOutput string, codeContext string) (bool, string, error) {
	prompt := fmt.Sprintf(`Analyze this test output and determine if snapshots should be auto-updated.

TEST OUTPUT:
%s

CODE CONTEXT (what was changed):
%s

Rules:
1. If the new output is CORRECT (matches intended behavior), answer YES
2. If the new output is WRONG (bug introduced), answer NO
3. If uncertain, answer NO and explain why

Examples of CORRECT changes (should update snapshots):
- Better formatting that improves readability
- Consistent style changes
- Removal of unnecessary whitespace/line breaks
- Better error messages

Examples of WRONG changes (should NOT update):
- Breaking existing functionality
- Removing important information
- Incorrect calculations or logic

Return format:
YES - [brief reason]
or
NO - [brief reason]`, testOutput, codeContext)

	resp, err := ta.llm.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a code reviewer specializing in test analysis.",
		UserPrompt:   prompt,
		Model:        ta.model,
	})

	if err != nil {
		return false, "", err
	}

	text := strings.TrimSpace(resp.Text)
	if strings.HasPrefix(strings.ToUpper(text), "YES") {
		reason := text
		if idx := strings.Index(text, "-"); idx > 0 {
			reason = strings.TrimSpace(text[idx+1:])
		}
		return true, reason, nil
	}

	reason := text
	if idx := strings.Index(text, "-"); idx > 0 {
		reason = strings.TrimSpace(text[idx+1:])
	}
	return false, reason, nil
}

func DetectTestCommand(projectType string) string {
	commands := map[string][]string{
		"go":         {"go test ./...", "go test -v ./..."},
		"node":       {"npm test", "npm run test", "npx jest"},
		"python":     {"pytest", "python -m pytest"},
		"rust":       {"cargo test"},
		"elixir":     {"mix test"},
		"ruby":       {"rspec", "rake spec"},
		"java":       {"./gradlew test", "mvn test"},
		"typescript": {"npm test", "npx jest"},
	}

	if cmds, ok := commands[projectType]; ok {
		return cmds[0]
	}
	return "npm test"
}

func DetectProjectType(cwd string) string {
	files := map[string]string{
		"go.mod":           "go",
		"package.json":     "node",
		"pyproject.toml":   "python",
		"requirements.txt": "python",
		"Cargo.toml":       "rust",
		"mix.exs":          "elixir",
		"Gemfile":          "ruby",
		"pom.xml":          "java",
		"build.gradle":     "java",
	}

	for file, lang := range files {
		if fileExists(cwd, file) {
			return lang
		}
	}

	return "node"
}

func fileExists(dir, file string) bool {
	paths := []string{
		filepath.Join(dir, file),
		filepath.Join(dir, "..", file),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// TestRunner handles automatic test execution and snapshot updates
type TestRunner struct {
	cwd   string
	model string
}

func NewTestRunner(cwd string, model string) *TestRunner {
	return &TestRunner{
		cwd:   cwd,
		model: model,
	}
}

// TestResult holds the result of a test run
type TestResult struct {
	Passed        bool
	Command       string
	Output        string
	Failures      int
	SnapshotAware bool
	CanAutoFix    bool
}

// RunTests executes tests for the project
func (tr *TestRunner) RunTests(ctx context.Context) (*TestResult, error) {
	projectType := DetectProjectType(tr.cwd)
	testCmd := DetectTestCommand(projectType)

	return tr.runTestCommand(ctx, testCmd, projectType)
}

// RunTestsWithUpdate runs tests with snapshot update flag
func (tr *TestRunner) RunTestsWithUpdate(ctx context.Context) (*TestResult, error) {
	projectType := DetectProjectType(tr.cwd)
	testCmd := DetectUpdateCommand(projectType)

	return tr.runTestCommand(ctx, testCmd, projectType)
}

// RunTestsInDirectory runs tests in a specific directory
func (tr *TestRunner) RunTestsInDirectory(ctx context.Context, dir string) (*TestResult, error) {
	projectType := DetectProjectType(filepath.Join(tr.cwd, dir))
	testCmd := DetectTestCommand(projectType)

	return tr.runTestCommandInDir(ctx, testCmd, dir)
}

func (tr *TestRunner) runTestCommand(ctx context.Context, cmd string, projectType string) (*TestResult, error) {
	return tr.runTestCommandInDir(ctx, cmd, ".")
}

func (tr *TestRunner) runTestCommandInDir(ctx context.Context, cmd string, dir string) (*TestResult, error) {
	fullCmd := fmt.Sprintf("cd %s && %s", dir, cmd)

	result := &TestResult{
		Command: cmd,
	}

	output, err := runCommand(fullCmd)
	result.Output = output

	if err != nil {
		// Check if it's just a test failure (not a crash)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				result.Passed = false
			}
		} else {
			result.Passed = false
			result.Output += fmt.Sprintf("\nError: %v", err)
		}
	} else {
		result.Passed = true
	}

	// Analyze output
	result.Analyze()

	return result, nil
}

// Analyze examines test output for failures and snapshots
func (r *TestResult) Analyze() {
	output := r.Output

	// Count failures
	failMatch := regexp.MustCompile(`(\d+)\s+(?:failures?|errors?|failed)`)
	if matches := failMatch.FindStringSubmatch(strings.ToLower(output)); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &r.Failures)
	}

	// Check if snapshot tests
	r.SnapshotAware = strings.Contains(output, "snapshot") ||
		strings.Contains(output, "Snapshot") ||
		strings.Contains(output, "-u") ||
		strings.Contains(output, "--update")

	// Can auto-fix if snapshot related
	r.CanAutoFix = r.SnapshotAware && r.Failures > 0 && !r.Passed
}

// DetectUpdateCommand returns the command to update snapshots
func DetectUpdateCommand(projectType string) string {
	commands := map[string]string{
		"go":         "go test -update ./...",
		"node":       "npm test -- -u",
		"python":     "pytest --snapshot-update",
		"rust":       "cargo test",
		"elixir":     "mix test --trace",
		"ruby":       "rspec --update-snapshots",
		"typescript": "npm test -- -u",
	}

	if cmd, ok := commands[projectType]; ok {
		return cmd
	}
	return "npm test -- -u"
}

// runCommand executes a shell command
func runCommand(cmd string) (string, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	dir := "."
	command := cmd

	// Parse cd command
	if parts[0] == "cd" && len(parts) >= 2 {
		dir = parts[1]
		command = strings.Join(parts[3:], " ")
	}

	execCmd := exec.Command("sh", "-c", command)
	execCmd.Dir = dir
	output, err := execCmd.CombinedOutput()

	return string(output), err
}
