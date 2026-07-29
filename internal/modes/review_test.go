package modes

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReviewPromptIncludesFileEvidenceAndConstraints(t *testing.T) {
	prompt := buildReviewPrompt(
		"/repo/session/store.go",
		false,
		"concurrency correctness and public API stability",
		"package session\n\ntype Store struct { sessions map[string]Session }\n",
		"<file path=\"task.md\">\n1 | Preserve the public API.\n</file>",
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
		`path="task.md"`,
		"Preserve the public API",
		"Reserve Critical Issues",
		"Go visibility is determined by capitalization",
		"exclusive lock prevents concurrent map mutation",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("review prompt missing %q:\n%s", required, prompt)
		}
	}
}

func TestFormatRelatedReviewEvidenceExcludesTargetAndNumbersContracts(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "cache.go")
	formatted := formatRelatedReviewEvidence(root, target, repositoryEvidence{
		Contents: map[string]string{
			"cache.go":      "package cache\n",
			"cache_test.go": "package cache\nfunc TestExpiry() {}\n",
			"task.md":       "Run go test -race ./...\n",
		},
	})

	if strings.Contains(formatted, `path="cache.go"`) {
		t.Fatalf("target was duplicated in related evidence: %s", formatted)
	}
	for _, required := range []string{
		`path="cache_test.go"`,
		"2 | func TestExpiry",
		`path="task.md"`,
		"1 | Run go test -race ./...",
	} {
		if !strings.Contains(formatted, required) {
			t.Fatalf("related evidence missing %q: %s", required, formatted)
		}
	}
}
