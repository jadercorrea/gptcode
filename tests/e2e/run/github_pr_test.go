//go:build e2e

package run_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubBranchOperations(t *testing.T) {
	t.Run("create new branch", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CreateBranch("feature-test", "master")
		require.NoError(t, err)

		currentBranch := strings.TrimSpace(runCommand(t, tempDir, "git", "branch", "--show-current"))
		assert.Equal(t, "feature-test", currentBranch)
	})

	t.Run("checkout existing branch", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CreateBranch("existing-branch", "master")
		require.NoError(t, err)

		runCommand(t, tempDir, "git", "checkout", "master")

		err = client.CreateBranch("existing-branch", "master")
		require.NoError(t, err)

		currentBranch := strings.TrimSpace(runCommand(t, tempDir, "git", "branch", "--show-current"))
		assert.Equal(t, "existing-branch", currentBranch)
	})

	t.Run("default to main branch", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		runCommand(t, tempDir, "git", "branch", "-m", "master", "main")

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CreateBranch("feature-from-main", "")
		require.NoError(t, err)

		currentBranch := strings.TrimSpace(runCommand(t, tempDir, "git", "branch", "--show-current"))
		assert.Equal(t, "feature-from-main", currentBranch)
	})
}

func TestGitHubCommitOperations(t *testing.T) {
	t.Run("commit specific files", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		file1 := filepath.Join(tempDir, "file1.txt")
		file2 := filepath.Join(tempDir, "file2.txt")

		require.NoError(t, os.WriteFile(file1, []byte("content 1"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("content 2"), 0644))

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CommitChanges(github.CommitOptions{
			Message:   "Add files",
			FilePaths: []string{"file1.txt", "file2.txt"},
		})
		require.NoError(t, err)

		output := runCommand(t, tempDir, "git", "log", "-1", "--pretty=%B")
		assert.Contains(t, output, "Add files")
	})

	t.Run("commit all files", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		file1 := filepath.Join(tempDir, "file1.txt")
		file2 := filepath.Join(tempDir, "file2.txt")

		require.NoError(t, os.WriteFile(file1, []byte("content 1"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("content 2"), 0644))

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CommitChanges(github.CommitOptions{
			Message:  "Add all files",
			AllFiles: true,
		})
		require.NoError(t, err)

		output := runCommand(t, tempDir, "git", "log", "-1", "--pretty=%B")
		assert.Contains(t, output, "Add all files")
	})

	t.Run("commit with issue reference", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		file := filepath.Join(tempDir, "fix.txt")
		require.NoError(t, os.WriteFile(file, []byte("fixed"), 0644))

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CommitChanges(github.CommitOptions{
			Message:     "Fix bug",
			IssueNumber: 42,
			FilePaths:   []string{"fix.txt"},
		})
		require.NoError(t, err)

		output := runCommand(t, tempDir, "git", "log", "-1", "--pretty=%B")
		assert.Contains(t, output, "Fix bug")
		assert.Contains(t, output, "Closes #42")
	})

	t.Run("nothing to commit", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		client := github.NewClient("test/repo")
		client.SetWorkDir(tempDir)

		err := client.CommitChanges(github.CommitOptions{
			Message:  "Empty commit",
			AllFiles: true,
		})
		require.NoError(t, err)
	})
}

func TestPRBodyGeneration(t *testing.T) {
	t.Run("generate PR body with changes", func(t *testing.T) {
		issue := &github.Issue{
			Number: 123,
			Title:  "Fix authentication bug",
			Body:   "The auth system has a bug",
		}

		changes := []string{
			"Updated token validation logic",
			"Added expiry check",
			"Fixed refresh token handling",
		}

		body := github.GeneratePRBody(issue, changes)

		assert.Contains(t, body, "Closes #123")
		assert.Contains(t, body, "## Changes")
		assert.Contains(t, body, "Updated token validation logic")
		assert.Contains(t, body, "Added expiry check")
		assert.Contains(t, body, "Fixed refresh token handling")
	})

	t.Run("generate PR body with requirements", func(t *testing.T) {
		issue := &github.Issue{
			Number: 456,
			Title:  "Add feature",
			Body: `Requirements:
- [ ] Implement feature A
- [ ] Add tests
- [ ] Update documentation`,
		}

		changes := []string{
			"Implemented feature A",
			"Added comprehensive tests",
		}

		body := github.GeneratePRBody(issue, changes)

		assert.Contains(t, body, "Closes #456")
		assert.Contains(t, body, "## Requirements Addressed")
		assert.Contains(t, body, "Implement feature A")
		assert.Contains(t, body, "Add tests")
		assert.Contains(t, body, "Update documentation")
	})
}

func TestGitHubFullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_GH_TESTS") != "" {
		t.Skip("Skipping GitHub integration test (SKIP_GH_TESTS set)")
	}

	t.Run("full workflow: issue to branch", func(t *testing.T) {
		tempDir := t.TempDir()
		setupGitRepo(t, tempDir)

		ghClient := github.NewClient("cli/cli")
		ghClient.SetWorkDir(tempDir)

		issue, err := ghClient.FetchIssue(1)
		require.NoError(t, err)

		branchName := issue.CreateBranchName()
		require.NotEmpty(t, branchName)

		err = ghClient.CreateBranch(branchName, "master")
		require.NoError(t, err)

		currentBranch := strings.TrimSpace(runCommand(t, tempDir, "git", "branch", "--show-current"))
		assert.Equal(t, branchName, currentBranch)

		file := filepath.Join(tempDir, "fix.go")
		err = os.WriteFile(file, []byte("package main\n\nfunc fix() {}\n"), 0644)
		require.NoError(t, err)

		err = ghClient.CommitChanges(github.CommitOptions{
			Message:     fmt.Sprintf("Fix: %s", issue.Title),
			IssueNumber: issue.Number,
			FilePaths:   []string{"fix.go"},
		})
		require.NoError(t, err)

		output := runCommand(t, tempDir, "git", "log", "-1", "--pretty=%B")
		assert.Contains(t, output, issue.Title)
		assert.Contains(t, output, fmt.Sprintf("Closes #%d", issue.Number))
	})
}
