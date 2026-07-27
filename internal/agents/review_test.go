package agents

import (
	"context"
	"strings"
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

	t.Run("rejects an empty synthesis after exhausting tool iterations", func(t *testing.T) {
		responses := make([]llm.ChatResponse, 0, 6)
		for i := 0; i < 5; i++ {
			responses = append(responses, llm.ChatResponse{
				ToolCalls: []llm.ChatToolCall{{
					ID:        string(rune('1' + i)),
					Name:      "project_map",
					Arguments: `{"max_depth": 1}`,
				}},
			})
		}
		responses = append(responses, llm.ChatResponse{Text: " \n\t"})

		mock := &mockProvider{responses: responses}
		agent := NewReview(mock, t.TempDir(), "test-model")

		result, err := agent.Execute(
			context.Background(),
			[]llm.ChatMessage{{Role: "user", Content: "review this directory"}},
			nil,
		)

		if err == nil {
			t.Fatal("expected empty final synthesis to return an error")
		}
		if result != "" {
			t.Fatalf("expected no successful result, got %q", result)
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Fatalf("expected actionable empty-response error, got %v", err)
		}
		if mock.callCount != 6 {
			t.Fatalf("expected five tool iterations and one synthesis call, got %d calls", mock.callCount)
		}
	})
}
