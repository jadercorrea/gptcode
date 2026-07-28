package agents

import (
	"context"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/tools"
)

type QueryAgent struct {
	provider llm.Provider
	cwd      string
	model    string
}

func NewQuery(provider llm.Provider, cwd string, model string) *QueryAgent {
	return &QueryAgent{
		provider: provider,
		cwd:      cwd,
		model:    model,
	}
}

const queryPrompt = `You are a code reader and explainer. Your job is to READ and UNDERSTAND code.

You can:
- List files in directories
- Read file contents
- Search for patterns
- Explain code structure

You CANNOT modify files. Be concise and direct in your explanations.`

func (q *QueryAgent) Execute(ctx context.Context, history []llm.ChatMessage, statusCallback StatusCallback) (string, error) {
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
				"name":        "search_code",
				"description": "Search for pattern in code",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Search pattern",
						},
						"file_pattern": map[string]interface{}{
							"type":        "string",
							"description": "File pattern filter",
						},
					},
					"required": []string{"pattern"},
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

	// Copy history
	messages := make([]llm.ChatMessage, len(history))
	copy(messages, history)

	maxIterations := 3
	for i := 0; i < maxIterations; i++ {
		if statusCallback != nil {
			statusCallback(fmt.Sprintf("Query: Thinking (Iteration %d/%d)...", i+1, maxIterations))
		}
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[QUERY] Iteration %d/%d\n", i+1, maxIterations)
		}

		resp, err := q.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: queryPrompt,
			Messages:     messages,
			Tools:        toolDefs,
			Model:        q.model,
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
				statusCallback(fmt.Sprintf("Query: Executing %s...", tc.Name))
			}
			result := tools.ExecuteToolFromLLMContext(ctx, llmCall, q.cwd)

			content := result.Result
			if result.Error != "" {
				content = "Error: " + result.Error
			}
			if content == "" {
				content = "Success"
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
		statusCallback("Query: Generating response...")
	}

	finalMessages := make([]llm.ChatMessage, len(messages))
	copy(finalMessages, messages)
	for i := range finalMessages {
		if finalMessages[i].Role == "assistant" && len(finalMessages[i].ToolCalls) > 0 {
			finalMessages[i].ToolCalls = nil
			finalMessages[i].Content = ""
		}
	}

	finalResp, err := q.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "Based on the tool execution results above, provide a clear and concise answer to the user's question. Answer directly without suggesting additional actions.",
		Messages:     finalMessages,
		Model:        q.model,
	})
	if err != nil {
		return "", err
	}

	return finalResp.Text, nil
}
