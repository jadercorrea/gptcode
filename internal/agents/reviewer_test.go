package agents

import (
	"context"
	"testing"

	"github.com/jadercorrea/gptcode/internal/llm"
)

func TestReviewerForcesFinalVerdictAfterToolRounds(t *testing.T) {
	toolResponse := llm.ChatResponse{ToolCalls: []llm.ChatToolCall{{
		ID: "read", Name: "read_file", Arguments: `{"path":"counter.go"}`,
	}}}
	provider := &mockProvider{responses: []llm.ChatResponse{
		toolResponse, toolResponse, toolResponse, {Text: "SUCCESS\nAll requirements are met."},
	}}
	result, err := NewReviewer(provider, t.TempDir(), "local").Review(
		context.Background(), "Tests pass", []string{"counter.go"}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected final synthesis to succeed, got %#v", result)
	}
	if provider.callCount != 4 {
		t.Fatalf("expected three tool rounds and one synthesis, got %d calls", provider.callCount)
	}
}

func TestExplicitReviewVerdictDoesNotMistakeCriterionForFailure(t *testing.T) {
	if verdict := explicitReviewVerdict("2. go test ./... runs without errors (tests pass)"); verdict != "" {
		t.Fatalf("expected ambiguous text to have no verdict, got %q", verdict)
	}
}
