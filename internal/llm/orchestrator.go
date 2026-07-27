package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/tools"
)

type OrchestratorProvider struct {
	compound       *ChatCompletionProvider
	customExecutor Provider
	customModel    string
}

func NewOrchestrator(baseURL, backendName string, customExecutor Provider, customModel string) *OrchestratorProvider {
	return &OrchestratorProvider{
		compound:       NewChatCompletion(baseURL, backendName),
		customExecutor: customExecutor,
		customModel:    customModel,
	}
}

var compoundBuiltInTools = map[string]bool{
	"web_search":         true,
	"code_interpreter":   true,
	"visit_website":      true,
	"browser_automation": true,
	"wolfram_alpha":      true,
}

func (o *OrchestratorProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "\n[ORCHESTRATOR] Starting autonomous execution loop\n")
	}

	// Extract intent from request for intent-aware loop detection
	intent := "edit" // Default to edit
	if req.Intent != "" {
		intent = req.Intent
	}

	// Initialize Claude Code-style loop detector
	loopDetector := NewLoopDetector(intent)

	conversation := append([]ChatMessage{}, req.Messages...)

	if req.UserPrompt != "" {
		conversation = append(conversation, ChatMessage{
			Role:    "user",
			Content: req.UserPrompt,
		})
	}

	for {
		// Check if we should continue (intent-aware limits)
		shouldContinue, reason := loopDetector.ShouldContinue()
		if !shouldContinue {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] Stopping: %s\n", reason)
			}
			return &ChatResponse{
				Text: fmt.Sprintf("Task stopped: %s. Stats: %s", reason, loopDetector.GetStats()),
			}, nil
		}

		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] Iteration %d (%s)\n", loopDetector.Iteration, loopDetector.GetStats())
		}

		executorReq := ChatRequest{
			SystemPrompt: req.SystemPrompt,
			Model:        o.customModel,
			Messages:     conversation,
			Tools:        req.Tools,
		}

		resp, err := o.customExecutor.Chat(ctx, executorReq)
		if err != nil {
			return nil, err
		}

		if len(resp.ToolCalls) == 0 && resp.Text != "" {
			parsedCalls := ParseToolCallsFromText(resp.Text)
			if len(parsedCalls) > 0 {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] Parsed %d tool calls from text\n", len(parsedCalls))
				}
				resp.ToolCalls = parsedCalls
			}
		}

		if len(resp.ToolCalls) == 0 {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] No more tools to call, returning final response\n")
			}
			return &ChatResponse{
				Text: resp.Text,
			}, nil
		}

		conversation = append(conversation, ChatMessage{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			// Claude Code-style tool loop detection
			isLoop, loopReason := loopDetector.RecordToolCall(tc.Name, tc.Arguments)
			if isLoop {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] %s\n", loopReason)
				}

				// Return last tool result if available
				for i := len(conversation) - 1; i >= 0; i-- {
					if conversation[i].Role == "tool" && conversation[i].Name == tc.Name {
						return &ChatResponse{
							Text: fmt.Sprintf("Loop detected: %s. Last result: %s", loopReason, conversation[i].Content),
						}, nil
					}
				}

				return &ChatResponse{
					Text: fmt.Sprintf("Loop detected: %s", loopReason),
				}, nil
			}

			var toolResult string

			if compoundBuiltInTools[tc.Name] {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] Executing Compound tool: %s\n", tc.Name)
				}

				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err == nil {
					prompt := tc.Arguments
					if query, ok := argsMap["query"].(string); ok {
						prompt = query
					} else if q, ok := argsMap["q"].(string); ok {
						prompt = q
					}

					compoundResp, compoundErr := o.compound.Chat(ctx, ChatRequest{
						Model:        "groq/compound",
						SystemPrompt: req.SystemPrompt,
						UserPrompt:   prompt,
					})

					if compoundErr != nil {
						toolResult = fmt.Sprintf("Error: %v", compoundErr)
					} else {
						toolResult = compoundResp.Text
					}
				} else {
					toolResult = "Error: invalid arguments"
				}
			} else {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] Executing custom tool: %s\n", tc.Name)
				}

				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err != nil {
					toolResult = fmt.Sprintf("Error parsing arguments: %v", err)
				} else {
					cwd, _ := os.Getwd()
					toolCall := tools.ToolCall{
						Name:      tc.Name,
						Arguments: argsMap,
					}
					result := tools.ExecuteTool(toolCall, cwd)
					if result.Error != "" {
						toolResult = fmt.Sprintf("Error: %s", result.Error)
					} else {
						toolResult = result.Result
						if toolResult == "" {
							toolResult = "Success"
						}
					}
				}
			}

			conversation = append(conversation, ChatMessage{
				Role:       "tool",
				Content:    toolResult,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			})

			// Track file modifications for progress detection
			if tc.Name == "write_file" || tc.Name == "patch_file" || tc.Name == "create_file" {
				loopDetector.RecordFileModification()
			}
			if tc.Name == "read_file" || tc.Name == "list_dir" || tc.Name == "grep" {
				loopDetector.RecordReadOperation()
			}
		}

		// Check for content loops (repeated responses)
		if isLoop, reason := loopDetector.RecordResponse(resp.Text); isLoop {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[ORCHESTRATOR] %s\n", reason)
			}
			return &ChatResponse{
				Text: fmt.Sprintf("Content loop detected: %s", reason),
			}, nil
		}
	}
}

func (o *OrchestratorProvider) ChatStream(ctx context.Context, req ChatRequest, callback func(chunk string)) error {
	resp, err := o.Chat(ctx, req)
	if err != nil {
		return err
	}
	callback(resp.Text)
	return nil
}
