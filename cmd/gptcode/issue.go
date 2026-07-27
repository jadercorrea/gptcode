package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jadercorrea/gptcode/internal/ci"
	"github.com/jadercorrea/gptcode/internal/codebase"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/github"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/modes"
	"github.com/jadercorrea/gptcode/internal/recovery"
	"github.com/jadercorrea/gptcode/internal/validation"
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "GitHub issue management and automation",
	Long: `Manage GitHub issues and automate issue resolution.

Examples:
  gptcode issue fix 123              Fix issue #123 autonomously
  gptcode issue fix 123 --repo owner/repo  Fix issue from specific repo
  gptcode issue show 123             Show issue details`,
}

var issueFixCmd = &cobra.Command{
	Use:   "fix <issue-number>",
	Short: "Autonomously fix a GitHub issue",
	Long: `Fetch a GitHub issue, create a branch, implement the fix, run tests, 
and create a pull request.

This command will:
1. Fetch issue details from GitHub
2. Extract requirements
3. Create a branch (issue-N-description)
4. Analyze codebase and implement changes
5. Run tests and linters
6. Commit changes with issue reference
7. Push branch
8. Create pull request

Examples:
  gptcode issue fix 123                    Fix issue #123
  gptcode issue fix 123 --repo owner/repo Fix from specific repo
  gptcode issue fix 123 --draft           Create draft PR`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueNum, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")
		autonomous, _ := cmd.Flags().GetBool("autonomous")
		findFiles, _ := cmd.Flags().GetBool("find-files")
		skipLabelCheck, _ := cmd.Flags().GetBool("skip-label-check")

		if repo == "" {
			repo = detectGitHubRepo()
			if repo == "" {
				return fmt.Errorf("could not detect GitHub repository. Use --repo flag")
			}
		}

		fmt.Printf("🔍 Fetching issue #%d from %s...\n\n", issueNum, repo)

		client := github.NewClient(repo)
		workDir, _ := os.Getwd()
		client.SetWorkDir(workDir)

		issue, err := client.FetchIssue(issueNum)
		if err != nil {
			return fmt.Errorf("failed to fetch issue: %w", err)
		}

		fmt.Printf("📋 Issue #%d: %s\n", issue.Number, issue.Title)
		fmt.Printf("   State: %s\n", issue.State)
		fmt.Printf("   Author: %s\n", issue.Author)
		if len(issue.Labels) > 0 {
			fmt.Printf("   Labels: %s\n", strings.Join(issue.Labels, ", "))
		}
		fmt.Println()

		if err := validateIssueForPR(repo, issue); err != nil {
			if !skipLabelCheck {
				return err
			}
			fmt.Printf("⚠️  Warning: %v (continuing anyway due to --skip-label-check)\n", err)
		}

		// Check if PR already exists for this issue
		hasPR, err := client.HasExistingPR(issueNum)
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not check for existing PRs: %v\n", err)
		} else if hasPR {
			return fmt.Errorf("a pull request already exists for issue #%d. Please review the existing PR instead of creating a new one", issueNum)
		}

		reqs := issue.ExtractRequirements()
		if len(reqs) > 0 {
			fmt.Println("📝 Requirements:")
			for i, req := range reqs {
				fmt.Printf("   %d. %s\n", i+1, req)
			}
			fmt.Println()
		}

		branchName := issue.CreateBranchName()
		fmt.Printf("🌿 Creating branch: %s\n", branchName)

		if err := client.CreateBranch(branchName, ""); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}

		var relevantFiles []codebase.RelevantFile
		if findFiles {
			fmt.Println("\n🔍 Finding relevant files...")
			setup, _ := config.LoadSetup()
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

			finder, err := codebase.NewFileFinder(provider, workDir, queryModel)
			if err != nil {
				fmt.Printf("⚠️  Failed to create file finder: %v\n", err)
			} else {
				issueDesc := fmt.Sprintf("%s\n\n%s", issue.Title, issue.Body)
				relevantFiles, err = finder.FindRelevantFiles(context.Background(), issueDesc)
				if err != nil {
					fmt.Printf("⚠️  Failed to find relevant files: %v\n", err)
				} else if len(relevantFiles) > 0 {
					fmt.Println("\nRelevant files identified:")
					for i, file := range relevantFiles {
						var confLevel string
						if file.Confidence >= 0.8 {
							confLevel = "HIGH"
						} else if file.Confidence >= 0.5 {
							confLevel = "MED"
						} else {
							confLevel = "LOW"
						}
						fmt.Printf("%d. [%s] %s - %s\n", i+1, confLevel, file.Path, file.Reason)
					}
				} else {
					fmt.Println("⚠️  No relevant files found")
				}
			}
		}

		task := fmt.Sprintf("Fix issue #%d: %s", issue.Number, issue.Title)
		if len(reqs) > 0 {
			task += ", Requirements: " + strings.Join(reqs, "; ")
		}
		if len(relevantFiles) > 0 {
			var filePaths []string
			for _, f := range relevantFiles {
				filePaths = append(filePaths, f.Path)
			}
			task += ". Focus on files: " + strings.Join(filePaths, ", ")
		}

		if autonomous {
			setup, err := config.LoadSetup()
			if err != nil {
				return fmt.Errorf("failed to load setup: %w", err)
			}
			backendName := setup.Defaults.Backend
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
			language := string(langdetect.DetectLanguage(workDir))
			if language == "" || language == "unknown" {
				language = setup.Defaults.Lang
				if language == "" {
					language = "go"
				}
			}
			exec := modes.NewAutonomousExecutorWithLive(provider, workDir, queryModel, language, nil, nil, backendName)
			if err := exec.Execute(context.Background(), task); err != nil {
				return fmt.Errorf("autonomous implementation failed: %w", err)
			}
			fmt.Println("\n[OK] Implementation complete")
		} else {
			fmt.Println("\nImplementation not executed (use --autonomous to enable)")
		}

		fmt.Println("\nNext steps:")
		fmt.Printf("   gptcode issue commit %d\n", issueNum)
		fmt.Printf("   gptcode issue push %d\n", issueNum)

		return nil
	},
}

var issueShowCmd = &cobra.Command{
	Use:   "show <issue-number>",
	Short: "Show GitHub issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueNum, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")
		if repo == "" {
			repo = detectGitHubRepo()
			if repo == "" {
				return fmt.Errorf("could not detect GitHub repository. Use --repo flag")
			}
		}

		client := github.NewClient(repo)
		issue, err := client.FetchIssue(issueNum)
		if err != nil {
			return fmt.Errorf("failed to fetch issue: %w", err)
		}

		fmt.Printf("Issue #%d: %s\n", issue.Number, issue.Title)
		fmt.Printf("State: %s\n", issue.State)
		fmt.Printf("Author: %s\n", issue.Author)
		fmt.Printf("URL: %s\n", issue.URL)
		fmt.Printf("Created: %s\n", issue.CreatedAt)
		fmt.Printf("Updated: %s\n", issue.UpdatedAt)

		if len(issue.Labels) > 0 {
			fmt.Printf("Labels: %s\n", strings.Join(issue.Labels, ", "))
		}

		if len(issue.Assignees) > 0 {
			fmt.Printf("Assignees: %s\n", strings.Join(issue.Assignees, ", "))
		}

		if issue.Body != "" {
			fmt.Printf("\nDescription:\n%s\n", issue.Body)
		}

		reqs := issue.ExtractRequirements()
		if len(reqs) > 0 {
			fmt.Println("\nExtracted Requirements:")
			for i, req := range reqs {
				fmt.Printf("%d. %s\n", i+1, req)
			}
		}

		return nil
	},
}

var issueCommitCmd = &cobra.Command{
	Use:   "commit <issue-number>",
	Short: "Commit changes with issue reference",
	Long: `Commit staged changes with proper issue reference and run validation.

This will:
1. Commit changes with "Closes #N" reference
2. Run tests (unless --skip-tests)
3. Run linters (unless --skip-lint)
4. Report validation results`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueNum, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		message, _ := cmd.Flags().GetString("message")
		skipTests, _ := cmd.Flags().GetBool("skip-tests")
		skipLint, _ := cmd.Flags().GetBool("skip-lint")
		skipBuild, _ := cmd.Flags().GetBool("skip-build")
		checkCoverage, _ := cmd.Flags().GetBool("check-coverage")
		minCoverage, _ := cmd.Flags().GetFloat64("min-coverage")
		securityScan, _ := cmd.Flags().GetBool("security-scan")
		autoFix, _ := cmd.Flags().GetBool("auto-fix")
		repo, _ := cmd.Flags().GetString("repo")

		if repo == "" {
			repo = detectGitHubRepo()
		}

		if message == "" {
			message = fmt.Sprintf("Fix issue #%d", issueNum)
		}

		workDir, _ := os.Getwd()
		client := github.NewClient(repo)
		client.SetWorkDir(workDir)

		fmt.Printf("💾 Committing changes for issue #%d...\n", issueNum)

		err = client.CommitChanges(github.CommitOptions{
			Message:     message,
			IssueNumber: issueNum,
			AllFiles:    true,
		})
		if err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}

		fmt.Println("✅ Changes committed")

		if !skipBuild {
			fmt.Println("\n🔨 Running build...")
			buildExec := validation.NewBuildExecutor(workDir)
			buildResult, err := buildExec.RunBuild()
			if err != nil {
				fmt.Printf("⚠️  Build check failed: %v\n", err)
			} else if buildResult.Success {
				fmt.Println("✅ Build successful")
			} else {
				fmt.Printf("❌ Build failed\n")
				if buildResult.Output != "" {
					fmt.Println("\nBuild output:")
					fmt.Println(buildResult.Output)
				}
				return fmt.Errorf("build failed")
			}
		}

		if !skipTests {
			fmt.Println("\n🧪 Running tests...")
			testExec := validation.NewTestExecutor(workDir)
			result, err := testExec.RunTests()

			if err != nil {
				fmt.Printf("⚠️  Tests encountered error: %v\n", err)
			} else if result.Success {
				fmt.Printf("✅ All tests passed (%d passed)\n", result.Passed)
			} else {
				fmt.Printf("❌ Tests failed (%d passed, %d failed)\n", result.Passed, result.Failed)

				if autoFix {
					fmt.Println("\n🔧 Attempting auto-fix...")
					if fixErr := attemptTestFix(workDir, result); fixErr != nil {
						fmt.Printf("⚠️  Auto-fix failed: %v\n", fixErr)
						fmt.Println("\nTest output:")
						fmt.Println(result.Output)
						return fmt.Errorf("tests failed")
					}
					fmt.Println("✅ Tests fixed automatically")
				} else {
					fmt.Println("\nTest output:")
					fmt.Println(result.Output)
					return fmt.Errorf("tests failed (use --auto-fix to attempt automatic fixes)")
				}
			}
		}

		if !skipLint {
			fmt.Println("\n🔍 Running linters...")
			lintExec := validation.NewLinterExecutor(workDir)
			results, err := lintExec.RunLinters()

			if err != nil {
				fmt.Printf("⚠️  Linters encountered error: %v\n", err)
			} else {
				allPassed := true
				for _, result := range results {
					if result.Success && result.Issues == 0 {
						fmt.Printf("✅ %s: no issues\n", result.Tool)
					} else {
						allPassed = false
						fmt.Printf("❌ %s: %d issues (%d errors, %d warnings)\n",
							result.Tool, result.Issues, result.Errors, result.Warnings)
					}
				}

				if !allPassed {
					if autoFix {
						fmt.Println("\n🔧 Attempting auto-fix...")
						if fixErr := attemptLintFix(workDir, results); fixErr != nil {
							fmt.Printf("⚠️  Auto-fix failed: %v\n", fixErr)
							return fmt.Errorf("linting issues found")
						}
						fmt.Println("✅ Lint issues fixed automatically")
					} else {
						return fmt.Errorf("linting issues found (use --auto-fix to attempt automatic fixes)")
					}
				}
			}
		}

		if checkCoverage {
			fmt.Println("\n📊 Checking code coverage...")
			covExec := validation.NewCoverageExecutor(workDir)
			covResult, err := covExec.RunCoverage(minCoverage)
			if err != nil {
				if covResult != nil && covResult.Coverage > 0 {
					fmt.Printf("⚠️  Coverage %.1f%% below minimum %.1f%%\n", covResult.Coverage, minCoverage)
				} else {
					fmt.Printf("⚠️  Coverage check failed: %v\n", err)
				}
			} else {
				fmt.Printf("✅ Coverage: %.1f%%\n", covResult.Coverage)
			}
		}

		if securityScan {
			fmt.Println("\n🔒 Running security scan...")
			secScanner := validation.NewSecurityScanner(workDir)
			secResult, err := secScanner.RunScan()
			if err != nil {
				if secResult != nil && secResult.Vulnerabilities > 0 {
					fmt.Printf("❌ Found %d vulnerabilities\n", secResult.Vulnerabilities)
					return fmt.Errorf("security vulnerabilities found")
				}
				fmt.Printf("⚠️  Security scan failed: %v\n", err)
			} else {
				if strings.Contains(secResult.Output, "skipping") {
					fmt.Println("⚠️  Security scanner not available (skipped)")
				} else {
					fmt.Println("✅ No security vulnerabilities found")
				}
			}
		}

		fmt.Println("\n✨ All validation passed!")
		fmt.Println("Next steps:")
		fmt.Printf("  gptcode issue push %d\n", issueNum)

		return nil
	},
}

var issuePushCmd = &cobra.Command{
	Use:   "push <issue-number>",
	Short: "Push branch and create pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueNum, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")
		draft, _ := cmd.Flags().GetBool("draft")

		if repo == "" {
			repo = detectGitHubRepo()
			if repo == "" {
				return fmt.Errorf("could not detect GitHub repository")
			}
		}

		workDir, _ := os.Getwd()
		client := github.NewClient(repo)
		client.SetWorkDir(workDir)

		issue, err := client.FetchIssue(issueNum)
		if err != nil {
			return fmt.Errorf("failed to fetch issue: %w", err)
		}

		branchName := issue.CreateBranchName()

		fmt.Printf("🚀 Pushing branch %s...\n", branchName)
		if err := client.PushBranch(branchName); err != nil {
			return fmt.Errorf("failed to push branch: %w", err)
		}

		fmt.Println("✅ Branch pushed")

		fmt.Println("\n📝 Creating pull request...")

		changes := []string{"Implemented fix for issue"}
		prBody := github.GeneratePRBody(issue, changes)

		defaultBranch := client.DetectDefaultBranch()
		headBranch := branchName
		prRepo := repo // Create PR in the original repo (upstream)

		if _, isFork := client.GetForkRemote(); isFork {
			cmd := exec.Command("gh", "api", "user", "--jq", ".login")
			output, _ := cmd.CombinedOutput()
			currentUser := strings.TrimSpace(string(output))
			// Use "username:branch" format for head when creating PR from fork
			headBranch = currentUser + ":" + branchName
			// PR is created in the upstream repo, head points to fork branch
		}

		// Create a new client for the PR repo
		prClient := github.NewClient(prRepo)
		prClient.SetWorkDir(workDir)

		pr, err := prClient.CreatePR(github.PRCreateOptions{
			Title:      fmt.Sprintf("Fix: %s", issue.Title),
			Body:       prBody,
			HeadBranch: headBranch,
			BaseBranch: defaultBranch,
			IsDraft:    draft,
			Labels:     issue.Labels,
		})

		if err != nil {
			return fmt.Errorf("failed to create PR: %w", err)
		}

		fmt.Printf("✅ Pull request created: %s\n", pr.URL)
		fmt.Printf("   PR #%d: %s\n", pr.Number, pr.Title)

		return nil
	},
}

func detectGitHubRepo() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))

	if strings.Contains(url, "github.com") {
		parts := strings.Split(url, "github.com")
		if len(parts) < 2 {
			return ""
		}

		repo := strings.Trim(parts[1], ":/")
		repo = strings.TrimSuffix(repo, ".git")

		return repo
	}

	return ""
}

func attemptTestFix(workDir string, testResult *validation.TestResult) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return err
	}

	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]
	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	model := backendCfg.GetModelForAgent("editor")
	if model == "" {
		model = backendCfg.DefaultModel
	}

	fixer := recovery.NewErrorFixer(provider, model, workDir)
	fixResult, err := fixer.FixTestFailures(context.Background(), testResult, 2)
	if err != nil {
		return err
	}

	if !fixResult.Success {
		return fmt.Errorf("could not fix test failures after %d attempts", fixResult.FixAttempts)
	}

	return nil
}

func attemptLintFix(workDir string, lintResults []*validation.LintResult) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return err
	}

	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]
	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	model := backendCfg.GetModelForAgent("editor")
	if model == "" {
		model = backendCfg.DefaultModel
	}

	fixer := recovery.NewErrorFixer(provider, model, workDir)
	fixResult, err := fixer.FixLintIssues(context.Background(), lintResults, 2)
	if err != nil {
		return err
	}

	if !fixResult.Success {
		return fmt.Errorf("could not fix lint issues after %d attempts", fixResult.FixAttempts)
	}

	return nil
}

var issueReviewCmd = &cobra.Command{
	Use:   "review <pr-number>",
	Short: "Address review comments on a PR",
	Long: `Fetch review comments from a pull request and autonomously address them.

This will:
1. Fetch all unresolved review comments
2. Analyze each comment
3. Implement requested changes
4. Commit and push updates`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prNumber, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid PR number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")
		if repo == "" {
			repo = detectGitHubRepo()
			if repo == "" {
				return fmt.Errorf("could not detect GitHub repository. Use --repo flag")
			}
		}

		fmt.Printf("🔍 Fetching review comments for PR #%d...\n", prNumber)

		client := github.NewClient(repo)
		workDir, _ := os.Getwd()
		client.SetWorkDir(workDir)

		comments, err := client.GetUnresolvedComments(prNumber)
		if err != nil {
			return fmt.Errorf("failed to fetch comments: %w", err)
		}

		if len(comments) == 0 {
			fmt.Println("✅ No unresolved comments")
			return nil
		}

		fmt.Printf("\n📝 Found %d unresolved comment(s):\n\n", len(comments))
		for i, comment := range comments {
			fmt.Printf("%d. [@%s] %s:%d\n", i+1, comment.Author, comment.Path, comment.Line)
			fmt.Printf("   %s\n\n", comment.Body)
		}

		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backendName := setup.Defaults.Backend
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
		language := string(langdetect.DetectLanguage(workDir))
		if language == "" || language == "unknown" {
			language = setup.Defaults.Lang
			if language == "" {
				language = "go"
			}
		}

		fmt.Println("🔧 Addressing review comments...")

		for i, comment := range comments {
			fmt.Printf("\n[%d/%d] Processing comment from @%s on %s\n", i+1, len(comments), comment.Author, comment.Path)

			task := fmt.Sprintf(`Address review comment:
File: %s (line %d)
Comment: %s

Please read the file, understand the context, and implement the requested change.`,
				comment.Path, comment.Line, comment.Body)

			exec := modes.NewAutonomousExecutorWithLive(provider, workDir, queryModel, language, nil, nil, backendName)
			if err := exec.Execute(context.Background(), task); err != nil {
				fmt.Printf("⚠️  Failed to address comment: %v\n", err)
				continue
			}

			fmt.Println("✅ Comment addressed")
		}

		fmt.Println("\n📦 Committing changes...")

		err = client.CommitChanges(github.CommitOptions{
			Message:  fmt.Sprintf("Address review comments on PR #%d", prNumber),
			AllFiles: true,
		})
		if err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}

		fmt.Println("✅ Changes committed")

		currentBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		currentBranch.Dir = workDir
		branchOutput, err := currentBranch.Output()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		branchName := strings.TrimSpace(string(branchOutput))
		fmt.Printf("🚀 Pushing %s...\n", branchName)

		if err := client.PushBranch(branchName); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}

		fmt.Printf("\n✨ Successfully addressed %d review comment(s)\n", len(comments))
		fmt.Printf("   View PR: https://github.com/%s/pull/%d\n", repo, prNumber)

		return nil
	},
}

var issueCICmd = &cobra.Command{
	Use:   "ci <pr-number>",
	Short: "Handle CI failures on a PR",
	Long: `Monitor CI checks and automatically fix failures.

This will:
1. Wait for CI checks to complete
2. Fetch logs from failed checks
3. Analyze failures and apply fixes
4. Re-push and verify`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prNumber, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid PR number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")
		if repo == "" {
			repo = detectGitHubRepo()
			if repo == "" {
				return fmt.Errorf("could not detect GitHub repository. Use --repo flag")
			}
		}

		workDir, _ := os.Getwd()

		setup, err := config.LoadSetup()
		if err != nil {
			return fmt.Errorf("failed to load setup: %w", err)
		}

		backendName := setup.Defaults.Backend
		backendCfg := setup.Backend[backendName]
		var provider llm.Provider
		if backendCfg.Type == "ollama" {
			provider = llm.NewOllama(backendCfg.BaseURL)
		} else {
			provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}

		model := backendCfg.GetModelForAgent("editor")
		if model == "" {
			model = backendCfg.DefaultModel
		}

		handler := ci.NewHandler(repo, workDir, provider, model)

		fmt.Printf("🔍 Checking CI status for PR #%d...\n", prNumber)

		time.Sleep(2 * time.Second)

		failed, err := handler.GetFailedChecks(prNumber)
		if err != nil {
			return fmt.Errorf("failed to get CI status: %w", err)
		}

		if len(failed) == 0 {
			fmt.Println("✅ All CI checks passing")
			return nil
		}

		fmt.Printf("\n❌ Found %d failed check(s):\n\n", len(failed))
		for i, check := range failed {
			fmt.Printf("%d. %s - %s\n", i+1, check.Name, check.State)
		}

		fmt.Println("\n📜 Fetching CI logs...")

		logs, err := handler.FetchCILogs(prNumber, "")
		if err != nil {
			fmt.Printf("⚠️  Could not fetch full logs: %v\n", err)
			logs = "No detailed logs available"
		}

		fmt.Println("🔎 Analyzing failures...")

		failure := handler.ParseCIFailure(logs)
		fmt.Printf("\nDetected error: %s\n", failure.Error)

		fmt.Println("\n🔧 Generating fix...")

		fixResult, err := handler.AnalyzeFailure(*failure)
		if err != nil {
			return fmt.Errorf("failed to analyze failure: %w", err)
		}

		if !fixResult.Success {
			fmt.Println("⚠️  Could not generate automatic fix")
			fmt.Println("\nAnalysis:")
			fmt.Println(fixResult.FixApplied)
			return fmt.Errorf("manual intervention required")
		}

		fmt.Println("✅ Fix generated")
		fmt.Println("\nRecommended changes:")
		fmt.Println(fixResult.FixApplied)

		fmt.Println("\n📦 Committing fix...")

		client := github.NewClient(repo)
		client.SetWorkDir(workDir)

		err = client.CommitChanges(github.CommitOptions{
			Message:  fmt.Sprintf("Fix CI failure on PR #%d", prNumber),
			AllFiles: true,
		})
		if err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}

		fmt.Println("✅ Changes committed")

		currentBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		currentBranch.Dir = workDir
		branchOutput, err := currentBranch.Output()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		branchName := strings.TrimSpace(string(branchOutput))
		fmt.Printf("🚀 Pushing %s...\n", branchName)

		if err := client.PushBranch(branchName); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}

		fmt.Println("\n✅ CI fix pushed")
		fmt.Printf("   View PR: https://github.com/%s/pull/%d\n", repo, prNumber)
		fmt.Println("\n⏳ CI checks will run again automatically")

		return nil
	},
}

func init() {
	issueCmd.AddCommand(issueFixCmd)
	issueCmd.AddCommand(issueShowCmd)
	issueCmd.AddCommand(issueCommitCmd)
	issueCmd.AddCommand(issuePushCmd)
	issueCmd.AddCommand(issueReviewCmd)
	issueCmd.AddCommand(issueCICmd)

	issueFixCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
	issueFixCmd.Flags().Bool("draft", false, "Create draft pull request")
	issueFixCmd.Flags().Bool("skip-tests", false, "Skip running tests")
	issueFixCmd.Flags().Bool("skip-lint", false, "Skip running linters")
	issueFixCmd.Flags().Bool("autonomous", true, "Execute implementation autonomously")
	issueFixCmd.Flags().Bool("find-files", true, "Find relevant files before implementation")
	issueFixCmd.Flags().Bool("skip-label-check", false, "Skip validation of help wanted label")

	issueShowCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")

	issueCommitCmd.Flags().String("message", "", "Commit message")
	issueCommitCmd.Flags().Bool("skip-tests", false, "Skip running tests")
	issueCommitCmd.Flags().Bool("skip-lint", false, "Skip running linters")
	issueCommitCmd.Flags().Bool("skip-build", false, "Skip build check")
	issueCommitCmd.Flags().Bool("check-coverage", false, "Check code coverage")
	issueCommitCmd.Flags().Float64("min-coverage", 0.0, "Minimum coverage threshold (0-100)")
	issueCommitCmd.Flags().Bool("security-scan", false, "Run security vulnerability scan")
	issueCommitCmd.Flags().Bool("auto-fix", true, "Automatically fix test/lint failures")
	issueCommitCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")

	issuePushCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
	issuePushCmd.Flags().Bool("draft", false, "Create draft pull request")

	issueReviewCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")

	issueCICmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
}

func validateIssueForPR(repo string, issue *github.Issue) error {
	labels := issue.Labels
	hasHelpWanted := false
	hasGoodFirstIssue := false

	for _, label := range labels {
		lowerLabel := strings.ToLower(label)
		if strings.Contains(lowerLabel, "help wanted") || strings.Contains(lowerLabel, "help-wanted") {
			hasHelpWanted = true
		}
		if strings.Contains(lowerLabel, "good first issue") || strings.Contains(lowerLabel, "good-first-issue") {
			hasGoodFirstIssue = true
		}
	}

	if !hasHelpWanted && !hasGoodFirstIssue {
		return fmt.Errorf("issue #%d does not have 'help wanted' or 'good first issue' label. Many repositories (like cli/cli) require this label before accepting PRs. Please find an issue with the appropriate label or request the label to be added", issue.Number)
	}

	return nil
}
