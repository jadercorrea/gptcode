package agents

import (
	"context"
	"testing"

	"github.com/jadercorrea/gptcode/internal/llm"
)

func TestQueryRetriesAnEmptyAnswer(t *testing.T) {
	provider := &mockProvider{responses: []llm.ChatResponse{
		{Text: ""},
		{Text: "The cache uses an RWMutex and verifies behavior with go test -race ./..."},
	}}
	query := NewQuery(provider, t.TempDir(), "test-model")

	result, err := query.Execute(context.Background(), []llm.ChatMessage{{
		Role:    "user",
		Content: "Explain cache concurrency.",
	}}, nil)
	if err != nil {
		t.Fatalf("expected empty answer recovery: %v", err)
	}
	if result == "" {
		t.Fatal("expected a grounded non-empty answer")
	}
	if provider.callCount != 2 {
		t.Fatalf("expected one retry after the empty answer, got %d calls", provider.callCount)
	}
}
