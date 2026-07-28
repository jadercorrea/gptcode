package agents

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/tools"
)

type ReviewerAgent struct {
	provider llm.Provider
	cwd      string
	model    string
}

type ReviewResult struct {
	Success     bool
	Issues      []string
	Suggestions string
}

type ReviewerConfig struct {
	OnValidationFail func(issues []string) (shouldRetry bool, newBackend string, newModel string)
}

func NewReviewer(provider llm.Provider, cwd string, model string) *ReviewerAgent {
	return &ReviewerAgent{
		provider: provider,
		cwd:      cwd,
		model:    model,
	}
}

const reviewerPrompt = `You are a STRICT code reviewer. Your job is to verify if changes EXACTLY meet ALL success criteria.

WORKFLOW:
1. Read the files that were modified (if any)
2. Check EACH success criterion one by one
3. Run commands to verify if needed (build, test, lint, etc)
4. Report pass/fail - be STRICT, not lenient

SPECIAL CASE - QUERY/READ-ONLY TASKS:
- If NO files were modified AND the task was a query/read-only command (git status, read file, show data)
- AND the command executed successfully without errors
- Then immediately report SUCCESS - there's nothing to validate
- Query tasks don't need build/compile validation

CRITICAL RULES:
- **ALL CRITERIA MUST PASS** - if even ONE fails, report FAIL
- **BE SPECIFIC**: Don't say "looks good" - verify each criterion explicitly
- **CHECK FILE EXISTENCE**: If criterion says "file X must exist", actually check it
- **CHECK FILE CONTENT**: If criterion says "file must contain Y", read and verify
- **ALWAYS RUN BUILD COMMANDS** if success criteria includes build/compile commands
- ONLY skip build if no files were modified (read-only tasks like git status, gh pr list)
- Skip build if only documentation files (.md, .txt, .json) were modified
- For Go code changes: run 'go build ./...' to check compilation
- For Elixir/Phoenix: run 'mix compile' to check compilation
- For TypeScript/Node code changes: run 'npm run build' or 'tsc'
- For Python code changes: check syntax with 'python -m py_compile file.py'
- **EXTRACT EXACT ERRORS**: When build fails, copy the EXACT error message with file:line
- If something is missing or wrong, say EXACTLY what and where
- **SAY "SUCCESS" ONLY if ALL criteria pass**
- Focus on the actual requirements, not style

VERSION VALIDATION FLEXIBILITY:
When validating version numbers, be FLEXIBLE about format:
- **Elixir (mix.exs)**: Accept "~> 1.15.4", ">= 1.15.4", "1.15.4" as valid for "version 1.15.4"
- **Node.js (package.json)**: Accept "^18.2.0", "~18.2.0", "18.2.0" as valid for "version 18.2.0"
- **Python (requirements.txt)**: Accept "==4.2.0", ">=4.2.0", "~=4.2.0" as valid for "version 4.2.0"
- **Go (go.mod)**: Accept "v1.2.3" or "1.2.3" as valid for "version 1.2.3"
- **Ruby (Gemfile)**: Accept "~> 7.0.0" or "7.0.0" as valid for "version 7.0.0"

The SEMANTIC version is what matters, not the exact operator syntax.
If criteria says "version 1.15.4" and file has "~> 1.15.4", that's SUCCESS.

SNAPSHOT TEST HANDLING:
When tests fail due to snapshot differences:
1. Look at the diff: "- Snapshot" = old, "+ Received" = new
2. Evaluate if the NEW output is BETTER (correct fix) or WRONG (bug)
3. If the new output is correct (better formatting, improved style):
   → Report SUCCESS with note: "Snapshot differences are improvements"
4. If the new output is wrong (bug introduced):
   → Report FAIL with specific issue

SNAPSHOT IMPROVEMENTS include:
- Better line breaking / formatting
- Consistent style changes
- Improved error messages
- Removal of unnecessary whitespace

EXAMPLES:
Success Criteria:
  - Tests pass: go test ./auth/...
  - File auth/handler.go contains Login function
  - Lints clean: golangci-lint run

Validation:
  1. Read auth/handler.go → contains func Login()
  2. Would run: go test ./auth/... → (assume passes)
  3. Would run: golangci-lint run → (assume clean)

Result: SUCCESS

EXAMPLE 2 - Validation FAIL with specific issues:
Success Criteria:
  - Tests pass: make test
  - File middleware/jwt.go exports VerifyToken function
  - No hardcoded secrets

Validation:
  1. Read middleware/jwt.go
  
Issues found:
  - FAIL: VerifyToken is not exported (lowercase verifyToken)
  - FAIL: JWT secret is hardcoded on line 15: "hardcoded-secret-123"
  - Tests: Would need to run make test to verify

Result:
FAIL

Issues:
- VerifyToken function must be exported (capitalize: VerifyToken)
- Remove hardcoded secret on line 15, use environment variable
- Run make test to verify tests pass

EXAMPLE 3 - Compilation error (extract exact error):
Success Criteria:
  - mix compile succeeds
  - File lib/my_app.ex contains new function

Validation:
  1. Read lib/my_app.ex → looks good
  2. Run: mix compile
     Output: 
     == Compilation error in file lib/my_app.ex ==
     ** (SyntaxError) lib/my_app.ex:10: syntax error before: 'end'

Result:
FAIL

Issues:
- Syntax error at lib/my_app.ex:10 - missing 'do' keyword before 'end'
- Run mix compile to verify after fix

EXAMPLE 3 - Snapshot test improvement (should report SUCCESS):
Success Criteria:
  - Code formatting improved
  - Tests pass

Validation:
  1. Read modified files → formatting changes look correct
  2. Run: npm test
     Output:
     SNAPSHOT SUMMARY: 3 snapshots updated
     + Received 3 snapshots

  The new formatting output is BETTER (improved line breaking).
  Snapshot updates are expected and correct.

Result: SUCCESS
Note: Snapshot differences are improvements - new formatting is better

EXAMPLE 4 - Clear issue reporting:
BAD:
  "Something is wrong with the file"
  
GOOD:
  "middleware/auth.go line 42: Missing error handling for jwt.Parse"
  "tests/auth_test.go: No test for Login with invalid password case"

Be direct and precise.`

func (v *ReviewerAgent) Review(ctx context.Context, plan string, modifiedFiles []string, statusCallback StatusCallback) (*ReviewResult, error) {
	if statusCallback != nil {
		statusCallback("Reviewer: Analyzing changes...")
	}

	toolDefs := []interface{}{
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_file",
				"description": "Read file contents",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "run_command",
				"description": "Run shell command to verify build, tests, linter, etc",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command to execute (e.g. 'go build', 'npm test')",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}

	filesStr := ""
	if len(modifiedFiles) > 0 {
		filesStr = fmt.Sprintf("\nFiles that were modified: %v", modifiedFiles)
	}

	reviewPrompt := fmt.Sprintf(`Validate if the implementation meets the requirements.

Plan and Success Criteria:
---
%s
---
%s

TASK:
1. Read the modified files to see what was changed (if any)
2. **ONLY** run build commands if code files (.go, .py, .js, .ts, etc) were modified
3. Skip build if:
   - No files were modified (read-only task like 'git status')
   - Only docs (.md, .txt, .json, .yaml) were modified
4. Check if changes meet the success criteria from the plan
5. Report:
   - Say "SUCCESS" if all criteria are met
   - If build was needed and failed, report "FAIL" with specific errors
   - If requirements not met, list specific issues

Be smart: Don't run 'go build' if this is not a Go project or no Go files changed.
Be precise and specific.`, plan, filesStr)

	history := []llm.ChatMessage{
		{Role: "user", Content: reviewPrompt},
	}

	// Tool call processing loop for validation - Maestro controls outer retry logic.
	// Lower internal limit (5) since most validations complete in 2-3 iterations.
	maxIterations := 3
	for i := 0; i < maxIterations; i++ {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[VALIDATOR] Iteration %d/%d\n", i+1, maxIterations)
		}

		resp, err := v.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: reviewerPrompt,
			Messages:     history,
			Tools:        toolDefs,
			Model:        v.model,
		})
		if err != nil {
			return nil, err
		}

		if len(resp.ToolCalls) == 0 {
			result := &ReviewResult{
				Success:     false,
				Issues:      []string{},
				Suggestions: resp.Text,
			}

			if containsSuccess(resp.Text) {
				result.Success = true
			} else {
				result.Issues = extractIssues(resp.Text)
			}

			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[VALIDATOR] Result: success=%v, issues=%v\n", result.Success, result.Issues)
			}

			return result, nil
		}

		history = append(history, llm.ChatMessage{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			llmCall := tools.LLMToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
			result := tools.ExecuteToolFromLLMContext(ctx, llmCall, v.cwd)

			content := result.Result
			if result.Error != "" {
				content = "Error: " + result.Error
			}
			if content == "" {
				content = "Success"
			}

			// Truncate very long content to prevent API errors
			// Max ~10k chars (~2500 tokens) per tool result
			maxContentLength := 10000
			if len(content) > maxContentLength {
				lines := strings.Split(content, "\n")
				if len(lines) > 200 {
					// Show first 100 and last 100 lines for large files
					firstLines := strings.Join(lines[:100], "\n")
					lastLines := strings.Join(lines[len(lines)-100:], "\n")
					content = fmt.Sprintf("%s\n\n... [%d lines omitted] ...\n\n%s",
						firstLines, len(lines)-200, lastLines)
				} else {
					// Just truncate by character count
					content = content[:maxContentLength] + "\n\n... [truncated]"
				}
			}

			history = append(history, llm.ChatMessage{
				Role:       "tool",
				Content:    content,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	history = append(history, llm.ChatMessage{
		Role: "user",
		Content: `Tool use is now complete. Based only on the evidence already collected, return a final verdict.
Start with exactly SUCCESS or FAIL. If FAIL, list concrete unmet requirements.`,
	})
	resp, err := v.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: reviewerPrompt,
		Messages:     history,
		Model:        v.model,
	})
	if err != nil {
		return nil, err
	}
	result := &ReviewResult{Suggestions: resp.Text}
	verdict := explicitReviewVerdict(resp.Text)
	result.Success = verdict == "success"
	if verdict == "" {
		return nil, fmt.Errorf("reviewer returned no explicit SUCCESS or FAIL verdict")
	}
	if !result.Success {
		result.Issues = extractIssues(resp.Text)
		if len(result.Issues) == 0 {
			result.Issues = []string{"Reviewer did not provide a conclusive evidence-based verdict"}
		}
	}
	return result, nil
}

func explicitReviewVerdict(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		first := strings.ToUpper(strings.Trim(strings.Fields(line)[0], ":.-"))
		switch first {
		case "SUCCESS":
			return "success"
		case "FAIL":
			return "fail"
		default:
			return ""
		}
	}
	return ""
}

func containsSuccess(text string) bool {
	lowerText := strings.ToLower(text)

	// Check for explicit success statements
	if strings.Contains(lowerText, "task is complete") ||
		strings.Contains(lowerText, "task was completed") ||
		strings.Contains(lowerText, "success criteria are met") ||
		strings.Contains(lowerText, "all criteria are met") ||
		strings.Contains(lowerText, "requirements are met") ||
		strings.Contains(lowerText, "successfully completed") ||
		strings.Contains(lowerText, "executed successfully") ||
		strings.Contains(lowerText, "completed successfully") ||
		strings.Contains(lowerText, "ran successfully") ||
		strings.Contains(lowerText, "no errors") {
		return true
	}

	// Fallback: explicit SUCCESS keyword without failure indicators
	if strings.Contains(lowerText, "success") {
		hasFail := strings.Contains(lowerText, " fail") ||
			strings.Contains(lowerText, "failed") ||
			strings.Contains(lowerText, " error") ||
			strings.Contains(lowerText, "not met")
		return !hasFail
	}

	return false
}

func extractIssues(text string) []string {
	issues := []string{}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if trimmed == "" {
			continue
		}

		if strings.HasSuffix(trimmed, ":") {
			continue
		}

		isFailureKeyword := strings.Contains(lower, "fail") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "not met") ||
			strings.Contains(lower, "missing") ||
			strings.Contains(lower, "incorrect") ||
			strings.Contains(lower, "invalid") ||
			strings.Contains(lower, "does not") ||
			strings.Contains(lower, "did not") ||
			strings.Contains(lower, "no such") ||
			strings.Contains(lower, "cannot")

		isSuccessPhrase := strings.Contains(lower, "success") ||
			strings.Contains(lower, "complete") ||
			strings.Contains(lower, "correct") ||
			strings.Contains(lower, "includes") ||
			strings.Contains(lower, "met") ||
			strings.Contains(lower, "display") ||
			strings.Contains(lower, "show") ||
			strings.Contains(lower, "executed successfully") ||
			strings.Contains(lower, "no errors")

		if isFailureKeyword && !isSuccessPhrase {
			issues = append(issues, trimmed)
		} else if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•") {
			if isFailureKeyword && !isSuccessPhrase {
				issues = append(issues, trimmed)
			}
		}
	}

	return issues
}
