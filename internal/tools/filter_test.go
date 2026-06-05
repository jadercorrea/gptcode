package tools

import (
	"os"
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	input := "\x1b[31mError:\x1b[0m failed to connect"
	expected := "Error: failed to connect"
	actual := StripANSI(input)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func TestFilterEngine_GoTest(t *testing.T) {
	fe := NewFilterEngine()

	// 1. All tests passing
	passingOutput := "=== RUN   TestAwesome\n--- PASS: TestAwesome (0.00s)\nPASS\nok  \tgptcode/internal/awesome\t0.005s\n"
	res := fe.Filter("go test ./...", passingOutput, 0)
	if !strings.Contains(res.FilteredOutput, "ok go test: all tests passed") {
		t.Errorf("expected passing summary, got: %q", res.FilteredOutput)
	}
	if strings.Contains(res.FilteredOutput, "=== RUN") {
		t.Error("expected running detail to be filtered out for passing tests")
	}

	// 2. Failing tests
	failingOutput := "=== RUN   TestFail\n    some_file_test.go:42: Error expectation failed\n--- FAIL: TestFail (0.00s)\nFAIL\nexit status 1\nFAIL\tgptcode/internal/fail\t0.010s\n"
	res = fe.Filter("go test ./...", failingOutput, 1)
	if !strings.Contains(res.FilteredOutput, "--- FAIL:") {
		t.Error("expected failure details to be retained")
	}
	if !strings.Contains(res.FilteredOutput, "Error expectation failed") {
		t.Error("expected trace detail to be retained")
	}
	if strings.Contains(res.FilteredOutput, "=== RUN") {
		t.Error("expected running detail to be filtered out")
	}
}

func TestFilterEngine_Pytest(t *testing.T) {
	fe := NewFilterEngine()

	// Passing Pytest
	passingOutput := "============================= test session starts ==============================\nplatform linux -- Python 3.10.12, pytest-7.4.0, pluggy-1.2.0\nrootdir: /app\ncollected 5 items\n\ntests/test_app.py .....                                                   [100%]\n\n============================== 5 passed in 0.12s ==============================="
	res := fe.Filter("pytest", passingOutput, 0)
	if !strings.Contains(res.FilteredOutput, "ok pytest: all tests passed") {
		t.Errorf("expected passing summary, got: %q", res.FilteredOutput)
	}

	// Failing Pytest
	failingOutput := "============================= test session starts ==============================\ncollected 2 items\n\ntests/test_app.py .F                                                     [100%]\n\n=================================== FAILURES ===================================\n__________________________________ test_fail ___________________________________\n\n    def test_fail():\n>       assert False\nE       assert False\n\ntests/test_app.py:12: AssertionError\n=========================== short test summary info ============================\nFAILED tests/test_app.py::test_fail - assert False\n============================== 1 failed, 1 passed in 0.15s =============================="
	res = fe.Filter("pytest", failingOutput, 1)
	if !strings.Contains(res.FilteredOutput, "==== FAILURES ====") {
		t.Error("expected failures section to be retained")
	}
	if !strings.Contains(res.FilteredOutput, "AssertionError") {
		t.Error("expected assertion details to be retained")
	}
}

func TestFilterEngine_NpmInstall(t *testing.T) {
	fe := NewFilterEngine()

	output := "npm info it worked if it ends with ok\nnpm http fetch GET 200 https://registry.npmjs.org/lodash\nadded 1 package, and audited 2 packages in 1s\nfound 0 vulnerabilities\n"
	res := fe.Filter("npm install lodash", output, 0)

	if strings.Contains(res.FilteredOutput, "npm info") || strings.Contains(res.FilteredOutput, "npm http fetch") {
		t.Error("expected verbose installation logs to be filtered out")
	}
	if !strings.Contains(res.FilteredOutput, "added 1 package") {
		t.Error("expected packages summary to be retained")
	}
}

func TestFilterEngine_GitStatus(t *testing.T) {
	fe := NewFilterEngine()

	output := "On branch main\nYour branch is up to date with 'origin/main'.\n\nChanges not staged for commit:\n  (use \"git add <file>...\" to update what will be committed)\n  (use \"git restore <file>...\" to discard changes in working directory)\n\tmodified:   main.go\n\nUntracked files:\n  (use \"git add <file>...\" to include in what will be committed)\n\tnew_file.go\n"
	res := fe.Filter("git status", output, 0)

	if strings.Contains(res.FilteredOutput, "use \"git add") {
		t.Error("expected git help guides to be removed")
	}
	if !strings.Contains(res.FilteredOutput, "modified:   main.go") {
		t.Error("expected modified files to be retained")
	}
	if !strings.Contains(res.FilteredOutput, "new_file.go") {
		t.Error("expected untracked files list to be retained")
	}
}

func TestFilterEngine_Tee(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gptcode_tee_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fe := &FilterEngine{teeDir: tmpDir}

	rawOutput := "Compilation failed:\nmain.go:5:10: syntax error: unexpected semicolon"
	res := fe.Filter("go build", rawOutput, 1)

	if !strings.Contains(res.FilteredOutput, "[GPTCode Tee] Command failed. Full unfiltered output saved to local log:") {
		t.Errorf("expected footnote about Tee logs, got: %q", res.FilteredOutput)
	}

	if res.TeePath == "" {
		t.Fatal("expected TeePath to be set in result")
	}

	// Verify file was written
	if _, err := os.Stat(res.TeePath); os.IsNotExist(err) {
		t.Errorf("expected tee file at %s to exist", res.TeePath)
	}

	content, err := os.ReadFile(res.TeePath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "Command: go build") {
		t.Errorf("expected command metadata in Tee log, got: %s", string(content))
	}
	if !strings.Contains(string(content), "syntax error:") {
		t.Errorf("expected raw output content in Tee log, got: %s", string(content))
	}
}
