package validation

import (
	"bytes"
	"fmt"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"os/exec"
	"path/filepath"
	"strings"
)

type TestResult struct {
	Success      bool
	Output       string
	Passed       int
	Failed       int
	Skipped      int
	Duration     string
	ErrorMessage string
}

type TestExecutor struct {
	workDir string
}

func NewTestExecutor(workDir string) *TestExecutor {
	return &TestExecutor{workDir: workDir}
}

func (te *TestExecutor) RunTests() (*TestResult, error) {
	lang := langdetect.DetectLanguage(te.workDir)

	switch lang {
	case langdetect.Go:
		return te.runGoTests()
	case langdetect.TypeScript:
		return te.runNodeTests()
	case langdetect.Python:
		return te.runPythonTests()
	case langdetect.Elixir:
		return te.runElixirTests()
	case langdetect.Ruby:
		return te.runRubyTests()
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func (te *TestExecutor) runGoTests() (*TestResult, error) {
	cmd := exec.Command("go", "test", "./...", "-v")
	cmd.Dir = te.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &TestResult{
		Success: err == nil,
		Output:  output,
	}

	result.parseGoOutput(output)

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result, nil
}

func (te *TestExecutor) runNodeTests() (*TestResult, error) {
	testCmd := "npm"
	args := []string{"test"}

	if fileExists(filepath.Join(te.workDir, "yarn.lock")) {
		testCmd = "yarn"
	} else if fileExists(filepath.Join(te.workDir, "pnpm-lock.yaml")) {
		testCmd = "pnpm"
	}

	cmd := exec.Command(testCmd, args...)
	cmd.Dir = te.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &TestResult{
		Success: err == nil,
		Output:  output,
	}

	result.parseJestOutput(output)

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result, nil
}

func (te *TestExecutor) runPythonTests() (*TestResult, error) {
	testCmd := "pytest"
	args := []string{"-v"}

	if fileExists(filepath.Join(te.workDir, "manage.py")) {
		testCmd = "python"
		args = []string{"manage.py", "test"}
	}

	cmd := exec.Command(testCmd, args...)
	cmd.Dir = te.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &TestResult{
		Success: err == nil,
		Output:  output,
	}

	result.parsePytestOutput(output)

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result, nil
}

func (te *TestExecutor) runElixirTests() (*TestResult, error) {
	cmd := exec.Command("mix", "test")
	cmd.Dir = te.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &TestResult{
		Success: err == nil,
		Output:  output,
	}

	result.parseElixirOutput(output)

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result, nil
}

func (te *TestExecutor) runRubyTests() (*TestResult, error) {
	testCmd := "rake"
	args := []string{"test"}

	if fileExists(filepath.Join(te.workDir, "spec")) {
		testCmd = "rspec"
		args = []string{}
	}

	cmd := exec.Command(testCmd, args...)
	cmd.Dir = te.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &TestResult{
		Success: err == nil,
		Output:  output,
	}

	result.parseRSpecOutput(output)

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result, nil
}

func (r *TestResult) parseGoOutput(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--- PASS:") {
			r.Passed++
		} else if strings.Contains(line, "--- FAIL:") {
			r.Failed++
		} else if strings.Contains(line, "--- SKIP:") {
			r.Skipped++
		}
	}

	if strings.Contains(output, "PASS") && r.Failed == 0 {
		r.Success = true
	}
}

func (r *TestResult) parseJestOutput(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Tests:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "passed," && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Passed)
				} else if part == "failed," && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Failed)
				} else if strings.HasPrefix(part, "skipped") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Skipped)
				}
			}
		}
	}
}

func (r *TestResult) parsePytestOutput(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, " passed") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "passed" && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Passed)
				} else if part == "failed" && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Failed)
				} else if part == "skipped" && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Skipped)
				}
			}
		}
	}
}

func (r *TestResult) parseElixirOutput(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "tests,") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(part, "tests") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Passed)
				} else if strings.Contains(part, "failure") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Failed)
				}
			}
		}
	}
}

func (r *TestResult) parseRSpecOutput(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "examples,") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(part, "examples") && i > 0 {
					total := 0
					fmt.Sscanf(parts[i-1], "%d", &total)
				} else if strings.Contains(part, "failure") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &r.Failed)
				}
			}
		}
	}
	r.Passed = r.Passed - r.Failed
}

func fileExists(path string) bool {
	cmd := exec.Command("test", "-f", path)
	return cmd.Run() == nil
}
