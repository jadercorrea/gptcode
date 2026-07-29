package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSuiteExecutesEveryRepetitionAndAggregatesResults(t *testing.T) {
	t.Parallel()

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "value.txt"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "suite")

	summary, err := RunSuite(context.Background(), Suite{
		ID:                "repeatability",
		Output:            output,
		Repetitions:       2,
		RunTimeoutSeconds: 5,
		Agent: Command{
			Name: "fixture-agent",
			Args: []string{"/bin/sh", "-c", "printf 'after\\n' > value.txt"},
		},
		Fixtures: []SuiteFixture{
			{
				ID:                     "text-change",
				Source:                 fixture,
				RequireFailingBaseline: true,
				Verifications: []Command{
					{
						Name: "content",
						Args: []string{"/bin/sh", "-c", "test \"$(cat value.txt)\" = after"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if summary.TotalRuns != 2 || summary.PassedRuns != 2 || summary.PassRate != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Runs) != 2 || summary.Runs[0].Repetition != 1 || summary.Runs[1].Repetition != 2 {
		t.Fatalf("unexpected run records: %+v", summary.Runs)
	}
	for _, run := range summary.Runs {
		if _, err := os.Stat(filepath.Join(output, run.EvidencePath, "manifest.json")); err != nil {
			t.Errorf("run evidence missing: %v", err)
		}
	}

	var persisted SuiteSummary
	content, err := os.ReadFile(filepath.Join(output, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.TotalRuns != summary.TotalRuns || persisted.PassRate != summary.PassRate {
		t.Fatalf("persisted summary = %+v, want %+v", persisted, summary)
	}
}

func TestRunSuiteExecutesAndRecordsSetupCommands(t *testing.T) {
	t.Parallel()

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "value.txt"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "configured")
	output := filepath.Join(t.TempDir(), "suite")

	_, err := RunSuite(context.Background(), Suite{
		ID:                "explicit-setup",
		Output:            output,
		Repetitions:       1,
		RunTimeoutSeconds: 5,
		Setup: []Command{
			{Name: "select-profile", Args: []string{"/usr/bin/touch", marker}},
		},
		Agent: Command{
			Name: "fixture-agent",
			Args: []string{"/bin/sh", "-c", "printf 'after\\n' > value.txt"},
		},
		Fixtures: []SuiteFixture{
			{
				ID:                     "text-change",
				Source:                 fixture,
				RequireFailingBaseline: true,
				Verifications: []Command{
					{Name: "content", Args: []string{"/bin/sh", "-c", "test \"$(cat value.txt)\" = after"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("setup command did not execute: %v", err)
	}
	var setup VerificationReport
	readJSONFile(t, filepath.Join(output, "setup.json"), &setup)
	if !setup.Passed || len(setup.Commands) != 1 || setup.Commands[0].Name != "select-profile" {
		t.Fatalf("setup evidence = %+v", setup)
	}
}

func TestRunSuiteRetainsFailedRunsInAggregate(t *testing.T) {
	t.Parallel()

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "value.txt"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	summary, err := RunSuite(context.Background(), Suite{
		ID:                "failure-is-evidence",
		Output:            filepath.Join(t.TempDir(), "suite"),
		Repetitions:       1,
		RunTimeoutSeconds: 5,
		Agent:             Command{Name: "noop", Args: []string{"/usr/bin/true"}},
		Fixtures: []SuiteFixture{
			{
				ID:                     "failing-check",
				Source:                 fixture,
				RequireFailingBaseline: true,
				Verifications: []Command{
					{Name: "failure", Args: []string{"/usr/bin/false"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if summary.TotalRuns != 1 || summary.PassedRuns != 0 || summary.PassRate != 0 {
		t.Fatalf("failed run was not retained: %+v", summary)
	}
	if len(summary.Runs) != 1 || summary.Runs[0].Passed {
		t.Fatalf("failed run record missing: %+v", summary.Runs)
	}
}

func TestRunSuiteRejectsFixtureThatAlreadyPassesRequiredBaseline(t *testing.T) {
	t.Parallel()

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "value.txt"), []byte("already valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := RunSuite(context.Background(), Suite{
		ID:                "green-baseline",
		Output:            filepath.Join(t.TempDir(), "suite"),
		Repetitions:       1,
		RunTimeoutSeconds: 5,
		Agent:             Command{Name: "noop", Args: []string{"/usr/bin/true"}},
		Fixtures: []SuiteFixture{
			{
				ID:                     "already-passing",
				Source:                 fixture,
				RequireFailingBaseline: true,
				Verifications: []Command{
					{Name: "passes", Args: []string{"/usr/bin/true"}},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("RunSuite() error = nil, want green baseline rejection")
	}
}

func TestRunSuiteRecordsAgentTimeoutAndContinuesVerification(t *testing.T) {
	t.Parallel()

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "value.txt"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "suite")

	summary, err := RunSuite(context.Background(), Suite{
		ID:                "bounded-run",
		Output:            output,
		Repetitions:       1,
		RunTimeoutSeconds: 1,
		Agent:             Command{Name: "slow", Args: []string{"/bin/sleep", "5"}},
		Fixtures: []SuiteFixture{
			{
				ID:     "timeout",
				Source: fixture,
				Verifications: []Command{
					{Name: "repository-still-valid", Args: []string{"/usr/bin/true"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if len(summary.Runs) != 1 || !summary.Runs[0].TimedOut || summary.Runs[0].Passed {
		t.Fatalf("timeout not represented as failed evidence: %+v", summary)
	}

	var verification VerificationReport
	readJSONFile(
		t,
		filepath.Join(output, summary.Runs[0].EvidencePath, "verification.json"),
		&verification,
	)
	if len(verification.Commands) != 1 || verification.Commands[0].ExitCode != 0 {
		t.Fatalf("verification did not run after timeout: %+v", verification)
	}
}
