package evidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxEventBytes = 16 << 20

var verificationCommand = regexp.MustCompile(
	`(?i)(^|[[:space:];|&])(go test|go vet|make verify|mix test|mix credo|mix format|` +
		`npm test|npm run lint|pytest|ruff check|cargo test|bundle exec|golangci-lint|` +
		`staticcheck|eslint|tsc)([[:space:];|&]|$)`,
)

// CodexSummary contains aggregate evidence counts only. It intentionally
// excludes prompts, outputs, paths, repository URLs, commit hashes, commands,
// session identifiers, and other potentially sensitive values.
type CodexSummary struct {
	Sessions                int `json:"sessions"`
	SessionsWithGit         int `json:"sessions_with_git"`
	Commands                int `json:"commands"`
	SuccessfulCommands      int `json:"successful_commands"`
	FailedCommands          int `json:"failed_commands"`
	VerificationCommands    int `json:"verification_commands"`
	SuccessfulVerifications int `json:"successful_verifications"`
	Patches                 int `json:"patches"`
	CompletedTasks          int `json:"completed_tasks"`
	Level3CandidateTurns    int `json:"level3_candidate_turns"`
}

type codexEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type sessionEvidence struct {
	hasMetadata bool
	hasGit      bool
	turns       map[string]*turnEvidence
}

type turnEvidence struct {
	hasSuccessfulPatch  bool
	hasSuccessfulVerify bool
	hasCompletion       bool
}

// ScanCodex reads Codex JSONL session files and returns content-free aggregate
// evidence metrics. It never returns or persists raw event values.
func ScanCodex(root string) (CodexSummary, error) {
	var summary CodexSummary

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		session, err := scanCodexSession(path, &summary)
		if err != nil {
			return err
		}
		if session.hasMetadata {
			summary.Sessions++
		}
		if session.hasGit {
			summary.SessionsWithGit++
		}
		if session.hasGit {
			for _, turn := range session.turns {
				if turn.hasSuccessfulPatch &&
					turn.hasSuccessfulVerify &&
					turn.hasCompletion {
					summary.Level3CandidateTurns++
				}
			}
		}
		return nil
	})
	if err != nil {
		return CodexSummary{}, fmt.Errorf("scanning Codex sessions: %w", err)
	}

	return summary, nil
}

func scanCodexSession(path string, summary *CodexSummary) (sessionEvidence, error) {
	file, err := os.Open(path)
	if err != nil {
		return sessionEvidence{}, fmt.Errorf("opening session file: %w", err)
	}
	defer file.Close()

	session := sessionEvidence{turns: make(map[string]*turnEvidence)}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxEventBytes)

	for line := 1; scanner.Scan(); line++ {
		var event codexEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return sessionEvidence{}, fmt.Errorf("decoding session event at line %d: %w", line, err)
		}
		if err := countCodexEvent(event, summary, &session); err != nil {
			return sessionEvidence{}, fmt.Errorf("reading session event at line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return sessionEvidence{}, fmt.Errorf("reading session file: %w", err)
	}

	return session, nil
}

func countCodexEvent(event codexEvent, summary *CodexSummary, session *sessionEvidence) error {
	var payload map[string]any
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
	}

	if event.Type == "session_meta" {
		session.hasMetadata = true
		_, session.hasGit = payload["git"].(map[string]any)
		return nil
	}

	eventType, _ := payload["type"].(string)
	turn := session.turn(payload)
	switch eventType {
	case "exec_command_end":
		countCommand(payload, summary, turn)
	case "patch_apply_end":
		if success, _ := payload["success"].(bool); success {
			summary.Patches++
			if turn != nil {
				turn.hasSuccessfulPatch = true
			}
		}
	case "task_complete":
		summary.CompletedTasks++
		if turn != nil {
			turn.hasCompletion = true
		}
	}
	return nil
}

func countCommand(payload map[string]any, summary *CodexSummary, turn *turnEvidence) {
	summary.Commands++

	exitCode, hasExitCode := numberAsInt(payload["exit_code"])
	if hasExitCode && exitCode == 0 {
		summary.SuccessfulCommands++
	} else {
		summary.FailedCommands++
	}

	command := commandText(payload["command"])
	if !verificationCommand.MatchString(command) {
		return
	}

	summary.VerificationCommands++
	if hasExitCode && exitCode == 0 {
		summary.SuccessfulVerifications++
		if turn != nil {
			turn.hasSuccessfulVerify = true
		}
	}
}

func (session *sessionEvidence) turn(payload map[string]any) *turnEvidence {
	turnID, _ := payload["turn_id"].(string)
	if turnID == "" {
		return nil
	}
	if session.turns[turnID] == nil {
		session.turns[turnID] = &turnEvidence{}
	}
	return session.turns[turnID]
}

func commandText(value any) string {
	switch command := value.(type) {
	case string:
		return command
	case []any:
		parts := make([]string, 0, len(command))
		for _, part := range command {
			if text, ok := part.(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func numberAsInt(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok {
		return 0, false
	}
	return int(number), true
}
