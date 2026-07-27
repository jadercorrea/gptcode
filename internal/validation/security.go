package validation

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
)

type SecurityResult struct {
	Success         bool
	Vulnerabilities int
	Output          string
	ErrorMessage    string
}

type SecurityScanner struct {
	workDir string
}

func NewSecurityScanner(workDir string) *SecurityScanner {
	return &SecurityScanner{workDir: workDir}
}

func (ss *SecurityScanner) RunScan() (*SecurityResult, error) {
	lang := langdetect.DetectLanguage(ss.workDir)
	switch lang {
	case langdetect.Go:
		return ss.scanGo()
	case langdetect.TypeScript:
		return ss.scanNode()
	case langdetect.Python:
		return ss.scanPython()
	default:
		return &SecurityResult{Success: true}, nil
	}
}

func (ss *SecurityScanner) scanGo() (*SecurityResult, error) {
	if !commandExists("govulncheck") {
		return &SecurityResult{Success: true, Output: "govulncheck not installed (skipping)"}, nil
	}
	cmd := exec.Command("govulncheck", "./...")
	cmd.Dir = ss.workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	res := &SecurityResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
		res.Vulnerabilities = strings.Count(out, "Vulnerability")
	}
	return res, nil
}

func (ss *SecurityScanner) scanNode() (*SecurityResult, error) {
	pkg := filepath.Join(ss.workDir, "package.json")
	if !fileExists(pkg) {
		return &SecurityResult{Success: true}, nil
	}
	cmd := exec.Command("npm", "audit", "--json")
	cmd.Dir = ss.workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	res := &SecurityResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
		res.Vulnerabilities = strings.Count(out, "\"severity\"")
	}
	return res, nil
}

func (ss *SecurityScanner) scanPython() (*SecurityResult, error) {
	if !commandExists("safety") {
		return &SecurityResult{Success: true, Output: "safety not installed (skipping)"}, nil
	}
	cmd := exec.Command("safety", "check", "--json")
	cmd.Dir = ss.workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	res := &SecurityResult{Success: err == nil, Output: out}
	if err != nil {
		res.ErrorMessage = err.Error()
		res.Vulnerabilities = strings.Count(out, "vulnerability")
	}
	return res, nil
}
