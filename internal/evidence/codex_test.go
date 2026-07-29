package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCodexSummarizesEvidenceWithoutLeakingContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSession(t, root, "candidate.jsonl", []map[string]any{
		event("session_meta", map[string]any{
			"id":  "session-sensitive-id",
			"cwd": "/private/customer/repository",
			"git": map[string]any{
				"branch":         "private-branch",
				"commit_hash":    "abc123",
				"repository_url": "git@example.com:customer/private.git",
			},
		}),
		event("event_msg", map[string]any{
			"type":      "exec_command_end",
			"turn_id":   "turn-1",
			"command":   []string{"/bin/zsh", "-lc", "go test ./..."},
			"exit_code": 0,
			"stdout":    "SECRET_OUTPUT",
		}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "turn-1",
			"success": true,
			"changes": []any{map[string]any{"path": "secret.go"}},
		}),
		event("event_msg", map[string]any{"type": "task_complete", "turn_id": "turn-1"}),
	})
	writeSession(t, root, "incomplete.jsonl", []map[string]any{
		event("session_meta", map[string]any{
			"id":  "another-sensitive-id",
			"cwd": "/private/other",
		}),
		event("event_msg", map[string]any{
			"type":      "exec_command_end",
			"command":   "rm -rf build",
			"exit_code": 1,
			"stderr":    "SECRET_ERROR",
		}),
	})
	writeSession(t, root, "uncorrelated.jsonl", []map[string]any{
		event("session_meta", map[string]any{
			"id":  "uncorrelated-sensitive-id",
			"cwd": "/private/uncorrelated",
			"git": map[string]any{"commit_hash": "def456"},
		}),
		event("event_msg", map[string]any{
			"type":    "patch_apply_end",
			"turn_id": "turn-a",
			"success": true,
		}),
		event("event_msg", map[string]any{
			"type":      "exec_command_end",
			"turn_id":   "turn-b",
			"command":   "go test ./...",
			"exit_code": 0,
		}),
		event("event_msg", map[string]any{"type": "task_complete", "turn_id": "turn-c"}),
	})

	summary, err := ScanCodex(root)
	if err != nil {
		t.Fatalf("ScanCodex() error = %v", err)
	}

	if summary.Sessions != 3 {
		t.Errorf("Sessions = %d, want 3", summary.Sessions)
	}
	if summary.SessionsWithGit != 2 {
		t.Errorf("SessionsWithGit = %d, want 2", summary.SessionsWithGit)
	}
	if summary.Commands != 3 || summary.SuccessfulCommands != 2 || summary.FailedCommands != 1 {
		t.Errorf(
			"command counts = (%d, %d, %d), want (3, 2, 1)",
			summary.Commands,
			summary.SuccessfulCommands,
			summary.FailedCommands,
		)
	}
	if summary.VerificationCommands != 2 || summary.SuccessfulVerifications != 2 {
		t.Errorf(
			"verification counts = (%d, %d), want (2, 2)",
			summary.VerificationCommands,
			summary.SuccessfulVerifications,
		)
	}
	if summary.Patches != 2 || summary.CompletedTasks != 2 {
		t.Errorf("result counts = (%d, %d), want (2, 2)", summary.Patches, summary.CompletedTasks)
	}
	if summary.Level3CandidateTurns != 1 {
		t.Errorf("Level3CandidateTurns = %d, want 1", summary.Level3CandidateTurns)
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, secret := range []string{
		"customer",
		"private-branch",
		"abc123",
		"SECRET_OUTPUT",
		"SECRET_ERROR",
		"session-sensitive-id",
	} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("summary leaked sensitive value %q", secret)
		}
	}
}

func TestScanCodexRejectsMalformedJSONL(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.jsonl"), []byte("{broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ScanCodex(root); err == nil {
		t.Fatal("ScanCodex() error = nil, want malformed JSON error")
	}
}

func event(kind string, payload map[string]any) map[string]any {
	return map[string]any{
		"type":    kind,
		"payload": payload,
	}
}

func writeSession(t *testing.T, root, name string, events []map[string]any) {
	t.Helper()

	var lines []byte
	for _, item := range events {
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, encoded...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(filepath.Join(root, name), lines, 0o600); err != nil {
		t.Fatal(err)
	}
}
