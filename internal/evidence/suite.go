package evidence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const SuiteSchemaVersion = "gptcode.evidence-suite/v1"

var suiteIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Suite struct {
	ID                string         `json:"id"`
	Output            string         `json:"output"`
	Repetitions       int            `json:"repetitions"`
	RunTimeoutSeconds int            `json:"run_timeout_seconds"`
	Setup             []Command      `json:"setup,omitempty"`
	Agent             Command        `json:"agent"`
	Fixtures          []SuiteFixture `json:"fixtures"`
	Progress          io.Writer      `json:"-"`
}

type SuiteFixture struct {
	ID                     string    `json:"id"`
	Source                 string    `json:"source"`
	RequireFailingBaseline bool      `json:"require_failing_baseline"`
	Verifications          []Command `json:"verifications"`
}

type SuiteRun struct {
	FixtureID    string        `json:"fixture_id"`
	Repetition   int           `json:"repetition"`
	Passed       bool          `json:"passed"`
	TimedOut     bool          `json:"timed_out"`
	Duration     time.Duration `json:"duration"`
	EvidencePath string        `json:"evidence_path"`
}

type SuiteSummary struct {
	SchemaVersion  string        `json:"schema_version"`
	SuiteID        string        `json:"suite_id"`
	TotalRuns      int           `json:"total_runs"`
	PassedRuns     int           `json:"passed_runs"`
	PassRate       float64       `json:"pass_rate"`
	MedianDuration time.Duration `json:"median_duration"`
	Runs           []SuiteRun    `json:"runs"`
}

// RunSuite executes every fixture and repetition sequentially. Sequential
// execution keeps local-model resource contention from biasing comparisons.
func RunSuite(ctx context.Context, suite Suite) (SuiteSummary, error) {
	if err := validateSuite(suite); err != nil {
		return SuiteSummary{}, err
	}
	if _, err := os.Stat(suite.Output); err == nil {
		return SuiteSummary{}, errors.New("suite output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return SuiteSummary{}, fmt.Errorf("checking suite output: %w", err)
	}
	if err := os.Mkdir(suite.Output, 0o700); err != nil {
		return SuiteSummary{}, fmt.Errorf("creating suite output: %w", err)
	}
	if len(suite.Setup) > 0 {
		setup := runVerification(ctx, ".", suite.Setup)
		if err := writeJSON(filepath.Join(suite.Output, "setup.json"), setup); err != nil {
			return SuiteSummary{}, fmt.Errorf("writing suite setup evidence: %w", err)
		}
		if !setup.Passed {
			return SuiteSummary{}, errors.New("suite setup command failed")
		}
	}

	summary := SuiteSummary{
		SchemaVersion: SuiteSchemaVersion,
		SuiteID:       suite.ID,
	}
	var durations []time.Duration
	for _, fixture := range suite.Fixtures {
		for repetition := 1; repetition <= suite.Repetitions; repetition++ {
			if err := ctx.Err(); err != nil {
				return summary, err
			}
			runDirectory := filepath.Join(
				suite.Output,
				"runs",
				fixture.ID,
				fmt.Sprintf("%03d", repetition),
			)
			workspace := filepath.Join(runDirectory, "workspace")
			evidencePath := filepath.Join(runDirectory, "evidence")
			if err := os.MkdirAll(runDirectory, 0o700); err != nil {
				return summary, fmt.Errorf("creating run directory: %w", err)
			}
			if err := copyFixture(fixture.Source, workspace); err != nil {
				return summary, fmt.Errorf("copying fixture %q: %w", fixture.ID, err)
			}
			if err := initializeFixtureRepository(ctx, workspace); err != nil {
				return summary, fmt.Errorf("initializing fixture %q: %w", fixture.ID, err)
			}
			baseline := runVerification(ctx, workspace, fixture.Verifications)
			if err := writeJSON(filepath.Join(runDirectory, "baseline.json"), baseline); err != nil {
				return summary, fmt.Errorf("writing fixture baseline: %w", err)
			}
			if fixture.RequireFailingBaseline && baseline.Passed {
				return summary, fmt.Errorf(
					"fixture %q repetition %d already passes its baseline",
					fixture.ID,
					repetition,
				)
			}

			started := time.Now()
			result, err := RunExperiment(ctx, Experiment{
				ID: fmt.Sprintf(
					"%s-%s-%03d",
					suite.ID,
					fixture.ID,
					repetition,
				),
				Repository:    workspace,
				Output:        evidencePath,
				Agent:         suite.Agent,
				Verifications: fixture.Verifications,
				AgentTimeout:  time.Duration(suite.RunTimeoutSeconds) * time.Second,
				Progress:      suite.Progress,
			})
			duration := time.Since(started)
			if err != nil {
				return summary, fmt.Errorf(
					"running fixture %q repetition %d: %w",
					fixture.ID,
					repetition,
					err,
				)
			}

			relativeEvidence, err := filepath.Rel(suite.Output, evidencePath)
			if err != nil {
				return summary, fmt.Errorf("relativizing evidence path: %w", err)
			}
			run := SuiteRun{
				FixtureID:    fixture.ID,
				Repetition:   repetition,
				Passed:       result.Passed,
				TimedOut:     result.TimedOut,
				Duration:     duration,
				EvidencePath: filepath.ToSlash(relativeEvidence),
			}
			summary.Runs = append(summary.Runs, run)
			summary.TotalRuns++
			if result.Passed {
				summary.PassedRuns++
			}
			durations = append(durations, duration)
		}
	}
	if summary.TotalRuns > 0 {
		summary.PassRate = float64(summary.PassedRuns) / float64(summary.TotalRuns)
		summary.MedianDuration = medianDuration(durations)
	}
	if err := writeJSON(filepath.Join(suite.Output, "summary.json"), summary); err != nil {
		return summary, fmt.Errorf("writing suite summary: %w", err)
	}
	return summary, nil
}

func runVerification(ctx context.Context, root string, commands []Command) VerificationReport {
	report := VerificationReport{Passed: true}
	for _, command := range commands {
		result := runCommand(ctx, root, command)
		report.Commands = append(report.Commands, result)
		if result.ExitCode != 0 {
			report.Passed = false
		}
	}
	return report
}

func validateSuite(suite Suite) error {
	if !suiteIDPattern.MatchString(suite.ID) {
		return errors.New("suite ID must contain lowercase letters, numbers, or hyphens")
	}
	if suite.Output == "" {
		return errors.New("suite output is required")
	}
	if suite.Repetitions < 1 {
		return errors.New("suite repetitions must be positive")
	}
	if suite.RunTimeoutSeconds < 1 {
		return errors.New("suite run timeout must be positive")
	}
	if suite.Agent.Name == "" || len(suite.Agent.Args) == 0 {
		return errors.New("suite agent name and executable are required")
	}
	if len(suite.Fixtures) == 0 {
		return errors.New("suite must contain at least one fixture")
	}
	seen := make(map[string]struct{}, len(suite.Fixtures))
	for _, fixture := range suite.Fixtures {
		if !suiteIDPattern.MatchString(fixture.ID) {
			return fmt.Errorf("invalid fixture ID %q", fixture.ID)
		}
		if _, found := seen[fixture.ID]; found {
			return fmt.Errorf("duplicate fixture ID %q", fixture.ID)
		}
		seen[fixture.ID] = struct{}{}
		if fixture.Source == "" {
			return fmt.Errorf("fixture %q source is required", fixture.ID)
		}
		if len(fixture.Verifications) == 0 {
			return fmt.Errorf("fixture %q requires deterministic verification", fixture.ID)
		}
	}
	return nil
}

func copyFixture(source, destination string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return errors.New("fixture source must be a directory")
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.Mkdir(destination, 0o700)
		}
		if relative == ".git" {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.Mkdir(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported fixture file type at %q", relative)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return outputCloseErr
	})
}

func initializeFixtureRepository(ctx context.Context, root string) error {
	commands := [][]string{
		{"init", "--quiet"},
		{"add", "."},
		{
			"-c", "user.name=GPTCode Evidence",
			"-c", "user.email=evidence@gptcode.dev",
			"-c", "core.hooksPath=/dev/null",
			"commit", "--quiet", "-m", "fixture: initial state",
		},
	}
	for _, args := range commands {
		if _, err := gitOutput(ctx, root, args...); err != nil {
			return err
		}
	}
	return nil
}

func medianDuration(durations []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), durations...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left] < ordered[right]
	})
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}
