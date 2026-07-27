package testgen

import (
	"os/exec"
	"path/filepath"

	"github.com/jadercorrea/gptcode/internal/langdetect"
)

type Validator struct {
	workDir  string
	language langdetect.Language
}

func NewValidator(workDir string, language langdetect.Language) *Validator {
	return &Validator{
		workDir:  workDir,
		language: language,
	}
}

func (v *Validator) Validate(testFile string) bool {
	switch v.language {
	case langdetect.Go:
		return v.validateGo(testFile)
	case langdetect.TypeScript:
		return v.validateTypeScript(testFile)
	case langdetect.Python:
		return v.validatePython(testFile)
	default:
		return true
	}
}

func (v *Validator) validateGo(testFile string) bool {
	cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", testFile)
	cmd.Dir = v.workDir
	return cmd.Run() == nil
}

func (v *Validator) validateTypeScript(testFile string) bool {
	absPath := filepath.Join(v.workDir, testFile)
	cmd := exec.Command("tsc", "--noEmit", absPath)
	cmd.Dir = v.workDir
	return cmd.Run() == nil
}

func (v *Validator) validatePython(testFile string) bool {
	absPath := filepath.Join(v.workDir, testFile)
	cmd := exec.Command("python", "-m", "py_compile", absPath)
	cmd.Dir = v.workDir
	return cmd.Run() == nil
}
