package recovery

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/validation"
)

type ErrorFixer struct {
	provider llm.Provider
	model    string
	workDir  string
}

type FixResult struct {
	Success       bool
	FixAttempts   int
	ErrorType     string
	OriginalError string
	FixApplied    string
	FinalStatus   string
}

func NewErrorFixer(provider llm.Provider, model string, workDir string) *ErrorFixer {
	return &ErrorFixer{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (ef *ErrorFixer) FixTestFailures(ctx context.Context, testResult *validation.TestResult, maxAttempts int) (*FixResult, error) {
	result := &FixResult{
		ErrorType:     "test_failure",
		OriginalError: testResult.Output,
		FixAttempts:   0,
	}

	if testResult.Success {
		result.Success = true
		result.FinalStatus = "No fixes needed - tests passing"
		return result, nil
	}

	failures := ef.extractTestFailures(testResult.Output)
	if len(failures) == 0 {
		return result, fmt.Errorf("could not extract test failures from output")
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.FixAttempts = attempt

		fixPrompt := ef.buildTestFixPrompt(failures, testResult.Output)

		resp, err := ef.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: `You are a code fixing expert. Analyze test failures and provide exact fixes.
Your response must be in this format:

## Analysis
[Brief description of the issue]

## Fix
[Exact code changes needed]

## Files to modify
- path/to/file.go: [specific change]`,
			UserPrompt: fixPrompt,
			Model:      ef.model,
		})

		if err != nil {
			return result, fmt.Errorf("LLM request failed: %w", err)
		}

		result.FixApplied = resp.Text

		testExec := validation.NewTestExecutor(ef.workDir)
		newResult, err := testExec.RunTests()
		if err != nil {
			continue
		}

		if newResult.Success {
			result.Success = true
			result.FinalStatus = fmt.Sprintf("Fixed after %d attempts", attempt)
			return result, nil
		}

		failures = ef.extractTestFailures(newResult.Output)
	}

	result.FinalStatus = fmt.Sprintf("Failed after %d attempts", maxAttempts)
	return result, fmt.Errorf("could not fix test failures after %d attempts", maxAttempts)
}

func (ef *ErrorFixer) FixLintIssues(ctx context.Context, lintResults []*validation.LintResult, maxAttempts int) (*FixResult, error) {
	result := &FixResult{
		ErrorType:   "lint_issues",
		FixAttempts: 0,
	}

	allPassed := true
	var issues []string
	for _, lr := range lintResults {
		if !lr.Success || lr.Issues > 0 {
			allPassed = false
			issues = append(issues, fmt.Sprintf("%s: %d issues", lr.Tool, lr.Issues))
		}
	}

	if allPassed {
		result.Success = true
		result.FinalStatus = "No fixes needed - all linters passing"
		return result, nil
	}

	result.OriginalError = strings.Join(issues, "; ")

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.FixAttempts = attempt

		fixPrompt := ef.buildLintFixPrompt(lintResults)

		resp, err := ef.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: `You are a code quality expert. Fix linting issues automatically.
Your response must include exact code changes.`,
			UserPrompt: fixPrompt,
			Model:      ef.model,
		})

		if err != nil {
			return result, fmt.Errorf("LLM request failed: %w", err)
		}

		result.FixApplied = resp.Text

		lintExec := validation.NewLinterExecutor(ef.workDir)
		newResults, err := lintExec.RunLinters()
		if err != nil {
			continue
		}

		allFixed := true
		for _, lr := range newResults {
			if !lr.Success || lr.Issues > 0 {
				allFixed = false
				break
			}
		}

		if allFixed {
			result.Success = true
			result.FinalStatus = fmt.Sprintf("Fixed after %d attempts", attempt)
			return result, nil
		}
	}

	result.FinalStatus = fmt.Sprintf("Failed after %d attempts", maxAttempts)
	return result, fmt.Errorf("could not fix lint issues after %d attempts", maxAttempts)
}

func (ef *ErrorFixer) FixSyntaxError(ctx context.Context, errorOutput string, language string) (*FixResult, error) {
	result := &FixResult{
		ErrorType:     "syntax_error",
		OriginalError: errorOutput,
		FixAttempts:   1,
	}

	syntaxError := ef.extractSyntaxError(errorOutput, language)
	if syntaxError == "" {
		return result, fmt.Errorf("could not extract syntax error")
	}

	fixPrompt := fmt.Sprintf(`Fix this %s syntax error:

%s

Provide the exact code fix needed.`, language, syntaxError)

	resp, err := ef.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a syntax error fixing expert. Provide exact fixes for compilation/syntax errors.",
		UserPrompt:   fixPrompt,
		Model:        ef.model,
	})

	if err != nil {
		return result, fmt.Errorf("LLM request failed: %w", err)
	}

	result.FixApplied = resp.Text
	result.Success = true
	result.FinalStatus = "Syntax fix provided"

	return result, nil
}

func (ef *ErrorFixer) FixGenericError(ctx context.Context, prompt string, errorOutput string, maxAttempts int) (*FixResult, error) {
	result := &FixResult{
		ErrorType:     "generic_error",
		OriginalError: errorOutput,
		FixAttempts:   1,
	}

	resp, err := ef.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: `You are an expert at diagnosing and fixing code errors.
Analyze the error, identify the root cause, and provide specific fixes.

Format your response as:
## Analysis
[What went wrong]

## Solution
[How to fix it]

## Changes
[Specific file and code changes]`,
		UserPrompt: prompt,
		Model:      ef.model,
	})

	if err != nil {
		return result, fmt.Errorf("LLM request failed: %w", err)
	}

	result.FixApplied = resp.Text
	result.Success = true
	result.FinalStatus = "Fix analysis provided"

	return result, nil
}

func (ef *ErrorFixer) extractTestFailures(output string) []string {
	var failures []string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "FAIL:") || strings.Contains(line, "Error:") {
			failures = append(failures, strings.TrimSpace(line))
		}
	}

	failPattern := regexp.MustCompile(`--- FAIL: (\S+)`)
	matches := failPattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) > 1 {
			failures = append(failures, "Test failed: "+match[1])
		}
	}

	return failures
}

func (ef *ErrorFixer) extractSyntaxError(output string, language string) string {
	lines := strings.Split(output, "\n")

	var errorLines []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error:") ||
			strings.Contains(lower, "syntaxerror") ||
			strings.Contains(lower, "unexpected") {
			errorLines = append(errorLines, line)
		}
	}

	if len(errorLines) > 0 {
		return strings.Join(errorLines, "\n")
	}

	return output
}

func (ef *ErrorFixer) buildTestFixPrompt(failures []string, fullOutput string) string {
	prompt := "Fix these test failures:\n\n"

	for i, failure := range failures {
		if i >= 5 {
			prompt += fmt.Sprintf("\n...and %d more failures", len(failures)-5)
			break
		}
		prompt += fmt.Sprintf("%d. %s\n", i+1, failure)
	}

	prompt += "\nFull test output:\n```\n"

	outputLines := strings.Split(fullOutput, "\n")
	if len(outputLines) > 50 {
		prompt += strings.Join(outputLines[:50], "\n")
		prompt += fmt.Sprintf("\n... (%d more lines)", len(outputLines)-50)
	} else {
		prompt += fullOutput
	}
	prompt += "\n```\n\n"
	prompt += "Provide exact code changes to fix these test failures."

	return prompt
}

func (ef *ErrorFixer) buildLintFixPrompt(lintResults []*validation.LintResult) string {
	prompt := "Fix these linting issues:\n\n"

	for _, result := range lintResults {
		if !result.Success || result.Issues > 0 {
			prompt += fmt.Sprintf("## %s (%d issues)\n", result.Tool, result.Issues)

			outputLines := strings.Split(result.Output, "\n")
			lineCount := 0
			for _, line := range outputLines {
				if strings.TrimSpace(line) != "" {
					prompt += line + "\n"
					lineCount++
					if lineCount >= 20 {
						prompt += "...(truncated)\n"
						break
					}
				}
			}
			prompt += "\n"
		}
	}

	prompt += "Provide exact code changes to fix these issues."
	return prompt
}

type RecoveryStrategy string

const (
	RetryWithFix     RecoveryStrategy = "retry_with_fix"
	SimplifyApproach RecoveryStrategy = "simplify_approach"
	SkipAndContinue  RecoveryStrategy = "skip_and_continue"
	Rollback         RecoveryStrategy = "rollback"
)

func (ef *ErrorFixer) DetermineStrategy(ctx context.Context, errorType string, attempts int, maxAttempts int) RecoveryStrategy {
	if attempts >= maxAttempts {
		return Rollback
	}

	if errorType == "syntax_error" {
		return RetryWithFix
	}

	if attempts < maxAttempts/2 {
		return RetryWithFix
	}

	return SimplifyApproach
}
