package validation

import (
	"bytes"
	"fmt"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"os/exec"
	"path/filepath"
	"strings"
)

type LintResult struct {
	Success      bool
	Output       string
	Issues       int
	Warnings     int
	Errors       int
	Tool         string
	ErrorMessage string
}

type LinterExecutor struct {
	workDir string
}

func NewLinterExecutor(workDir string) *LinterExecutor {
	return &LinterExecutor{workDir: workDir}
}

func (le *LinterExecutor) RunLinters() ([]*LintResult, error) {
	lang := langdetect.DetectLanguage(le.workDir)

	switch lang {
	case langdetect.Go:
		return le.runGoLinters()
	case langdetect.TypeScript:
		return le.runNodeLinters()
	case langdetect.Python:
		return le.runPythonLinters()
	case langdetect.Elixir:
		return le.runElixirLinters()
	case langdetect.Ruby:
		return le.runRubyLinters()
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func (le *LinterExecutor) runGoLinters() ([]*LintResult, error) {
	results := []*LintResult{}

	if commandExists("golangci-lint") {
		result := le.runLinter("golangci-lint", []string{"run", "./..."})
		result.Tool = "golangci-lint"
		results = append(results, result)
	}

	vetResult := le.runLinter("go", []string{"vet", "./..."})
	vetResult.Tool = "go vet"
	results = append(results, vetResult)

	return results, nil
}

func (le *LinterExecutor) runNodeLinters() ([]*LintResult, error) {
	results := []*LintResult{}

	packageJSON := filepath.Join(le.workDir, "package.json")
	if !fileExists(packageJSON) {
		return results, nil
	}

	if commandExists("eslint") {
		result := le.runLinter("eslint", []string{".", "--ext", ".js,.jsx,.ts,.tsx"})
		result.Tool = "eslint"
		results = append(results, result)
	}

	if fileExists(filepath.Join(le.workDir, "tsconfig.json")) && commandExists("tsc") {
		result := le.runLinter("tsc", []string{"--noEmit"})
		result.Tool = "tsc"
		results = append(results, result)
	}

	if commandExists("prettier") {
		result := le.runLinter("prettier", []string{"--check", "."})
		result.Tool = "prettier"
		results = append(results, result)
	}

	return results, nil
}

func (le *LinterExecutor) runPythonLinters() ([]*LintResult, error) {
	results := []*LintResult{}

	if commandExists("ruff") {
		result := le.runLinter("ruff", []string{"check", "."})
		result.Tool = "ruff"
		results = append(results, result)
	} else if commandExists("flake8") {
		result := le.runLinter("flake8", []string{"."})
		result.Tool = "flake8"
		results = append(results, result)
	}

	if commandExists("mypy") {
		result := le.runLinter("mypy", []string{"."})
		result.Tool = "mypy"
		results = append(results, result)
	}

	if commandExists("black") {
		result := le.runLinter("black", []string{"--check", "."})
		result.Tool = "black"
		results = append(results, result)
	}

	return results, nil
}

func (le *LinterExecutor) runElixirLinters() ([]*LintResult, error) {
	results := []*LintResult{}

	if commandExists("mix") {
		credoResult := le.runLinter("mix", []string{"credo", "--strict"})
		credoResult.Tool = "credo"
		results = append(results, credoResult)

		dialyzerResult := le.runLinter("mix", []string{"dialyzer"})
		dialyzerResult.Tool = "dialyzer"
		results = append(results, dialyzerResult)

		formatResult := le.runLinter("mix", []string{"format", "--check-formatted"})
		formatResult.Tool = "mix format"
		results = append(results, formatResult)
	}

	return results, nil
}

func (le *LinterExecutor) runRubyLinters() ([]*LintResult, error) {
	results := []*LintResult{}

	if commandExists("rubocop") {
		result := le.runLinter("rubocop", []string{})
		result.Tool = "rubocop"
		results = append(results, result)
	}

	return results, nil
}

func (le *LinterExecutor) runLinter(command string, args []string) *LintResult {
	cmd := exec.Command(command, args...)
	cmd.Dir = le.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	result := &LintResult{
		Success: err == nil,
		Output:  output,
	}

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	result.parseIssues(output)

	return result
}

func (r *LintResult) parseIssues(output string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "✗") {
			r.Errors++
			r.Issues++
		} else if strings.Contains(lower, "warning") || strings.Contains(lower, "⚠") {
			r.Warnings++
			r.Issues++
		}
	}

	if strings.Contains(output, "0 issues") || strings.Contains(output, "no issues") {
		r.Issues = 0
		r.Errors = 0
		r.Warnings = 0
	}
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
