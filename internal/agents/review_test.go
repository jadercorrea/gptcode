package agents

import (
	"context"
	"testing"

	"github.com/jadercorrea/gptcode/internal/llm"
)

func TestReviewAgent(t *testing.T) {
	t.Run("simple review without tools", func(t *testing.T) {
		mock := &mockProvider{
			responses: []llm.ChatResponse{
				{Text: "Code looks good."},
			},
		}

		agent := NewReview(mock, ".", "test-model")
		result, err := agent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: "review main.go"}}, nil)

		if err != nil {
			t.Fatalf("ReviewAgent failed: %v", err)
		}
		if result != "Code looks good." {
			t.Errorf("Expected 'Code looks good.', got '%s'", result)
		}
	})

	t.Run("review with tool calls", func(t *testing.T) {
		mock := &mockProvider{
			responses: []llm.ChatResponse{
				{
					Text: "Need to read file",
					ToolCalls: []llm.ChatToolCall{
						{ID: "1", Name: "read_file", Arguments: `{"path": "test.go"}`},
					},
				},
				{Text: "File analyzed. Found issues."},
			},
		}

		agent := NewReview(mock, ".", "test-model")
		result, err := agent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: "review test.go"}}, nil)

		if err != nil {
			t.Fatalf("ReviewAgent with tools failed: %v", err)
		}
		if result == "" {
			t.Error("Expected non-empty result")
		}
	})

	t.Run("with status callback", func(t *testing.T) {
		mock := &mockProvider{
			responses: []llm.ChatResponse{
				{Text: "Analysis complete."},
			},
		}

		var statusUpdates []string
		callback := func(status string) {
			statusUpdates = append(statusUpdates, status)
		}

		agent := NewReview(mock, ".", "test-model")
		_, err := agent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: "review"}}, callback)

		if err != nil {
			t.Fatalf("ReviewAgent with callback failed: %v", err)
		}
		if len(statusUpdates) == 0 {
			t.Error("Expected status updates but got none")
		}
	})
}
