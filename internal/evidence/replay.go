package evidence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileChange is one ordered filesystem mutation reconstructed from a recorded
// patch event.
type FileChange struct {
	Path        string `json:"-"`
	Type        string `json:"-"`
	Content     string `json:"-"`
	UnifiedDiff string `json:"-"`
	MovePath    string `json:"-"`
}

// ReplayPlan contains sensitive local reconstruction inputs and is explicitly
// excluded from JSON serialization.
type ReplayPlan struct {
	RepositoryRoot string       `json:"-"`
	BaseCommit     string       `json:"-"`
	Changes        []FileChange `json:"-"`
}

type replayPayload struct {
	Type    string          `json:"type"`
	TurnID  string          `json:"turn_id"`
	Success bool            `json:"success"`
	CWD     string          `json:"cwd"`
	Git     replayGit       `json:"git"`
	Changes json.RawMessage `json:"changes"`
}

type replayGit struct {
	CommitHash string `json:"commit_hash"`
}

type recordedChange struct {
	Type        string `json:"type"`
	Content     string `json:"content"`
	UnifiedDiff string `json:"unified_diff"`
	MovePath    string `json:"move_path"`
}

// ExtractReplayPlan reconstructs the ordered filesystem mutations from the
// session start through completion of targetTurn.
func ExtractReplayPlan(sessionPath, targetTurn string) (ReplayPlan, error) {
	if targetTurn == "" {
		return ReplayPlan{}, errors.New("target turn is required")
	}

	file, err := os.Open(sessionPath)
	if err != nil {
		return ReplayPlan{}, fmt.Errorf("opening session file: %w", err)
	}
	defer file.Close()

	var plan ReplayPlan
	foundTarget := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxEventBytes)
	for line := 1; scanner.Scan(); line++ {
		var event codexEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return ReplayPlan{}, fmt.Errorf("decoding session event at line %d: %w", line, err)
		}

		var payload replayPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return ReplayPlan{}, fmt.Errorf("decoding replay event at line %d: %w", line, err)
		}
		if event.Type == "session_meta" {
			plan.RepositoryRoot = payload.CWD
			plan.BaseCommit = payload.Git.CommitHash
			continue
		}
		if payload.Type == "patch_apply_end" && payload.Success {
			changes, err := decodeRecordedChanges(payload.Changes)
			if err != nil {
				return ReplayPlan{}, fmt.Errorf("decoding changes at line %d: %w", line, err)
			}
			changes, err = normalizeRecordedChanges(plan.RepositoryRoot, changes)
			if err != nil {
				return ReplayPlan{}, fmt.Errorf("normalizing changes at line %d: %w", line, err)
			}
			plan.Changes = append(plan.Changes, changes...)
		}
		if payload.Type == "task_complete" && payload.TurnID == targetTurn {
			foundTarget = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return ReplayPlan{}, fmt.Errorf("reading session file: %w", err)
	}
	if !foundTarget {
		return ReplayPlan{}, errors.New("target turn completion was not found")
	}
	if plan.RepositoryRoot == "" || plan.BaseCommit == "" {
		return ReplayPlan{}, errors.New("session is missing repository root or base commit")
	}

	return plan, nil
}

func decodeRecordedChanges(raw json.RawMessage) ([]FileChange, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("changes must be a JSON object")
	}

	var changes []FileChange
	for decoder.More() {
		pathToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		path, ok := pathToken.(string)
		if !ok {
			return nil, errors.New("change path must be a string")
		}
		var recorded recordedChange
		if err := decoder.Decode(&recorded); err != nil {
			return nil, err
		}
		changes = append(changes, FileChange{
			Path:        path,
			Type:        recorded.Type,
			Content:     recorded.Content,
			UnifiedDiff: recorded.UnifiedDiff,
			MovePath:    recorded.MovePath,
		})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return changes, nil
}

func normalizeRecordedChanges(root string, changes []FileChange) ([]FileChange, error) {
	if root == "" {
		return nil, errors.New("repository root must precede patch events")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving repository root: %w", err)
	}

	normalized := make([]FileChange, 0, len(changes))
	for _, change := range changes {
		change.Path, err = recordedRelativePath(absoluteRoot, change.Path)
		if err != nil {
			return nil, err
		}
		if change.MovePath != "" {
			change.MovePath, err = recordedRelativePath(absoluteRoot, change.MovePath)
			if err != nil {
				return nil, fmt.Errorf("normalizing move destination: %w", err)
			}
		}
		normalized = append(normalized, change)
	}
	return normalized, nil
}

func recordedRelativePath(root, recorded string) (string, error) {
	if recorded == "" {
		return "", errors.New("recorded path is empty")
	}
	if !filepath.IsAbs(recorded) {
		return filepath.Clean(recorded), nil
	}

	relative, err := filepath.Rel(root, filepath.Clean(recorded))
	if err != nil {
		return "", fmt.Errorf("relativizing recorded path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("recorded path is outside repository root")
	}
	return relative, nil
}

// ApplyChanges applies a previously validated, ordered patch sequence inside
// root. The operation rejects paths that can escape root or alter Git metadata.
func ApplyChanges(ctx context.Context, root string, changes []FileChange) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving replay root: %w", err)
	}

	for index, change := range changes {
		if err := validateChange(absoluteRoot, change); err != nil {
			return fmt.Errorf("validating change %d: %w", index+1, err)
		}
	}

	for index, change := range changes {
		if err := applyChange(ctx, absoluteRoot, change); err != nil {
			return fmt.Errorf("applying change %d: %w", index+1, err)
		}
	}
	return nil
}

func validateChange(root string, change FileChange) error {
	if _, err := safePath(root, change.Path); err != nil {
		return err
	}
	if change.MovePath != "" {
		if _, err := safePath(root, change.MovePath); err != nil {
			return fmt.Errorf("invalid move destination: %w", err)
		}
	}

	switch change.Type {
	case "add":
		if change.UnifiedDiff != "" {
			return errors.New("add change must not include a unified diff")
		}
	case "delete":
	case "update":
		if change.UnifiedDiff == "" {
			return errors.New("update change is missing a unified diff")
		}
		if err := validateDiffPaths(root, change.UnifiedDiff); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported change type %q", change.Type)
	}
	return nil
}

func applyChange(ctx context.Context, root string, change FileChange) error {
	path, err := safePath(root, change.Path)
	if err != nil {
		return err
	}

	switch change.Type {
	case "add":
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("creating parent directory: %w", err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("creating added file: %w", err)
		}
		if _, err := file.WriteString(change.Content); err != nil {
			file.Close()
			return fmt.Errorf("writing added file: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("closing added file: %w", err)
		}
	case "delete":
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("deleting file: %w", err)
		}
	case "update":
		command := exec.CommandContext(
			ctx,
			"git",
			"-C",
			root,
			"apply",
			"--whitespace=nowarn",
			"-",
		)
		command.Stdin = strings.NewReader(change.UnifiedDiff)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("applying unified diff: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		if change.MovePath != "" {
			destination, err := safePath(root, change.MovePath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				return fmt.Errorf("creating move destination directory: %w", err)
			}
			if err := os.Rename(path, destination); err != nil {
				return fmt.Errorf("moving updated file: %w", err)
			}
		}
	}
	return nil
}

func safePath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("path must be a non-empty relative path")
	}

	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes replay root")
	}
	if clean == ".git" || strings.HasPrefix(clean, ".git"+string(filepath.Separator)) {
		return "", errors.New("Git metadata paths are not allowed")
	}

	target := filepath.Join(root, clean)
	relativeToRoot, err := filepath.Rel(root, target)
	if err != nil || relativeToRoot == ".." ||
		strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes replay root")
	}

	current := root
	for _, component := range strings.Split(filepath.Dir(clean), string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("inspecting path component: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlinked path components are not allowed")
		}
	}

	return target, nil
}

func validateDiffPaths(root, diff string) error {
	for line := range strings.SplitSeq(diff, "\n") {
		if line == "new file mode 120000" || line == "new mode 120000" {
			return errors.New("unified diff may not create symlinks")
		}
		if !strings.HasPrefix(line, "--- ") && !strings.HasPrefix(line, "+++ ") {
			continue
		}
		path := strings.TrimSpace(line[4:])
		path = strings.SplitN(path, "\t", 2)[0]
		if path == "/dev/null" {
			continue
		}
		path = strings.TrimPrefix(path, "a/")
		path = strings.TrimPrefix(path, "b/")
		if _, err := safePath(root, path); err != nil {
			return fmt.Errorf("unsafe unified diff path: %w", err)
		}
	}
	return nil
}
