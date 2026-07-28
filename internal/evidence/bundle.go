package evidence

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const EvidenceSchemaVersion = "gptcode.evidence/v1"

type Command struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

type Experiment struct {
	ID            string        `json:"id"`
	Repository    string        `json:"repository"`
	Output        string        `json:"output"`
	Agent         Command       `json:"agent"`
	Verifications []Command     `json:"verifications"`
	AgentTimeout  time.Duration `json:"-"`
	Progress      io.Writer     `json:"-"`
}

type Snapshot struct {
	FileCount int    `json:"file_count"`
	SHA256    string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion   string    `json:"schema_version"`
	ExperimentID    string    `json:"experiment_id"`
	BaseCommit      string    `json:"base_commit"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	InitialSnapshot Snapshot  `json:"initial_snapshot"`
	FinalSnapshot   Snapshot  `json:"final_snapshot"`
}

type CommandResult struct {
	Name      string        `json:"name"`
	Args      []string      `json:"args"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	ExitCode  int           `json:"exit_code"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
}

type VerificationReport struct {
	Passed   bool            `json:"passed"`
	Commands []CommandResult `json:"commands"`
}

type ExperimentResult struct {
	Passed   bool
	TimedOut bool
}

type bundleEvent struct {
	Type string    `json:"type"`
	At   time.Time `json:"at"`
}

// RunExperiment executes one agent command and its deterministic checks in a
// clean Git fixture, preserving enough state to inspect and replay the run.
func RunExperiment(ctx context.Context, experiment Experiment) (ExperimentResult, error) {
	if err := validateExperiment(experiment); err != nil {
		return ExperimentResult{}, err
	}

	baseCommit, err := gitOutput(ctx, experiment.Repository, "rev-parse", "HEAD")
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("reading base commit: %w", err)
	}
	status, err := gitOutput(ctx, experiment.Repository, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("checking repository state: %w", err)
	}
	if status != "" {
		return ExperimentResult{}, errors.New("repository must be clean before an experiment")
	}
	if _, err := os.Stat(experiment.Output); err == nil {
		return ExperimentResult{}, errors.New("evidence directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return ExperimentResult{}, fmt.Errorf("checking evidence directory: %w", err)
	}
	if err := os.Mkdir(experiment.Output, 0o700); err != nil {
		return ExperimentResult{}, fmt.Errorf("creating evidence directory: %w", err)
	}

	started := time.Now().UTC()
	events, err := os.Create(filepath.Join(experiment.Output, "events.jsonl"))
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("creating event log: %w", err)
	}
	defer events.Close()
	eventEncoder := json.NewEncoder(events)

	initial, err := writeSnapshot(experiment.Repository, filepath.Join(experiment.Output, "initial.tar"))
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("capturing initial snapshot: %w", err)
	}
	if err := writeGitDiff(ctx, experiment.Repository, filepath.Join(experiment.Output, "initial.patch")); err != nil {
		return ExperimentResult{}, err
	}
	if err := eventEncoder.Encode(bundleEvent{Type: "agent_started", At: time.Now().UTC()}); err != nil {
		return ExperimentResult{}, fmt.Errorf("recording agent start: %w", err)
	}

	agentContext := ctx
	cancelAgent := func() {}
	if experiment.AgentTimeout > 0 {
		agentContext, cancelAgent = context.WithTimeout(ctx, experiment.AgentTimeout)
	}
	var progress []io.Writer
	if experiment.Progress != nil {
		progress = append(progress, experiment.Progress)
	}
	agentResult := runCommand(agentContext, experiment.Repository, experiment.Agent, progress...)
	timedOut := errors.Is(agentContext.Err(), context.DeadlineExceeded)
	cancelAgent()
	if err := appendJSONLine(filepath.Join(experiment.Output, "commands.jsonl"), agentResult); err != nil {
		return ExperimentResult{}, err
	}
	if err := eventEncoder.Encode(bundleEvent{Type: "agent_completed", At: time.Now().UTC()}); err != nil {
		return ExperimentResult{}, fmt.Errorf("recording agent completion: %w", err)
	}
	if err := writeGitDiff(ctx, experiment.Repository, filepath.Join(experiment.Output, "agent.patch")); err != nil {
		return ExperimentResult{}, err
	}

	verification := VerificationReport{Passed: agentResult.ExitCode == 0}
	for _, check := range experiment.Verifications {
		result := runCommand(ctx, experiment.Repository, check)
		verification.Commands = append(verification.Commands, result)
		if result.ExitCode != 0 {
			verification.Passed = false
		}
		if err := appendJSONLine(filepath.Join(experiment.Output, "commands.jsonl"), result); err != nil {
			return ExperimentResult{}, err
		}
	}
	if err := writeJSON(filepath.Join(experiment.Output, "verification.json"), verification); err != nil {
		return ExperimentResult{}, err
	}
	if err := eventEncoder.Encode(bundleEvent{Type: "verification_completed", At: time.Now().UTC()}); err != nil {
		return ExperimentResult{}, fmt.Errorf("recording verification completion: %w", err)
	}

	finalSnapshot, err := writeSnapshot(experiment.Repository, filepath.Join(experiment.Output, "final.tar"))
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("capturing final snapshot: %w", err)
	}
	if err := writeGitDiff(ctx, experiment.Repository, filepath.Join(experiment.Output, "final.patch")); err != nil {
		return ExperimentResult{}, err
	}

	manifest := Manifest{
		SchemaVersion:   EvidenceSchemaVersion,
		ExperimentID:    experiment.ID,
		BaseCommit:      strings.TrimSpace(baseCommit),
		StartedAt:       started,
		CompletedAt:     time.Now().UTC(),
		InitialSnapshot: initial,
		FinalSnapshot:   finalSnapshot,
	}
	if err := writeJSON(filepath.Join(experiment.Output, "manifest.json"), manifest); err != nil {
		return ExperimentResult{}, err
	}
	if err := writeReport(filepath.Join(experiment.Output, "report.md"), manifest, agentResult, verification); err != nil {
		return ExperimentResult{}, err
	}

	return ExperimentResult{Passed: verification.Passed, TimedOut: timedOut}, nil
}

func validateExperiment(experiment Experiment) error {
	if experiment.ID == "" {
		return errors.New("experiment ID is required")
	}
	if experiment.Repository == "" || experiment.Output == "" {
		return errors.New("repository and output paths are required")
	}
	if len(experiment.Agent.Args) == 0 || experiment.Agent.Name == "" {
		return errors.New("agent name and executable are required")
	}
	if len(experiment.Verifications) == 0 {
		return errors.New("at least one deterministic verification is required")
	}
	repository, err := filepath.Abs(experiment.Repository)
	if err != nil {
		return fmt.Errorf("resolving repository: %w", err)
	}
	output, err := filepath.Abs(experiment.Output)
	if err != nil {
		return fmt.Errorf("resolving output: %w", err)
	}
	relative, err := filepath.Rel(repository, output)
	if err != nil {
		return fmt.Errorf("comparing repository and output: %w", err)
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("evidence output must be outside the evaluated repository")
	}
	for _, command := range experiment.Verifications {
		if command.Name == "" || len(command.Args) == 0 {
			return errors.New("verification name and executable are required")
		}
	}
	return nil
}

func runCommand(ctx context.Context, directory string, command Command, mirrors ...io.Writer) CommandResult {
	started := time.Now().UTC()
	process := exec.CommandContext(ctx, command.Args[0], command.Args[1:]...)
	process.Dir = directory
	var stdout, stderr strings.Builder
	stdoutWriters := []io.Writer{&stdout}
	stderrWriters := []io.Writer{&stderr}
	if len(mirrors) > 0 {
		progress := &synchronizedWriter{writer: io.MultiWriter(mirrors...)}
		stdoutWriters = append(stdoutWriters, progress)
		stderrWriters = append(stderrWriters, progress)
	}
	process.Stdout = io.MultiWriter(stdoutWriters...)
	process.Stderr = io.MultiWriter(stderrWriters...)
	err := process.Run()

	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
	}
	return CommandResult{
		Name:      command.Name,
		Args:      append([]string(nil), command.Args...),
		StartedAt: started,
		Duration:  time.Since(started),
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
	}
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(content []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(content)
}

func writeSnapshot(root, destination string) (Snapshot, error) {
	paths, err := snapshotPaths(root)
	if err != nil {
		return Snapshot{}, err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Snapshot{}, err
	}
	hash := sha256.New()
	writer := tar.NewWriter(io.MultiWriter(file, hash))
	for _, relative := range paths {
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if err != nil {
			writer.Close()
			file.Close()
			return Snapshot{}, err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			writer.Close()
			file.Close()
			return Snapshot{}, err
		}
		header.Name = filepath.ToSlash(relative)
		header.ModTime = time.Unix(0, 0)
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "", ""
		if err := writer.WriteHeader(header); err != nil {
			writer.Close()
			file.Close()
			return Snapshot{}, err
		}
		if info.Mode().IsRegular() {
			source, err := os.Open(path)
			if err != nil {
				writer.Close()
				file.Close()
				return Snapshot{}, err
			}
			_, copyErr := io.Copy(writer, source)
			closeErr := source.Close()
			if copyErr != nil {
				writer.Close()
				file.Close()
				return Snapshot{}, copyErr
			}
			if closeErr != nil {
				writer.Close()
				file.Close()
				return Snapshot{}, closeErr
			}
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return Snapshot{}, err
	}
	if err := file.Close(); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{FileCount: len(paths), SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func snapshotPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if relative == ".git" {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("unsupported snapshot file type at %q", relative)
		}
		paths = append(paths, relative)
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

// RestoreSnapshot safely restores a bundle snapshot into an empty directory.
func RestoreSnapshot(snapshotPath, destination string) error {
	file, err := os.Open(snapshotPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(bufio.NewReader(file))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		path, err := safePath(destination, filepath.FromSlash(header.Name))
		if err != nil {
			return fmt.Errorf("unsafe snapshot entry: %w", err)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, header.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported snapshot entry type %d", header.Typeflag)
		}
	}
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = root
	command.Env = isolatedGitEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func isolatedGitEnvironment() []string {
	blocked := map[string]struct{}{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
		"GIT_COMMON_DIR":                   {},
		"GIT_DIR":                          {},
		"GIT_INDEX_FILE":                   {},
		"GIT_OBJECT_DIRECTORY":             {},
		"GIT_PREFIX":                       {},
		"GIT_WORK_TREE":                    {},
	}
	environment := make([]string, 0, len(os.Environ()))
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if _, found := blocked[name]; !found {
			environment = append(environment, variable)
		}
	}
	return environment
}

func writeGitDiff(ctx context.Context, root, destination string) error {
	diff, err := gitOutput(ctx, root, "diff", "--binary", "--no-ext-diff", "HEAD")
	if err != nil {
		return fmt.Errorf("capturing Git diff: %w", err)
	}
	if diff != "" {
		diff += "\n"
	}
	if err := os.WriteFile(destination, []byte(diff), 0o600); err != nil {
		return fmt.Errorf("writing Git diff: %w", err)
	}
	return nil
}

func appendJSONLine(path string, value any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening command log: %w", err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(value); err != nil {
		return fmt.Errorf("writing command log: %w", err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func writeReport(path string, manifest Manifest, agent CommandResult, verification VerificationReport) error {
	status := "FAILED"
	if verification.Passed {
		status = "PASSED"
	}
	report := fmt.Sprintf(
		"# Evidence report: %s\n\n"+
			"- Result: **%s**\n"+
			"- Base commit: `%s`\n"+
			"- Agent exit code: `%d`\n"+
			"- Verification commands: `%d`\n"+
			"- Initial snapshot: `%s`\n"+
			"- Final snapshot: `%s`\n",
		manifest.ExperimentID,
		status,
		manifest.BaseCommit,
		agent.ExitCode,
		len(verification.Commands),
		manifest.InitialSnapshot.SHA256,
		manifest.FinalSnapshot.SHA256,
	)
	return os.WriteFile(path, []byte(report), 0o600)
}
