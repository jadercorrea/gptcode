package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jadercorrea/gptcode/internal/config"
)

func TestSelectDoModelsPrefersConfiguredAgentModels(t *testing.T) {
	setup := &config.Setup{
		Backend: map[string]config.BackendConfig{
			"openrouter": {
				DefaultModel: "google/gemini-2.5-pro",
				AgentModels: config.AgentModels{
					Editor: "anthropic/claude-sonnet-4",
					Query:  "google/gemini-2.5-flash",
				},
			},
		},
	}
	setup.Defaults.Backend = "openrouter"

	selection, err := selectDoModels(setup)
	if err != nil {
		t.Fatal(err)
	}
	if selection.editorModel != "anthropic/claude-sonnet-4" {
		t.Fatalf("editor model = %q, want configured model", selection.editorModel)
	}
	if selection.queryModel != "google/gemini-2.5-flash" {
		t.Fatalf("query model = %q, want configured model", selection.queryModel)
	}
}

func TestExecutionLanguagePrefersRepositoryOverGlobalDefault(t *testing.T) {
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "mix.exs"), []byte("defmodule Demo.MixProject do\nend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := executionLanguage(repository, "go"); got != "elixir" {
		t.Fatalf("execution language = %q, want repository language elixir", got)
	}
}

func TestRetryPromptRequiresExplicitInteractiveMode(t *testing.T) {
	if shouldPromptForRetry(false) {
		t.Fatal("non-interactive execution must not prompt for retry input")
	}
}
