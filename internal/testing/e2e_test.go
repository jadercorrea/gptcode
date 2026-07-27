package testing

import (
	"os"
	"testing"
)

func TestGTVersion(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("version")

	AssertSuccess(t, result)
	AssertContains(t, result.Output(), "GPTCode")
}

// TestGTStatus removed as status command no longer exists

func TestGTHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("--help")

	AssertSuccess(t, result)
	AssertContains(t, result.Output(), "GPTCode")
}

func TestGTRunHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("run", "--help")

	AssertSuccess(t, result)
	AssertContains(t, result.Output(), "Execute")
}

func TestGTDoHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("do", "--help")

	AssertSuccess(t, result)
}

func TestGTModelList(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Requires setup.yaml")
	}
	cli := NewCLI("gptcode")
	result := cli.Run("model", "list")

	AssertSuccess(t, result)
}

func TestGTBackendList(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Requires setup.yaml")
	}
	cli := NewCLI("gptcode")
	result := cli.Run("backend", "list")

	AssertSuccess(t, result)
}

func getTestWorkDir(t *testing.T) string {
	dir := t.TempDir()
	return dir
}

func TestGTRunSimple(t *testing.T) {
	cli := NewCLI("gptcode")
	cli.workDir = getTestWorkDir(t)

	result := cli.Run("run", "--once", "echo hello")

	// Should succeed or fail gracefully
	if result.Failed() {
		t.Logf("Command failed with: %s", result.Stderr)
	}
}

// TestGTPRHelp and TestGTIssueHelp removed as pr/issue commands no longer exist on root

// TestGTInit removed as the init command no longer exists

func TestGTSetupHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("setup", "--help")

	AssertSuccess(t, result)
}

func TestGTKeyHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("key", "--help")

	AssertSuccess(t, result)
}

func TestGTChatHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("chat", "--help")

	AssertSuccess(t, result)
}

func TestGTResearchHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("research", "--help")

	AssertSuccess(t, result)
}

func TestGTPlanHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("plan", "--help")

	AssertSuccess(t, result)
}

func TestGTImplementHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("implement", "--help")

	AssertSuccess(t, result)
}

func TestGTFeatureHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("feature", "--help")

	AssertSuccess(t, result)
}

func TestGTReviewHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("review", "--help")

	AssertSuccess(t, result)
}

func TestGTGitHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("git", "--help")

	AssertSuccess(t, result)
}

func TestGTContextHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("context", "--help")

	AssertSuccess(t, result)
}

func TestGTPerfHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("perf", "--help")

	AssertSuccess(t, result)
}

func TestGTCoverageHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("coverage", "--help")

	AssertSuccess(t, result)
}

func TestGTTestHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("test", "--help")

	AssertSuccess(t, result)
}

func TestGTGraphHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("graph", "--help")

	AssertSuccess(t, result)
}

func TestGTRefactorHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("refactor", "--help")

	AssertSuccess(t, result)
}

func TestGTMergeHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("merge", "--help")

	AssertSuccess(t, result)
}

func TestGTDocHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("docs", "--help")

	AssertSuccess(t, result)
}

func TestGTGenHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("gen", "--help")

	AssertSuccess(t, result)
}

func TestGTEvolveHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("evolve", "--help")

	AssertSuccess(t, result)
}

func TestGTProfileHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("profile", "--help")

	AssertSuccess(t, result)
}

func TestGTProfilesHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("profiles", "--help")

	AssertSuccess(t, result)
}

func TestGTFeedbackHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("feedback", "--help")

	AssertSuccess(t, result)
}

func TestGTDetectHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("detect", "--help")

	AssertSuccess(t, result)
}

func TestGTBackendHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("backend", "--help")

	AssertSuccess(t, result)
}

func TestGTConfigHelp(t *testing.T) {
	cli := NewCLI("gptcode")
	result := cli.Run("config", "--help")

	AssertSuccess(t, result)
}

// TestGTDoctorHelp removed as status command no longer exists
