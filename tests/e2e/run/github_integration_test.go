//go:build e2e

package run_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCommand(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command %s %v failed: %v\nOutput: %s", name, args, err, string(output))
	}
	return string(output)
}

func TestGitHubIssueIntegration(t *testing.T) {
	t.Run("fetch real issue from public repo", func(t *testing.T) {
		if os.Getenv("SKIP_GH_TESTS") != "" {
			t.Skip("Skipping GitHub integration test (SKIP_GH_TESTS set)")
		}

		client := github.NewClient("cli/cli")

		issue, err := client.FetchIssue(1)
		require.NoError(t, err, "Failed to fetch issue #1")

		assert.Equal(t, 1, issue.Number)
		assert.NotEmpty(t, issue.Title)
		assert.NotEmpty(t, issue.URL)
		assert.Contains(t, issue.URL, "github.com")
		assert.Equal(t, "cli/cli", issue.Repository)
	})

	t.Run("fetch issue with labels and metadata", func(t *testing.T) {
		if os.Getenv("SKIP_GH_TESTS") != "" {
			t.Skip("Skipping GitHub integration test (SKIP_GH_TESTS set)")
		}

		client := github.NewClient("cli/cli")

		issue, err := client.FetchIssue(1)
		require.NoError(t, err, "Failed to fetch issue #1")

		assert.NotEmpty(t, issue.State)
		assert.NotEmpty(t, issue.Author)
		assert.NotEmpty(t, issue.CreatedAt)
		assert.NotEmpty(t, issue.UpdatedAt)
	})
}

func TestIssueRequirementExtraction(t *testing.T) {
	t.Run("extract task list requirements", func(t *testing.T) {
		issue := &github.Issue{
			Title: "Fix authentication bug",
			Body: `Need to fix the auth flow:
- [ ] Update token validation
- [x] Add expiry check
- [ ] Implement refresh logic`,
		}

		reqs := issue.ExtractRequirements()

		require.Len(t, reqs, 3)
		assert.Equal(t, "Update token validation", reqs[0])
		assert.Equal(t, "Add expiry check", reqs[1])
		assert.Equal(t, "Implement refresh logic", reqs[2])
	})

	t.Run("extract bullet point requirements", func(t *testing.T) {
		issue := &github.Issue{
			Title: "Add new feature",
			Body: `This feature should:
- Support multiple file formats
* Handle large file uploads
- Validate file contents properly`,
		}

		reqs := issue.ExtractRequirements()

		require.GreaterOrEqual(t, len(reqs), 3)
		assert.Contains(t, reqs[0], "multiple file formats")
		assert.Contains(t, reqs[1], "large file uploads")
		assert.Contains(t, reqs[2], "file contents")
	})

	t.Run("extract numbered list requirements", func(t *testing.T) {
		issue := &github.Issue{
			Title: "Refactor database layer",
			Body: `Steps:
1. Extract common queries
2. Add connection pooling
3. Implement retry logic`,
		}

		reqs := issue.ExtractRequirements()

		require.Len(t, reqs, 3)
		assert.Equal(t, "Extract common queries", reqs[0])
		assert.Equal(t, "Add connection pooling", reqs[1])
		assert.Equal(t, "Implement retry logic", reqs[2])
	})

	t.Run("fallback to title when no structured requirements", func(t *testing.T) {
		issue := &github.Issue{
			Title: "Fix memory leak in worker pool",
			Body:  `The worker pool has a memory leak that needs to be fixed.`,
		}

		reqs := issue.ExtractRequirements()

		require.Len(t, reqs, 1)
		assert.Equal(t, "Fix memory leak in worker pool", reqs[0])
	})
}

func TestIssueReferencesParsing(t *testing.T) {
	t.Run("extract issue references", func(t *testing.T) {
		text := "This fixes #123 and is related to #456"
		refs := github.ParseReferences(text)

		require.Len(t, refs, 2)
		assert.Contains(t, refs, "#123")
		assert.Contains(t, refs, "#456")
	})

	t.Run("handle references with punctuation", func(t *testing.T) {
		text := "See #789, #101. Also check #202!"
		refs := github.ParseReferences(text)

		require.Len(t, refs, 3)
		assert.Contains(t, refs, "#789")
		assert.Contains(t, refs, "#101")
		assert.Contains(t, refs, "#202")
	})

	t.Run("ignore non-numeric references", func(t *testing.T) {
		text := "Use #define and check #abc"
		refs := github.ParseReferences(text)

		assert.Len(t, refs, 0)
	})
}

func TestIssueBranchNaming(t *testing.T) {
	t.Run("create branch name from issue", func(t *testing.T) {
		issue := &github.Issue{
			Number: 123,
			Title:  "Fix authentication bug in login flow",
		}

		branch := issue.CreateBranchName()

		assert.Equal(t, "issue-123-fix-authentication-bug-in-login-flow", branch)
	})

	t.Run("handle special characters", func(t *testing.T) {
		issue := &github.Issue{
			Number: 456,
			Title:  "Update API: v2.0 (breaking changes!)",
		}

		branch := issue.CreateBranchName()

		assert.Equal(t, "issue-456-update-api-v2-0-breaking-changes", branch)
		assert.NotContains(t, branch, ":")
		assert.NotContains(t, branch, "(")
		assert.NotContains(t, branch, ")")
	})

	t.Run("limit branch name length", func(t *testing.T) {
		issue := &github.Issue{
			Number: 789,
			Title:  "This is a very long issue title that should be truncated to fit within reasonable branch name length limits",
		}

		branch := issue.CreateBranchName()

		assert.LessOrEqual(t, len(branch), 60)
		assert.True(t, strings.HasPrefix(branch, "issue-789-"))
		assert.False(t, strings.HasSuffix(branch, "-"))
	})

	t.Run("handle consecutive spaces", func(t *testing.T) {
		issue := &github.Issue{
			Number: 999,
			Title:  "Fix   multiple    spaces    issue",
		}

		branch := issue.CreateBranchName()

		assert.Equal(t, "issue-999-fix-multiple-spaces-issue", branch)
		assert.NotContains(t, branch, "--")
	})
}

func TestGitHubWorkflowIntegration(t *testing.T) {
	if os.Getenv("SKIP_GH_TESTS") != "" {
		t.Skip("Skipping GitHub integration test (SKIP_GH_TESTS set)")
	}

	t.Run("full workflow: fetch issue and create branch", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		client := github.NewClient("cli/cli")
		issue, err := client.FetchIssue(1)
		require.NoError(t, err)

		branchName := issue.CreateBranchName()
		assert.NotEmpty(t, branchName)

		output := runCommand(t, tempDir, "git", "checkout", "-b", branchName)
		assert.Contains(t, output, "Switched to a new branch")

		currentBranch := strings.TrimSpace(runCommand(t, tempDir, "git", "branch", "--show-current"))
		assert.Equal(t, branchName, currentBranch)
	})

	t.Run("extract requirements from real issue", func(t *testing.T) {
		client := github.NewClient("cli/cli")
		issue, err := client.FetchIssue(1)
		require.NoError(t, err)

		reqs := issue.ExtractRequirements()
		assert.NotEmpty(t, reqs, "Should extract at least one requirement")

		for _, req := range reqs {
			assert.NotEmpty(t, req, "Requirement should not be empty")
		}
	})
}

func TestGitHubCommitWithIssueReference(t *testing.T) {
	t.Run("create commit message with issue reference", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		testFile := filepath.Join(tempDir, "fix.txt")
		err := os.WriteFile(testFile, []byte("fixed the bug"), 0644)
		require.NoError(t, err)

		runCommand(t, tempDir, "git", "add", "fix.txt")

		commitMsg := "Fix authentication bug\n\nCloses #123"
		runCommand(t, tempDir, "git", "commit", "-m", commitMsg)

		output := runCommand(t, tempDir, "git", "log", "-1", "--pretty=%B")
		assert.Contains(t, output, "Fix authentication bug")
		assert.Contains(t, output, "Closes #123")
	})
}
