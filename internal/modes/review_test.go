package modes

import (
	"strings"
	"testing"
)

func TestBuildReviewPromptIncludesFileEvidenceAndConstraints(t *testing.T) {
	prompt := buildReviewPrompt(
		"/repo/session/store.go",
		false,
		"concurrency correctness and public API stability",
		"package session\n\ntype Store struct { sessions map[string]Session }\n",
	)

	for _, required := range []string{
		"session/store.go",
		"type Store struct",
		"public API stability",
		"Do not recommend changes that violate",
		"file:line",
		"Do not label an absent feature as a defect",
		"Do not invent lifecycle requirements",
		"Treat an evidenced violation of the requested focus as a defect",
		"Private implementation changes do not alter the public API",
		"1 | package session",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("review prompt missing %q:\n%s", required, prompt)
		}
	}
}
