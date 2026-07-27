package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Advanced Git operations with AI assistance",
	Long:  `Smart Git operations powered by LLM for complex workflows.`,
}

var gitBisectCmd = &cobra.Command{
	Use:   "bisect <good-commit> <bad-commit>",
	Short: "AI-assisted binary search for bug introduction",
	Long: `Automatically find which commit introduced a bug using binary search.

Examples:
  gptcode git bisect v1.0.0 HEAD
  gptcode git bisect abc123 def456`,
	Args: cobra.ExactArgs(2),
	RunE: runGitBisect,
}

var gitCherryPickCmd = &cobra.Command{
	Use:   "cherry-pick <commits...>",
	Short: "Intelligent cherry-pick with conflict resolution",
	Long: `Cherry-pick commits with AI-powered conflict resolution.

Examples:
  gptcode git cherry-pick abc123
  gptcode git cherry-pick abc123 def456 ghi789`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGitCherryPick,
}

var gitRebaseCmd = &cobra.Command{
	Use:   "rebase [branch]",
	Short: "Interactive rebase with AI assistance",
	Long: `Rebase with intelligent conflict resolution and commit management.

Examples:
  gptcode git rebase main
  gptcode git rebase --interactive HEAD~5`,
	RunE: runGitRebase,
}

var gitSquashCmd = &cobra.Command{
	Use:   "squash <base-commit>",
	Short: "Squash commits with AI-powered commit message",
	Long: `Squash multiple commits into one with an intelligent commit message.

Examples:
  gptcode git squash HEAD~3
  gptcode git squash abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runGitSquash,
}

var gitRewordCmd = &cobra.Command{
	Use:   "reword <commit>",
	Short: "Reword commit message with AI assistance",
	Long: `Improve or fix a commit message using AI.

Examples:
  gptcode git reword HEAD
  gptcode git reword abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runGitReword,
}

var gitModel string
var gitInteractive bool

func init() {
	rootCmd.AddCommand(gitCmd)
	gitCmd.AddCommand(gitBisectCmd)
	gitCmd.AddCommand(gitCherryPickCmd)
	gitCmd.AddCommand(gitRebaseCmd)
	gitCmd.AddCommand(gitSquashCmd)
	gitCmd.AddCommand(gitRewordCmd)

	gitCmd.PersistentFlags().StringVar(&gitModel, "model", "", "LLM model to use")
	gitRebaseCmd.Flags().BoolVar(&gitInteractive, "interactive", false, "Interactive rebase")
}

func runGitBisect(cmd *cobra.Command, args []string) error {
	goodCommit := args[0]
	badCommit := args[1]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGitProvider(setup)
	if err != nil {
		return err
	}

	fmt.Printf("🔍 Starting bisect: %s (good) ... %s (bad)\n", goodCommit, badCommit)

	bisectCmd := exec.Command("git", "bisect", "start", badCommit, goodCommit)
	if err := bisectCmd.Run(); err != nil {
		return fmt.Errorf("failed to start bisect: %w", err)
	}

	defer exec.Command("git", "bisect", "reset").Run()

	iteration := 0
	for {
		iteration++
		fmt.Printf("\n📍 Iteration %d: Testing commit...\n", iteration)

		currentCmd := exec.Command("git", "rev-parse", "HEAD")
		currentHash, err := currentCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get current commit: %w", err)
		}

		testCmd := exec.Command("go", "test", "./...")
		testOutput, testErr := testCmd.CombinedOutput()

		var status string
		if testErr == nil {
			status = "good"
			fmt.Println("✅ Tests passed - marking as good")
		} else {
			status = "bad"
			fmt.Println("❌ Tests failed - marking as bad")
		}

		markCmd := exec.Command("git", "bisect", status)
		output, err := markCmd.CombinedOutput()

		if strings.Contains(string(output), "is the first bad commit") {
			fmt.Println("\n🎯 Found the culprit!")

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			analysis, _ := analyzeCommit(ctx, provider, model, string(currentHash), string(testOutput))
			fmt.Println("\n📊 Analysis:")
			fmt.Println(analysis)
			break
		}

		if err != nil {
			break
		}
	}

	return nil
}

func runGitCherryPick(cmd *cobra.Command, args []string) error {
	commits := args

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGitProvider(setup)
	if err != nil {
		return err
	}

	fmt.Printf("🍒 Cherry-picking %d commit(s)...\n", len(commits))

	for i, commit := range commits {
		fmt.Printf("\n[%d/%d] Cherry-picking %s\n", i+1, len(commits), commit)

		cherryCmd := exec.Command("git", "cherry-pick", commit)
		output, err := cherryCmd.CombinedOutput()

		if err == nil {
			fmt.Println("✅ Applied successfully")
			continue
		}

		if !strings.Contains(string(output), "conflict") {
			return fmt.Errorf("cherry-pick failed: %s", string(output))
		}

		fmt.Println("⚠️  Conflicts detected - resolving...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := resolveConflicts(ctx, provider, model, commit); err != nil {
			cancel()
			return fmt.Errorf("failed to resolve conflicts: %w", err)
		}
		cancel()

		continueCmd := exec.Command("git", "cherry-pick", "--continue")
		if err := continueCmd.Run(); err != nil {
			return fmt.Errorf("failed to continue cherry-pick: %w", err)
		}

		fmt.Println("✅ Conflicts resolved and applied")
	}

	fmt.Println("\n✅ All commits cherry-picked successfully")
	return nil
}

func runGitRebase(cmd *cobra.Command, args []string) error {
	target := "main"
	if len(args) > 0 {
		target = args[0]
	}

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGitProvider(setup)
	if err != nil {
		return err
	}

	fmt.Printf("🔄 Rebasing onto %s...\n", target)

	rebaseArgs := []string{"rebase", target}
	if gitInteractive {
		rebaseArgs = []string{"rebase", "-i", target}
	}

	rebaseCmd := exec.Command("git", rebaseArgs...)
	output, err := rebaseCmd.CombinedOutput()

	if err == nil {
		fmt.Println("✅ Rebase completed successfully")
		return nil
	}

	if !strings.Contains(string(output), "conflict") {
		return fmt.Errorf("rebase failed: %s", string(output))
	}

	fmt.Println("⚠️  Conflicts detected - resolving...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := resolveConflicts(ctx, provider, model, target); err != nil {
		return fmt.Errorf("failed to resolve conflicts: %w", err)
	}

	continueCmd := exec.Command("git", "rebase", "--continue")
	if err := continueCmd.Run(); err != nil {
		return fmt.Errorf("failed to continue rebase: %w", err)
	}

	fmt.Println("✅ Rebase completed with conflict resolution")
	return nil
}

func analyzeCommit(ctx context.Context, provider llm.Provider, model, commit, testOutput string) (string, error) {
	showCmd := exec.Command("git", "show", commit)
	diff, _ := showCmd.Output()

	prompt := fmt.Sprintf(`Analyze this commit that introduced a bug:

Commit: %s

Diff:
%s

Test failure:
%s

Provide:
1. What the commit changed
2. Why it likely broke tests
3. Suggested fix

Be concise.`, commit, string(diff), testOutput)

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that analyzes git commits and identifies bugs.",
		UserPrompt:   prompt,
		Model:        model,
	})

	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func resolveConflicts(ctx context.Context, provider llm.Provider, model, reference string) error {
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(statusOutput), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "UU ") && !strings.HasPrefix(line, "AA ") {
			continue
		}

		file := strings.TrimSpace(line[3:])
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		if !strings.Contains(string(content), "<<<<<<<") {
			continue
		}

		fmt.Printf("  Resolving %s...\n", file)

		prompt := fmt.Sprintf(`Resolve this merge conflict:

File: %s
Reference: %s

Content with conflicts:
%s

Return ONLY the resolved file content with conflicts removed.`, file, reference, string(content))

		resp, err := provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: "You are a helpful assistant that resolves merge conflicts intelligently.",
			UserPrompt:   prompt,
			Model:        model,
		})

		if err != nil {
			return err
		}

		resolved := strings.TrimSpace(resp.Text)
		if strings.HasPrefix(resolved, "```") {
			resolved = strings.TrimPrefix(resolved, "```go\n")
			resolved = strings.TrimPrefix(resolved, "```\n")
			resolved = strings.TrimSuffix(resolved, "```")
		}

		if err := os.WriteFile(file, []byte(resolved), 0644); err != nil {
			return err
		}

		addCmd := exec.Command("git", "add", file)
		if err := addCmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func runGitSquash(cmd *cobra.Command, args []string) error {
	baseCommit := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGitProvider(setup)
	if err != nil {
		return err
	}

	logCmd := exec.Command("git", "log", "--oneline", baseCommit+"..HEAD")
	commitsOutput, err := logCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	commitLines := strings.Split(strings.TrimSpace(string(commitsOutput)), "\n")
	if len(commitLines) == 0 {
		fmt.Println("✅ No commits to squash")
		return nil
	}

	fmt.Printf("📦 Squashing %d commit(s)...\n\n", len(commitLines))
	for _, line := range commitLines {
		fmt.Printf("  • %s\n", line)
	}

	diffCmd := exec.Command("git", "diff", baseCommit+"..HEAD")
	diffOutput, _ := diffCmd.Output()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`Generate a concise commit message for squashing these commits:

%s

Diff summary:
%s

Provide only the commit message (first line is subject, then blank line, then optional body).`, string(commitsOutput), truncate(string(diffOutput), 2000))

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that generates concise, well-formatted git commit messages.",
		UserPrompt:   prompt,
		Model:        model,
	})

	if err != nil {
		return fmt.Errorf("failed to generate message: %w", err)
	}

	message := strings.TrimSpace(resp.Text)
	fmt.Println("\n📝 Generated commit message:")
	fmt.Println(message)

	resetCmd := exec.Command("git", "reset", "--soft", baseCommit)
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Println("\n✅ Commits squashed successfully")
	return nil
}

func runGitReword(cmd *cobra.Command, args []string) error {
	commit := args[0]

	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	provider, model, err := getGitProvider(setup)
	if err != nil {
		return err
	}

	showCmd := exec.Command("git", "show", "--no-patch", "--format=%B", commit)
	currentMsg, err := showCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit message: %w", err)
	}

	diffCmd := exec.Command("git", "show", commit)
	diffOutput, _ := diffCmd.Output()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`Improve this commit message following best practices:

Current message:
%s

Commit diff:
%s

Provide an improved message (subject line + optional body). Be concise.`, string(currentMsg), truncate(string(diffOutput), 2000))

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that improves git commit messages following best practices.",
		UserPrompt:   prompt,
		Model:        model,
	})

	if err != nil {
		return fmt.Errorf("failed to generate message: %w", err)
	}

	message := strings.TrimSpace(resp.Text)
	fmt.Println("📝 Suggested commit message:")
	fmt.Println(message)
	fmt.Println("\n💡 To apply: git commit --amend -m \"<message>\"")

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}

func getGitProvider(setup *config.Setup) (llm.Provider, string, error) {
	model := gitModel
	backendName := setup.Defaults.Backend
	if backendName == "" {
		backendName = "anthropic"
	}

	backendCfg, ok := setup.Backend[backendName]
	if !ok {
		return nil, "", fmt.Errorf("backend %s not configured", backendName)
	}

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	if model == "" {
		model = backendCfg.GetModelForAgent("editor")
		if model == "" {
			model = backendCfg.DefaultModel
		}
	}

	if model == "" {
		return nil, "", fmt.Errorf("no model configured")
	}

	return provider, model, nil
}
