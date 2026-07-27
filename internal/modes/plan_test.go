package modes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePlanEvidenceRejectsInventedFilesAndMakeTargets(t *testing.T) {
	repository := t.TempDir()
	writeTestFile(t, filepath.Join(repository, "Makefile"), "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n")
	writeTestFile(t, filepath.Join(repository, "internal", "tools", "tools.go"), "package tools\n")

	plan := `
## Changes Required
**File**: internal/tool/file.go
**New file**: internal/tools/path_boundary_test.go

## Automated Verification
- [ ] Tests pass: make test
- [ ] Linting passes: make lint
`

	problems := validatePlanEvidence(plan, repository)
	if len(problems) != 2 {
		t.Fatalf("expected two evidence problems, got %#v", problems)
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "internal/tool/file.go") {
		t.Fatalf("missing invented-file problem: %s", joined)
	}
	if !strings.Contains(joined, "make lint") {
		t.Fatalf("missing invented-target problem: %s", joined)
	}
}

func TestValidatePlanEvidenceAcceptsExistingAndExplicitNewFiles(t *testing.T) {
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "Makefile"), []byte("verify:\n\tgo test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(repository, "internal", "tools.go"), "package internal\n")

	plan := `
**Existing file**: internal/tools.go
**New file**: internal/tools_test.go
- [ ] Run: make verify
`

	if problems := validatePlanEvidence(plan, repository); len(problems) != 0 {
		t.Fatalf("expected grounded plan, got %#v", problems)
	}
}
