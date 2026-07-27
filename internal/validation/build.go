package validation

import (
	"bytes"
	"os/exec"
	"path/filepath"

	"github.com/jadercorrea/gptcode/internal/langdetect"
)

type BuildResult struct {
	Success      bool
	Output       string
	ErrorMessage string
}

type BuildExecutor struct {
	workDir string
}

func NewBuildExecutor(workDir string) *BuildExecutor {
	return &BuildExecutor{workDir: workDir}
}

func (be *BuildExecutor) RunBuild() (*BuildResult, error) {
	lang := langdetect.DetectLanguage(be.workDir)
	switch lang {
	case langdetect.Go:
		return be.runGoBuild()
	case langdetect.TypeScript:
		return be.runNodeBuild()
	case langdetect.Elixir:
		return be.runElixirBuild()
	default:
		return &BuildResult{Success: true}, nil
	}
}

func (be *BuildExecutor) runGoBuild() (*BuildResult, error) {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = be.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	res := &BuildResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
	}
	return res, nil
}

func (be *BuildExecutor) runNodeBuild() (*BuildResult, error) {
	pkg := filepath.Join(be.workDir, "package.json")
	if !fileExists(pkg) {
		return &BuildResult{Success: true}, nil
	}
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = be.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	res := &BuildResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
	}
	return res, nil
}

func (be *BuildExecutor) runElixirBuild() (*BuildResult, error) {
	cmd := exec.Command("mix", "compile")
	cmd.Dir = be.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	res := &BuildResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
	}
	return res, nil
}
