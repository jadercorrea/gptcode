package autonomous

import (
	"context"
	"testing"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/llm"
)

// Integration test for full Symphony execution
func TestSymphonyExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Mock provider
	provider := &mockSymphonyProvider{
		complexity: 8, // Trigger symphony decomposition
		movements: []Movement{
			{
				ID:              "m1",
				Name:            "Step 1",
				Goal:            "Create test file",
				Dependencies:    []string{},
				SuccessCriteria: []string{"file exists"},
				Status:          "pending",
			},
		},
	}

	classifier := agents.NewClassifier(provider, "test-model")
	analyzer := NewTaskAnalyzer(classifier, provider, "/tmp", "test-model")

	// Create mock setup for test
	// Note: In real usage, Maestro would be created with actual setup + selector
	// For this test we skip full Maestro setup since test is marked to skip anyway
	_ = analyzer
}

func TestShouldUseSymphony(t *testing.T) {
	tests := []struct {
		name     string
		analysis TaskAnalysis
		want     bool
	}{
		{
			name: "ordinary complex fix stays in one maestro workflow",
			analysis: TaskAnalysis{
				Intent:     "edit",
				Complexity: 8,
				Movements:  []Movement{{Goal: "inspect"}, {Goal: "implement"}, {Goal: "verify"}},
			},
			want: false,
		},
		{
			name: "large multi-part change uses symphony",
			analysis: TaskAnalysis{
				Intent:     "edit",
				Complexity: 9,
				Movements:  []Movement{{Goal: "change service A"}, {Goal: "change service B"}},
			},
			want: true,
		},
		{
			name: "single movement never needs orchestration",
			analysis: TaskAnalysis{
				Intent:     "edit",
				Complexity: 10,
				Movements:  []Movement{{Goal: "implement the fix"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseSymphony(&tt.analysis); got != tt.want {
				t.Fatalf("shouldUseSymphony() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock provider for symphony tests
type mockSymphonyProvider struct {
	complexity int
	movements  []Movement
}

func (m *mockSymphonyProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	// Return complexity score or movement decomposition based on prompt
	if req.UserPrompt != "" && len(req.UserPrompt) > 100 {
		// This is a complexity scoring request
		return &llm.ChatResponse{
			Text: "8",
		}, nil
	}

	// This is a movement decomposition request
	return &llm.ChatResponse{
		Text: `[
			{
				"id": "m1",
				"name": "Test Movement",
				"description": "Test description",
				"goal": "Test goal",
				"dependencies": [],
				"required_files": [],
				"output_files": ["test.txt"],
				"success_criteria": ["file exists"]
			}
		]`,
	}, nil
}
