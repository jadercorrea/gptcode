//go:build e2e

package run_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadercorrea/gptcode/internal/changelog"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
)

func TestDocumentationUpdates(t *testing.T) {
	t.Run("update README with new features", func(t *testing.T) {
		t.Skip("TODO: Implement - Reflect new features/changes in README")
	})

	t.Run("generate CHANGELOG", func(t *testing.T) {
		if os.Getenv("SKIP_E2E_LLM") != "" {
			t.Skip("Skipping LLM-dependent E2E test")
		}

		tmpDir := t.TempDir()

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to init git: %v", err)
		}

		// Configure git
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()

		// Create some commits with conventional format
		commits := []struct {
			file    string
			content string
			message string
		}{
			{"feat1.go", "package main", "feat: Add user authentication"},
			{"feat2.go", "package main", "feat(api): Add rate limiting"},
			{"fix1.go", "package main", "fix: Resolve memory leak in cache"},
			{"docs.md", "# Docs", "docs: Update API documentation"},
		}

		for _, c := range commits {
			filePath := filepath.Join(tmpDir, c.file)
			if err := os.WriteFile(filePath, []byte(c.content), 0644); err != nil {
				t.Fatalf("Failed to create %s: %v", c.file, err)
			}
			exec.Command("git", "-C", tmpDir, "add", c.file).Run()
			cmd := exec.Command("git", "-C", tmpDir, "commit", "-m", c.message)
			if err := cmd.Run(); err != nil {
				t.Fatalf("Failed to commit %s: %v", c.file, err)
			}
		}

		// Load config and create generator
		setup, err := config.LoadSetup()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		backendName := setup.Defaults.Backend
		if backendName == "" {
			backendName = "anthropic"
		}
		backendCfg := setup.Backend[backendName]

		var provider llm.Provider
		if backendCfg.Type == "ollama" {
			provider = llm.NewOllama(backendCfg.BaseURL)
		} else {
			provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}

		queryModel := backendCfg.GetModelForAgent("query")
		if queryModel == "" {
			queryModel = backendCfg.DefaultModel
		}

		generator := changelog.NewChangelogGenerator(provider, queryModel, tmpDir)

		// Generate changelog
		t.Log("Generating CHANGELOG...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		entry, err := generator.Generate(ctx, "", "HEAD")
		if err != nil {
			t.Fatalf("Failed to generate changelog: %v", err)
		}

		if entry == "" {
			t.Fatal("Generated changelog is empty")
		}

		t.Logf("Generated changelog (%d bytes)", len(entry))

		// Verify basic structure
		if !strings.Contains(entry, "##") {
			t.Error("Changelog missing version header")
		}
		if !strings.Contains(entry, "Features") {
			t.Error("Changelog missing Features section")
		}
		if !strings.Contains(entry, "Bug Fixes") {
			t.Error("Changelog missing Bug Fixes section")
		}
		if !strings.Contains(entry, "authentication") || !strings.Contains(entry, "rate limiting") {
			t.Error("Changelog missing feature content")
		}
		if !strings.Contains(entry, "memory leak") {
			t.Error("Changelog missing fix content")
		}

		t.Log("✓ CHANGELOG generated successfully")
	})

	t.Run("update API documentation", func(t *testing.T) {
		t.Skip("TODO: Implement - Reflect changed endpoints/types")
	})
}
