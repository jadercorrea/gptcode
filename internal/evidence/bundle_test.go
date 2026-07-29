package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandMirrorsOutputWhilePreservingEvidence(t *testing.T) {
	t.Parallel()

	var progress bytes.Buffer
	result := runCommand(
		context.Background(),
		t.TempDir(),
		Command{
			Name: "visible-agent",
			Args: []string{"/bin/sh", "-c", "printf 'stage: analyze\\n'; printf 'stage: verify\\n' >&2"},
		},
		&progress,
	)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "stage: analyze\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if result.Stderr != "stage: verify\n" {
		t.Fatalf("stderr = %q", result.Stderr)
	}
	if got := progress.String(); !strings.Contains(got, "stage: analyze") || !strings.Contains(got, "stage: verify") {
		t.Fatalf("mirrored output = %q", got)
	}
}

func TestRunExperimentProducesReplayableEvidenceBundle(t *testing.T) {
	t.Parallel()

	repository := newFixtureRepository(t)
	output := filepath.Join(t.TempDir(), "evidence")

	result, err := RunExperiment(context.Background(), Experiment{
		ID:         "fix-greeting",
		Repository: repository,
		Output:     output,
		Agent: Command{
			Name: "fixture-agent",
			Args: []string{"/bin/sh", "-c", "printf 'hello, evidence\\n' > greeting.txt"},
		},
		Verifications: []Command{
			{
				Name: "greeting-test",
				Args: []string{"/bin/sh", "-c", "test \"$(cat greeting.txt)\" = 'hello, evidence'"},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunExperiment() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("RunExperiment() Passed = false, want true")
	}

	for _, name := range []string{
		"manifest.json",
		"initial.tar",
		"initial.patch",
		"events.jsonl",
		"commands.jsonl",
		"agent.patch",
		"verification.json",
		"final.tar",
		"final.patch",
		"report.md",
	} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("%s was not created: %v", name, err)
		}
	}

	var manifest Manifest
	readJSONFile(t, filepath.Join(output, "manifest.json"), &manifest)
	if manifest.SchemaVersion != EvidenceSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", manifest.SchemaVersion, EvidenceSchemaVersion)
	}
	if manifest.InitialSnapshot.SHA256 == "" || manifest.FinalSnapshot.SHA256 == "" {
		t.Error("snapshot hashes must be recorded")
	}
	if manifest.InitialSnapshot.SHA256 == manifest.FinalSnapshot.SHA256 {
		t.Error("initial and final snapshot hashes unexpectedly match")
	}

	replay := t.TempDir()
	if err := RestoreSnapshot(filepath.Join(output, "final.tar"), replay); err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(replay, "greeting.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello, evidence\n" {
		t.Errorf("replayed greeting = %q", content)
	}
}

func TestRunExperimentRejectsDirtyRepository(t *testing.T) {
	t.Parallel()

	repository := newFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "dirty.txt"), []byte("not captured\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := RunExperiment(context.Background(), Experiment{
		ID:         "dirty",
		Repository: repository,
		Output:     filepath.Join(t.TempDir(), "evidence"),
		Agent:      Command{Name: "noop", Args: []string{"/usr/bin/true"}},
	})
	if err == nil {
		t.Fatal("RunExperiment() error = nil, want dirty repository rejection")
	}
}

func TestRunExperimentRejectsExistingEvidenceDirectory(t *testing.T) {
	t.Parallel()

	repository := newFixtureRepository(t)
	output := filepath.Join(t.TempDir(), "evidence")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := RunExperiment(context.Background(), Experiment{
		ID:         "existing-output",
		Repository: repository,
		Output:     output,
		Agent:      Command{Name: "noop", Args: []string{"/usr/bin/true"}},
	})
	if err == nil {
		t.Fatal("RunExperiment() error = nil, want existing output rejection")
	}
}

func TestRunExperimentRequiresDeterministicVerification(t *testing.T) {
	t.Parallel()

	repository := newFixtureRepository(t)
	_, err := RunExperiment(context.Background(), Experiment{
		ID:         "no-verification",
		Repository: repository,
		Output:     filepath.Join(t.TempDir(), "evidence"),
		Agent:      Command{Name: "noop", Args: []string{"/usr/bin/true"}},
	})
	if err == nil {
		t.Fatal("RunExperiment() error = nil, want missing verification rejection")
	}
}

func TestRunExperimentRecordsFailedVerificationWithoutDiscardingEvidence(t *testing.T) {
	t.Parallel()

	repository := newFixtureRepository(t)
	output := filepath.Join(t.TempDir(), "evidence")

	result, err := RunExperiment(context.Background(), Experiment{
		ID:         "failed-check",
		Repository: repository,
		Output:     output,
		Agent:      Command{Name: "noop", Args: []string{"/usr/bin/true"}},
		Verifications: []Command{
			{Name: "expected-failure", Args: []string{"/usr/bin/false"}},
		},
	})
	if err != nil {
		t.Fatalf("RunExperiment() error = %v", err)
	}
	if result.Passed {
		t.Fatal("RunExperiment() Passed = true, want false")
	}

	var verification VerificationReport
	readJSONFile(t, filepath.Join(output, "verification.json"), &verification)
	if verification.Passed || len(verification.Commands) != 1 || verification.Commands[0].ExitCode == 0 {
		t.Fatalf("unexpected verification report: %+v", verification)
	}
}

func newFixtureRepository(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runTestCommand(t, root, "git", "init", "-q")
	runTestCommand(t, root, "git", "config", "user.email", "evidence@example.test")
	runTestCommand(t, root, "git", "config", "user.name", "Evidence Test")
	if err := os.WriteFile(filepath.Join(root, "greeting.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, root, "git", "add", "greeting.txt")
	runTestCommand(t, root, "git", "commit", "-qm", "fixture")
	return root
}

func runTestCommand(t *testing.T, directory, name string, args ...string) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = directory
	if name == "git" {
		command.Env = isolatedGitEnvironment()
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", name, err, output)
	}
}

func readJSONFile(t *testing.T, path string, destination any) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, destination); err != nil {
		t.Fatal(err)
	}
}
