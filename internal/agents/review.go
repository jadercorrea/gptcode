package agents

import (
	"context"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/tools"
)

type ReviewAgent struct {
	provider llm.Provider
	cwd      string
	model    string
}

func NewReview(provider llm.Provider, cwd string, model string) *ReviewAgent {
	return &ReviewAgent{
		provider: provider,
		cwd:      cwd,
		model:    model,
	}
}

func getCodeStandards() string {
	return `
## Code Standards Summary

### Naming (Clean Code principles)
- Use intention-revealing names (no "doWork", "data", "temp")
- Functions: verbs (calculate_total, validate_order)
- Classes/Modules: nouns (InvoiceCalculator, UserRepository)
- Avoid "Manager", "Helper", "Utils" - too vague
- Be searchable and pronounceable

### Language Best Practices
- Small functions, one responsibility each
- Handle errors explicitly
- No global state or hidden side-effects
- Language-specific idioms (e.g., Go: interfaces at consumer, Rust: Result/Option)

### TDD & Testing
- Tests drive design, implementation follows
- Cover happy path AND edge cases (empty inputs, invalid data, errors)
- Keep tests focused and well-named

### Edge Cases to Check
- Empty/nil/null inputs
- Invalid types or out-of-range values
- External failures (network, IO, DB)
`
}

func buildReviewPrompt() string {
	prompt := `You are a senior code reviewer. Analyze code and provide constructive critique.

Focus on:
- Bugs and potential runtime errors
- Security vulnerabilities
- Performance bottlenecks
- Code style and readability (idiomatic usage)
- Maintainability

You can:
- List files to understand structure
- Read specific files to analyze details
- Use project_map to get a high-level view

Output Format:
Provide a structured review with:
1. **Summary**: High-level assessment.
2. **Critical Issues**: Must-fix bugs or security risks.
3. **Suggestions**: Improvements for quality/performance.
4. **Nitpicks**: Style/naming preferences.

Be concise but thorough. If the code is good, say so.`

	prompt += "\n" + getCodeStandards()

	return prompt
}

func (r *ReviewAgent) Execute(ctx context.Context, history []llm.ChatMessage, statusCallback StatusCallback) (string, error) {
	reviewPrompt := buildReviewPrompt()

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
				"name":        "list_files",
				"description": "List files in directory",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Glob pattern (e.g., *.go)",
						},
					},
				},
			},
		},
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "project_map",
				"description": "Get project structure",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"max_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Max depth",
						},
					},
				},
			},
		},
	}

	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}
	messages := make([]llm.ChatMessage, len(history[start:]))
	copy(messages, history[start:])

	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("Review: Analyzing (Iteration %d/%d)...", i+1, maxIterations))
		}
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[REVIEW] Iteration %d/%d\n", i+1, maxIterations)
		}

		resp, err := r.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: reviewPrompt,
			Messages:     messages,
			Tools:        toolDefs,
			Model:        r.model,
		})
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Text, nil
		}

		messages = append(messages, llm.ChatMessage{
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
			if statusCallback != nil {
				statusCallback(fmt.Sprintf("Review: Executing %s...", tc.Name))
			}
			result := tools.ExecuteToolFromLLM(llmCall, r.cwd)

			content := result.Result
			if result.Error != "" {
				content = "Error: " + result.Error
			} else if content == "" {
				content = "No output"
			}

			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				Content:    content,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	if statusCallback != nil {
		statusCallback("Review: Finalizing...")
	}
	finalResp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: reviewPrompt + "\n\nProvide your final review based on all the information gathered. Summarize findings.",
		Messages:     messages,
		Model:        r.model,
	})
	if err != nil {
		return "Review completed but failed to generate final summary", nil
	}
	return finalResp.Text, nil
}
