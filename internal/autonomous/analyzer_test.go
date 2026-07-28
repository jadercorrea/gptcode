package autonomous

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/llm"
)

func TestRepositoryEvidenceReadsOnlyFilesNamedByTask(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "counter.go"), []byte("func Add(a, b int) int { return a - b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("do not include"), 0600); err != nil {
		t.Fatal(err)
	}
	evidence := repositoryEvidence(root, []string{"counter.go"}, 4096)
	if !strings.Contains(evidence, "return a - b") {
		t.Fatalf("expected source evidence, got %q", evidence)
	}
	if strings.Contains(evidence, "do not include") {
		t.Fatal("included a file that was not named by the task")
	}
}

func TestRepositoryEvidenceRejectsSymlinkOutsideRepository(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("outside secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if evidence := repositoryEvidence(root, []string{"linked.go"}, 4096); strings.Contains(evidence, "outside secret") {
		t.Fatal("repository evidence followed a symlink outside the repository")
	}
}

func TestExtractVerb(t *testing.T) {
	tests := []struct {
		task     string
		expected string
	}{
		{"create summary.md with overview", "create"},
		{"remove TODO from main.go", "remove"},
		{"refactor authentication system", "refactor"},
		{"reorganize all docs files", "reorganize"},
		{"add error handling", "add"},
		{"something unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			result := extractVerb(tt.task)
			if result != tt.expected {
				t.Errorf("extractVerb(%q) = %q, want %q", tt.task, result, tt.expected)
			}
		})
	}
}

func TestExtractFileMentions(t *testing.T) {
	tests := []struct {
		task     string
		expected []string
	}{
		{
			"read docs/_posts/2025-11-28-file.md and create summary",
			[]string{"docs/_posts/2025-11-28-file.md"},
		},
		{
			"modify main.go and test.go",
			[]string{"main.go", "test.go"},
		},
		{
			"create new file without mention",
			[]string{},
		},
		{
			"update config.yml and app.json",
			[]string{"config.yml", "app.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			result := extractFileMentions(tt.task)
			if len(result) != len(tt.expected) {
				t.Errorf("extractFileMentions(%q) returned %d files, want %d", tt.task, len(result), len(tt.expected))
				return
			}
			for i, file := range result {
				if file != tt.expected[i] {
					t.Errorf("extractFileMentions(%q)[%d] = %q, want %q", tt.task, i, file, tt.expected[i])
				}
			}
		})
	}
}

// Mock provider for testing
type mockProvider struct {
	response string
}

func (m *mockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Text: m.response,
	}, nil
}

func TestEstimateComplexity(t *testing.T) {
	tests := []struct {
		name     string
		task     string
		llmResp  string
		expected int
	}{
		{
			name:     "simple task",
			task:     "create hello.md",
			llmResp:  "2",
			expected: 3, // ML retrained with expanded dataset
		},
		{
			name:     "complex task",
			task:     "reorganize all docs",
			llmResp:  "8",
			expected: 8,
		},
		{
			name:     "response with text",
			task:     "some task",
			llmResp:  "The complexity is 5 out of 10",
			expected: 8, // ML retrained with expanded dataset
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{response: tt.llmResp}
			classifier := agents.NewClassifier(provider, "test-model")
			analyzer := NewTaskAnalyzer(classifier, provider, "/tmp", "test-model")

			result, err := analyzer.estimateComplexity(context.Background(), tt.task)
			if err != nil {
				t.Fatalf("estimateComplexity() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("estimateComplexity(%q) = %d, want %d", tt.task, result, tt.expected)
			}
		})
	}
}
