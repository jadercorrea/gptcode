package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractReplayPlanIncludesOrderedChangesThroughTargetTurn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "session.jsonl")
	writeSession(t, root, "session.jsonl", []map[string]any{
		event("session_meta", map[string]any{
			"cwd": "/private/repository",
			"git": map[string]any{"commit_hash": "base123"},
		}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "prior",
			"success": true,
			"changes": map[string]any{
				"/private/repository/first.txt": map[string]any{
					"type": "add", "content": "first\n",
				},
			},
		}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "target",
			"success": true,
			"changes": map[string]any{
				"/private/repository/second.txt": map[string]any{
					"type": "add", "content": "second\n",
				},
			},
		}),
		event("event_msg", map[string]any{"type": "task_complete", "turn_id": "target"}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "later",
			"success": true,
			"changes": map[string]any{
				"later.txt": map[string]any{"type": "add", "content": "later\n"},
			},
		}),
	})

	plan, err := ExtractReplayPlan(sessionPath, "target")
	if err != nil {
		t.Fatalf("ExtractReplayPlan() error = %v", err)
	}

	if plan.BaseCommit != "base123" {
		t.Errorf("BaseCommit = %q, want base123", plan.BaseCommit)
	}
	if plan.RepositoryRoot != "/private/repository" {
		t.Errorf("RepositoryRoot = %q, want /private/repository", plan.RepositoryRoot)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("len(Changes) = %d, want 2", len(plan.Changes))
	}
	if plan.Changes[0].Path != "first.txt" || plan.Changes[1].Path != "second.txt" {
		t.Errorf("change order = %q, %q", plan.Changes[0].Path, plan.Changes[1].Path)
	}

	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("encoded plan leaked reconstruction inputs: %s", encoded)
	}
}

func TestExtractReplayPlanRejectsRecordedPathOutsideRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "session.jsonl")
	writeSession(t, root, "session.jsonl", []map[string]any{
		event("session_meta", map[string]any{
			"cwd": "/private/repository",
			"git": map[string]any{"commit_hash": "base123"},
		}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "target",
			"success": true,
			"changes": map[string]any{
				"/private/other/escape.txt": map[string]any{
					"type": "add", "content": "escape\n",
				},
			},
		}),
		event("event_msg", map[string]any{"type": "task_complete", "turn_id": "target"}),
	})

	if _, err := ExtractReplayPlan(sessionPath, "target"); err == nil {
		t.Fatal("ExtractReplayPlan() error = nil, want outside-repository rejection")
	}
}

func TestApplyChangesReconstructsOrderedFileState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "obsolete.txt"), []byte("remove\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	changes := []FileChange{
		{
			Path: "existing.txt",
			Type: "update",
			UnifiedDiff: "diff --git a/existing.txt b/existing.txt\n" +
				"--- a/existing.txt\n" +
				"+++ b/existing.txt\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
		},
		{Path: "nested/added.txt", Type: "add", Content: "created\n"},
		{Path: "obsolete.txt", Type: "delete"},
	}

	if err := ApplyChanges(context.Background(), root, changes); err != nil {
		t.Fatalf("ApplyChanges() error = %v", err)
	}

	assertFileContent(t, filepath.Join(root, "existing.txt"), "new\n")
	assertFileContent(t, filepath.Join(root, "nested/added.txt"), "created\n")
	if _, err := os.Stat(filepath.Join(root, "obsolete.txt")); !os.IsNotExist(err) {
		t.Fatalf("obsolete file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestApplyChangesRejectsUnsafePathsBeforeWriting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sentinel := filepath.Join(root, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	changes := []FileChange{
		{Path: "safe.txt", Type: "add", Content: "must not be written\n"},
		{Path: "../escape.txt", Type: "add", Content: "escaped\n"},
	}

	if err := ApplyChanges(context.Background(), root, changes); err == nil {
		t.Fatal("ApplyChanges() error = nil, want unsafe path rejection")
	}
	if _, err := os.Stat(filepath.Join(root, "safe.txt")); !os.IsNotExist(err) {
		t.Fatalf("safe change was applied before validation completed: %v", err)
	}
	assertFileContent(t, sentinel, "unchanged\n")
}

func TestApplyChangesRejectsGitMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := ApplyChanges(context.Background(), root, []FileChange{
		{Path: ".git/config", Type: "add", Content: "malicious\n"},
	})
	if err == nil {
		t.Fatal("ApplyChanges() error = nil, want .git path rejection")
	}
}

func TestApplyChangesRejectsDiffThatCreatesSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := ApplyChanges(context.Background(), root, []FileChange{
		{
			Path: "escape",
			Type: "update",
			UnifiedDiff: "diff --git a/escape b/escape\n" +
				"new file mode 120000\n" +
				"--- /dev/null\n" +
				"+++ b/escape\n" +
				"@@ -0,0 +1 @@\n" +
				"+../../outside\n",
		},
	})
	if err == nil {
		t.Fatal("ApplyChanges() error = nil, want symlink creation rejection")
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Errorf("%s content = %q, want %q", filepath.Base(path), content, want)
	}
}
