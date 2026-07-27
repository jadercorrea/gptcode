package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/refactor"
)

func TestBreakingChangesDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "breaking-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := isolatedGitCommand(tmpDir, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git: %v", err)
	}

	apiFile := filepath.Join(tmpDir, "api.go")
	originalContent := `package main

func ProcessRequest(data string) string {
	return "processed: " + data
}
`
	if err := os.WriteFile(apiFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write api.go: %v", err)
	}

	cmd = isolatedGitCommand(tmpDir, "add", "api.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	if err := isolatedGitCommand(tmpDir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatalf("configure git user name: %v", err)
	}
	if err := isolatedGitCommand(tmpDir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("configure git user email: %v", err)
	}

	cmd = isolatedGitCommand(tmpDir, "commit", "-m", "initial")
	if err := cmd.Run(); err != nil {
		t.Skipf("git commit failed (expected in test env): %v", err)
	}

	modifiedContent := `package main

import "context"

func ProcessRequest(ctx context.Context, data string) (string, error) {
	return "processed: " + data, nil
}
`
	if err := os.WriteFile(apiFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify api.go: %v", err)
	}

	consumerFile := filepath.Join(tmpDir, "consumer.go")
	consumerContent := `package main

func useAPI() {
	result := ProcessRequest("test")
	println(result)
}
`
	if err := os.WriteFile(consumerFile, []byte(consumerContent), 0644); err != nil {
		t.Fatalf("failed to write consumer.go: %v", err)
	}

	provider := &agents.MockProvider{
		Response: `# Migration Plan

## Strategy: Immediate

### Updates Required
1. Add context.Context parameter
2. Handle error return value

### Code Pattern
\x60\x60\x60go
result, err := ProcessRequest(ctx, data)
if err != nil {
    // handle error
}
\x60\x60\x60`,
	}

	coordinator := refactor.NewBreakingCoordinator(provider, "test-model", tmpDir)

	result, err := coordinator.DetectAndCoordinate(context.Background())
	if err != nil {
		t.Fatalf("DetectAndCoordinate failed: %v", err)
	}

	if len(result.Changes) == 0 {
		t.Error("expected at least one breaking change")
	}

	found := false
	for _, change := range result.Changes {
		if change.Symbol == "ProcessRequest" && change.Type == "signature_changed" {
			found = true
			if !strings.Contains(change.OldAPI, "func ProcessRequest(data string) string") {
				t.Errorf("unexpected old API: %s", change.OldAPI)
			}
			if !strings.Contains(change.NewAPI, "context.Context") {
				t.Errorf("new API should contain context.Context: %s", change.NewAPI)
			}
		}
	}

	if !found {
		t.Error("ProcessRequest signature change not detected")
	}

	if result.MigrationPlan == "" {
		t.Error("expected migration plan to be generated")
	}

	if !strings.Contains(result.MigrationPlan, "Migration Plan") {
		t.Errorf("migration plan format unexpected: %s", result.MigrationPlan)
	}
}

func isolatedGitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = make([]string, 0, len(os.Environ()))
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "GIT_INDEX_FILE=") ||
			strings.HasPrefix(variable, "GIT_DIR=") ||
			strings.HasPrefix(variable, "GIT_WORK_TREE=") {
			continue
		}
		cmd.Env = append(cmd.Env, variable)
	}
	return cmd
}
