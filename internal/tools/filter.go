package tools

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ANSI sequence regex
var ansiRegex = regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

// StripANSI removes ANSI escape codes from string
func StripANSI(input string) string {
	return ansiRegex.ReplaceAllString(input, "")
}

// FilterEngine manages output filtering and compression
type FilterEngine struct {
	teeDir string
}

// NewFilterEngine creates a new filter engine instance
func NewFilterEngine() *FilterEngine {
	home, _ := os.UserHomeDir()
	teeDir := filepath.Join(home, ".gptcode", "tee")
	return &FilterEngine{teeDir: teeDir}
}

// FilterResult contains metrics and the optimized output
type FilterResult struct {
	FilteredOutput string
	TokensSaved    int
	OriginalLen    int
	FilteredLen    int
	TeePath        string
}

// Filter processes a command's execution output and compresses it
func (fe *FilterEngine) Filter(command string, output string, exitCode int) FilterResult {
	originalLen := len(output)
	cleaned := StripANSI(output)

	// Determine matching filter
	filtered := cleaned
	isFiltered := false

	normalizedCmd := strings.TrimSpace(strings.ToLower(command))

	// 1. Check for test commands
	if strings.Contains(normalizedCmd, "go test") {
		filtered = fe.filterGoTest(cleaned)
		isFiltered = true
	} else if strings.Contains(normalizedCmd, "pytest") || strings.Contains(normalizedCmd, "manage.py test") {
		filtered = fe.filterPytest(cleaned)
		isFiltered = true
	} else if strings.Contains(normalizedCmd, "cargo test") {
		filtered = fe.filterCargoTest(cleaned)
		isFiltered = true
	} else if strings.Contains(normalizedCmd, "npm test") || strings.Contains(normalizedCmd, "yarn test") || strings.Contains(normalizedCmd, "pnpm test") || strings.Contains(normalizedCmd, "vitest") || strings.Contains(normalizedCmd, "jest") {
		filtered = fe.filterNodeTest(cleaned)
		isFiltered = true
	} else // 2. Check for package installer commands
	if strings.Contains(normalizedCmd, "npm install") || strings.Contains(normalizedCmd, "pnpm install") || strings.Contains(normalizedCmd, "yarn install") || strings.Contains(normalizedCmd, "pnpm add") || strings.Contains(normalizedCmd, "yarn add") {
		filtered = fe.filterNpmInstall(cleaned)
		isFiltered = true
	} else if strings.Contains(normalizedCmd, "bundle install") || strings.Contains(normalizedCmd, "bundle update") {
		filtered = fe.filterBundleInstall(cleaned)
		isFiltered = true
	} else // 3. Check for git commands
	if strings.HasPrefix(normalizedCmd, "git status") {
		filtered = fe.filterGitStatus(cleaned)
		isFiltered = true
	} else if strings.HasPrefix(normalizedCmd, "git diff") {
		filtered = fe.filterGitDiff(cleaned)
		isFiltered = true
	} else if strings.HasPrefix(normalizedCmd, "git log") {
		filtered = fe.filterGitLog(cleaned)
		isFiltered = true
	} else // 4. Check for linters
	if strings.Contains(normalizedCmd, "eslint") || strings.Contains(normalizedCmd, "ruff check") || strings.Contains(normalizedCmd, "golangci-lint") {
		filtered = fe.filterLinter(cleaned)
		isFiltered = true
	}

	// Apply default truncation if still too long and not filtered
	if !isFiltered && len(filtered) > 8000 {
		lines := strings.Split(filtered, "\n")
		if len(lines) > 200 {
			filtered = strings.Join(lines[:100], "\n") +
				fmt.Sprintf("\n... [truncated %d lines of verbose output] ...\n", len(lines)-200) +
				strings.Join(lines[len(lines)-100:], "\n")
		}
	}

	// Tee feature: if execution failed (non-zero exit code), save full raw log
	var teePath string
	if exitCode != 0 && originalLen > 0 {
		var err error
		teePath, err = fe.saveToTee(command, output)
		if err == nil {
			// Inject footnote informing the agent about the full logs
			filtered = filtered + fmt.Sprintf("\n\n---\n[GPTCode Tee] Command failed. Full unfiltered output saved to local log: %s\nUse read_file or run a command to view it if you need the full stack trace.", teePath)
		}
	}

	filteredLen := len(filtered)

	// Heuristic token calculation: 4 characters per token
	originalTokens := (originalLen + 3) / 4
	filteredTokens := (filteredLen + 3) / 4
	tokensSaved := originalTokens - filteredTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	return FilterResult{
		FilteredOutput: filtered,
		TokensSaved:    tokensSaved,
		OriginalLen:    originalLen,
		FilteredLen:    filteredLen,
		TeePath:        teePath,
	}
}

// saveToTee stores raw output to local file under ~/.gptcode/tee/
func (fe *FilterEngine) saveToTee(command string, output string) (string, error) {
	if err := os.MkdirAll(fe.teeDir, 0755); err != nil {
		return "", err
	}

	// Generate safe file name
	h := sha256.New()
	h.Write([]byte(command))
	cmdHash := fmt.Sprintf("%x", h.Sum(nil))[:8]

	// Sanitize command name for filename
	cleanCmd := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, command)
	if len(cleanCmd) > 30 {
		cleanCmd = cleanCmd[:30]
	}
	cleanCmd = strings.Trim(cleanCmd, "_")

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.log", timestamp, cleanCmd, cmdHash)
	fullPath := filepath.Join(fe.teeDir, filename)

	content := fmt.Sprintf("Command: %s\nTimestamp: %s\nExit Code: Failed\n\n================ RAW OUTPUT ================\n%s",
		command, time.Now().Format(time.RFC3339), output)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", err
	}

	return fullPath, nil
}

// Filter implementations

func (fe *FilterEngine) filterGoTest(output string) string {
	if !strings.Contains(output, "--- FAIL:") && (strings.Contains(output, "PASS") || strings.Contains(output, "ok  \t")) {
		// All tests passed, keep only final status line
		lines := strings.Split(output, "\n")
		var passSummary []string
		for _, line := range lines {
			if strings.Contains(line, "ok  \t") || strings.Contains(line, "PASS") {
				passSummary = append(passSummary, line)
			}
		}
		if len(passSummary) > 0 {
			return "ok go test: all tests passed\n" + strings.Join(passSummary, "\n")
		}
		return "ok go test: all tests passed"
	}

	// Tests failed: filter to keep failures and errors
	lines := strings.Split(output, "\n")
	var result []string
	inFailureBlock := false
	failureBlockLines := 0

	for _, line := range lines {
		isFailLine := strings.Contains(line, "--- FAIL:") || strings.Contains(line, "FAIL\t")
		isPassLine := strings.Contains(line, "--- PASS:")

		if isFailLine {
			inFailureBlock = true
			failureBlockLines = 0
			result = append(result, line)
		} else if isPassLine {
			inFailureBlock = false
		} else if inFailureBlock {
			if failureBlockLines < 30 { // limit individual failure trace size to 30 lines
				result = append(result, line)
				failureBlockLines++
			} else if failureBlockLines == 30 {
				result = append(result, "   ... [trace truncated] ...")
				failureBlockLines++
			}
		} else if strings.HasPrefix(line, "# ") ||
			strings.Contains(strings.ToLower(line), "undefined") ||
			strings.Contains(strings.ToLower(line), "compile") ||
			strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(line, "_test.go:") ||
			strings.HasPrefix(line, "    ") {
			// Keep compilation errors/package build errors and test failures
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return output // Fallback
	}

	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterPytest(output string) string {
	if !strings.Contains(output, "failed") && (strings.Contains(output, " passed in ") || strings.Contains(output, "OK")) {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "passed in") || strings.Contains(line, "====") {
				return "ok pytest: all tests passed\n" + line
			}
		}
		return "ok pytest: all tests passed"
	}

	lines := strings.Split(output, "\n")
	var result []string
	inFailureSummary := false

	for _, line := range lines {
		if strings.Contains(line, "==== FAILURES ====") {
			inFailureSummary = true
		}

		if inFailureSummary {
			result = append(result, line)
		} else if strings.Contains(line, "FAIL") || strings.Contains(line, "ERROR") {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterCargoTest(output string) string {
	if !strings.Contains(output, "FAILED") && strings.Contains(output, "test result: ok.") {
		lines := strings.Split(output, "\n")
		var passSummary []string
		for _, line := range lines {
			if strings.Contains(line, "test result: ok.") {
				passSummary = append(passSummary, line)
			}
		}
		return "ok cargo test: all tests passed\n" + strings.Join(passSummary, "\n")
	}

	lines := strings.Split(output, "\n")
	var result []string
	inFailureDetails := false

	for _, line := range lines {
		if strings.Contains(line, "failures:") {
			inFailureDetails = true
		}
		if inFailureDetails {
			result = append(result, line)
		} else if strings.Contains(line, "FAILED") {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterNodeTest(output string) string {
	// Simple filter for Jest/Vitest
	if !strings.Contains(output, "FAIL") && (strings.Contains(output, "Pass") || strings.Contains(output, "pass")) {
		lines := strings.Split(output, "\n")
		var summary []string
		for _, line := range lines {
			if strings.Contains(line, "Test Suites:") || strings.Contains(line, "Tests:") || strings.Contains(line, "Snapshots:") {
				summary = append(summary, line)
			}
		}
		if len(summary) > 0 {
			return "ok node tests: all tests passed\n" + strings.Join(summary, "\n")
		}
		return "ok node tests: all tests passed"
	}

	// Keep only failed test suites details
	lines := strings.Split(output, "\n")
	var result []string
	inError := false

	for _, line := range lines {
		if strings.Contains(line, "FAIL") || strings.Contains(line, "Error:") || strings.Contains(line, "Failed") {
			inError = true
		}
		if inError {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterNpmInstall(output string) string {
	lines := strings.Split(output, "\n")
	var result []string
	for _, line := range lines {
		// Only keep summaries, vulnerabilities, warnings and errors
		if strings.Contains(line, "added ") || strings.Contains(line, "removed ") ||
			strings.Contains(line, "audited ") || strings.Contains(line, "warn") ||
			strings.Contains(line, "ERR") || strings.Contains(line, "vulnerabilit") {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return "ok npm install completed successfully"
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterBundleInstall(output string) string {
	lines := strings.Split(output, "\n")
	var result []string
	for _, line := range lines {
		// Keep install actions, errors, and bundle complete message
		if strings.Contains(line, "Installing ") || strings.Contains(line, "Bundle complete!") ||
			strings.Contains(line, "Bundle updated!") || strings.Contains(line, "error") ||
			strings.Contains(line, "Error") {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return "ok bundle install completed"
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterGitStatus(output string) string {
	lines := strings.Split(output, "\n")
	var result []string
	for _, line := range lines {
		// Keep modified, new, deleted lines, branch name, and drop help guides
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "modified:") || strings.HasPrefix(trimmed, "new file:") ||
			strings.HasPrefix(trimmed, "deleted:") || strings.Contains(line, "On branch") ||
			strings.Contains(line, "Your branch is") || strings.Contains(line, "Untracked files:") ||
			(strings.HasPrefix(trimmed, "") && len(trimmed) > 0 && !strings.Contains(line, "use \"git")) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func (fe *FilterEngine) filterGitDiff(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 200 {
		// Truncate long git diffs to avoid LLM context blowout
		return strings.Join(lines[:80], "\n") +
			fmt.Sprintf("\n... [git diff truncated %d lines] ...\n", len(lines)-160) +
			strings.Join(lines[len(lines)-80:], "\n")
	}
	return output
}

func (fe *FilterEngine) filterGitLog(output string) string {
	// git log is usually clean, but we enforce limit
	lines := strings.Split(output, "\n")
	if len(lines) > 40 {
		return strings.Join(lines[:30], "\n") + fmt.Sprintf("\n... [git log truncated %d lines] ...", len(lines)-30)
	}
	return output
}

func (fe *FilterEngine) filterLinter(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 100 {
		// Keep only the first 50 lines and summary of errors
		return strings.Join(lines[:50], "\n") +
			fmt.Sprintf("\n... [linter output truncated %d lines] ...\n", len(lines)-50)
	}
	return output
}
