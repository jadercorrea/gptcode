package main

import (
	"os"
	"strings"

	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gptcode_lang_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	tests := []struct {
		filename string
		expected string
	}{
		{"go.mod", "go"},
		{"package.json", "typescript"},
		{"mix.exs", "elixir"},
		{"Gemfile", "ruby"},
		{"requirements.txt", "python"},
		{"Cargo.toml", "rust"},
		{"unknown.txt", "unknown"},
	}

	for _, tt := range tests {
		if tt.filename != "unknown.txt" {
			os.WriteFile(tt.filename, []byte(""), 0644)
		}

		got := detectLanguage()
		if got != tt.expected {
			t.Errorf("detectLanguage() with %s = %s, want %s", tt.filename, got, tt.expected)
		}

		if tt.filename != "unknown.txt" {
			os.Remove(tt.filename)
		}
	}
}

func TestResolveVersionUsesGoModuleBuildInfo(t *testing.T) {
	tests := []struct {
		name          string
		injected      string
		moduleVersion string
		want          string
	}{
		{
			name:          "go install module version",
			injected:      "dev",
			moduleVersion: "v0.0.105",
			want:          "v0.0.105",
		},
		{
			name:          "goreleaser injected version",
			injected:      "0.0.105",
			moduleVersion: "v0.0.105",
			want:          "0.0.105",
		},
		{
			name:          "local development",
			injected:      "dev",
			moduleVersion: "(devel)",
			want:          "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.injected, tt.moduleVersion); got != tt.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", tt.injected, tt.moduleVersion, got, tt.want)
			}
		})
	}
}

func TestRootHelpReflectsVerifiableWorkflowPositioning(t *testing.T) {
	if !strings.Contains(rootCmd.Long, "evidence, not just answers") {
		t.Fatal("root help must state the evidence-driven thesis")
	}
	for _, rejected := range []string{"$0-5/month", "COPILOT (Autonomous)"} {
		if strings.Contains(rootCmd.Long, rejected) {
			t.Fatalf("root help contains retired positioning %q", rejected)
		}
	}
}

func TestPublicCommandSurfaceHidesLegacyCommands(t *testing.T) {
	configurePublicCommandSurface()

	legacy := []string{
		"acp",
		"agent",
		"caveman",
		"cfg",
		"context",
		"coverage",
		"demo",
		"detect-language",
		"docs",
		"evolve",
		"feedback",
		"feature",
		"gen",
		"git",
		"go",
		"graph",
		"issue",
		"login",
		"logout",
		"ml",
		"mem",
		"merge",
		"monitor",
		"perf",
		"pr",
		"profiles",
		"refactor",
		"release",
		"security",
		"status",
		"test",
		"tdd",
		"training",
		"watch",
	}

	for _, name := range legacy {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil || command == rootCmd {
			continue
		}
		if !command.Hidden {
			t.Errorf("legacy command %q must be hidden from the maintained CLI surface", name)
		}
	}

	for _, name := range []string{
		"do",
		"implement",
		"plan",
		"research",
		"review",
		"run",
		"setup",
		"skills",
	} {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil || command == rootCmd {
			t.Errorf("maintained command %q is not registered", name)
			continue
		}
		if command.Hidden {
			t.Errorf("maintained command %q must remain visible", name)
		}
	}
}

func TestFeedbackCommandsRegistered(t *testing.T) {
	tests := []struct {
		name     string
		cmdPath  []string
		wantHelp string
	}{
		{
			name:     "feedback command exists",
			cmdPath:  []string{"feedback"},
			wantHelp: "Record and analyze feedback",
		},
		{
			name:     "feedback submit exists",
			cmdPath:  []string{"feedback", "submit"},
			wantHelp: "Submit feedback event",
		},
		{
			name:     "feedback stats exists",
			cmdPath:  []string{"feedback", "stats"},
			wantHelp: "View feedback statistics",
		},
		{
			name:     "feedback hook exists",
			cmdPath:  []string{"feedback", "hook"},
			wantHelp: "Install shell hooks",
		},
		{
			name:     "feedback hook install exists",
			cmdPath:  []string{"feedback", "hook", "install"},
			wantHelp: "Install shell hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := rootCmd
			for _, part := range tt.cmdPath {
				var found bool
				for _, subcmd := range cmd.Commands() {
					if subcmd.Name() == part {
						cmd = subcmd
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Command %v not found", tt.cmdPath)
				}
			}
			if !strings.Contains(cmd.Short, tt.wantHelp) {
				t.Errorf("Command help = %q, want to contain %q", cmd.Short, tt.wantHelp)
			}
		})
	}
}

func TestDemoCommandsRegistered(t *testing.T) {
	tests := []struct {
		name     string
		cmdPath  []string
		wantHelp string
	}{
		{
			name:     "demo command exists",
			cmdPath:  []string{"demo"},
			wantHelp: "Demos and recordings",
		},
		{
			name:     "demo feedback exists",
			cmdPath:  []string{"demo", "feedback"},
			wantHelp: "Feedback capture demos",
		},
		{
			name:     "demo feedback create exists",
			cmdPath:  []string{"demo", "feedback", "create"},
			wantHelp: "Generate feedback demos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := rootCmd
			for _, part := range tt.cmdPath {
				var found bool
				for _, subcmd := range cmd.Commands() {
					if subcmd.Name() == part {
						cmd = subcmd
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Command %v not found", tt.cmdPath)
				}
			}
			if !strings.Contains(cmd.Short, tt.wantHelp) {
				t.Errorf("Command help = %q, want to contain %q", cmd.Short, tt.wantHelp)
			}
		})
	}
}

func TestDemoFeedbackCreateFlags(t *testing.T) {
	cmd := rootCmd
	for _, part := range []string{"demo", "feedback", "create"} {
		for _, subcmd := range cmd.Commands() {
			if subcmd.Name() == part {
				cmd = subcmd
				break
			}
		}
	}

	if cmd.Name() != "create" {
		t.Fatal("demo feedback create command not found")
	}

	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("--repo flag not found")
	} else if repoFlag.DefValue != "." {
		t.Errorf("--repo default value = %q, want .", repoFlag.DefValue)
	}

	triesFlag := cmd.Flags().Lookup("tries")
	if triesFlag == nil {
		t.Error("--tries flag not found")
	} else if triesFlag.DefValue != "3" {
		t.Errorf("--tries default value = %q, want 3", triesFlag.DefValue)
	}

	aliases := cmd.Aliases
	if len(aliases) != 2 {
		t.Errorf("Expected 2 aliases, got %d", len(aliases))
	}
	expectedAliases := map[string]bool{"feedback:create": true, "feedback.create": true}
	for _, alias := range aliases {
		if !expectedAliases[alias] {
			t.Errorf("Unexpected alias: %s", alias)
		}
	}
}
