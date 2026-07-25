package modes

import (
	"strings"
	"testing"
)

func TestBuildCodebaseResearchPromptRequiresGroundedEvidence(t *testing.T) {
	prompt := buildCodebaseResearchPrompt(
		"Explain session expiration and concurrent access",
		repositoryEvidence{
			Language: "Go",
			Files:    []string{"README.md", "go.mod", "session/store.go", "session/store_test.go"},
			Contents: map[string]string{
				"session/store.go": "func (s *Store) Active(token string, now time.Time) bool",
			},
		},
	)

	for _, required := range []string{
		"Main language detected deterministically: Go",
		"session/store.go",
		"session/store_test.go",
		"func (s *Store) Active",
		"read_file",
		"exact file paths",
		"Do not speculate",
		"verification command",
		"Contracts and tests describe expectations",
		"report contradictions",
		"Concurrency claims require synchronization in the implementation",
		"WaitGroup or goroutines in a test exercise concurrency",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("research prompt missing %q:\n%s", required, prompt)
		}
	}
}
