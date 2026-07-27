package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/tools"
)

type AnalyzerAgent struct {
	provider llm.Provider
	cwd      string
	model    string
}

func NewAnalyzer(provider llm.Provider, cwd string, model string) *AnalyzerAgent {
	return &AnalyzerAgent{
		provider: provider,
		cwd:      cwd,
		model:    model,
	}
}

const analyzerPrompt = `You are a code analyzer. Your ONLY job is to understand existing code.

WORKFLOW:
1. Use project_map to see structure
2. Use read_file to read relevant files
3. Summarize what you found

CRITICAL RULES:
- Do NOT suggest changes
- Do NOT create plans
- ONLY analyze and report what exists
- Be concise and factual

EXAMPLE 1 - Analyzing Go authentication code:
Task: "How does user authentication work?"
Analysis:
  Files found:
    - auth/handler.go (Login, Logout functions)
    - middleware/jwt.go (JWT verification)
  
  Dependencies:
    middleware/jwt.go imports auth/handler.go
  
  Summary:  
    - Login handler generates JWT tokens
    - Middleware verifies tokens on protected routes
    - Uses bcrypt for password hashing

EXAMPLE 2 - Finding relevant files for feature:
Task: "add user registration"
Analysis:
  Existing auth structure:
    - auth/handler.go (login/logout)
    - models/user.go (User struct)
    - middleware/auth.go (auth middleware)
  
  For registration, will need:
    - Modify: auth/handler.go (add Register function)
    - Modify: models/user.go (validation methods)
    - Possibly create: migrations/ (if DB schema changes)

Focus on understanding the codebase.`

func (a *AnalyzerAgent) Analyze(ctx context.Context, task string, statusCallback StatusCallback) (string, error) {
	if statusCallback != nil {
		statusCallback("Analyzer: Understanding codebase...")
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

	analyzePrompt := fmt.Sprintf(`Analyze the codebase for this task:

Task: %s

Your job:
1. Understand the project structure
2. Find relevant files
3. Read key files to understand context
4. Summarize what you found (keep it brief)

Do NOT suggest changes. Just report what exists.`, task)

	history := []llm.ChatMessage{
		{Role: "user", Content: analyzePrompt},
	}

	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[ANALYZER] Iteration %d/%d\n", i+1, maxIterations)
		}

		resp, err := a.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: analyzerPrompt,
			Messages:     history,
			Tools:        toolDefs,
			Model:        a.model,
		})
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[ANALYZER] Analysis complete\n")
			}
			return resp.Text, nil
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
			if statusCallback != nil {
				var argsMap map[string]interface{}
				if json.Unmarshal([]byte(tc.Arguments), &argsMap) == nil {
					if path, ok := argsMap["path"].(string); ok {
						statusCallback(fmt.Sprintf("Analyzer: Reading %s...", path))
					}
				}
			}
			result := tools.ExecuteToolFromLLM(llmCall, a.cwd)

			content := result.Result
			if result.Error != "" {
				content = "Error: " + result.Error
			}
			if content == "" {
				content = "Success"
			}

			history = append(history, llm.ChatMessage{
				Role:       "tool",
				Content:    content,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	return "Analyzer reached max iterations", nil
}
