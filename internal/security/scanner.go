package security

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type Vulnerability struct {
	ID          string
	Severity    string
	Package     string
	Version     string
	File        string
	Line        int
	Description string
	Fix         string
	CVE         string
}

type SecurityReport struct {
	Language        string
	Vulnerabilities []Vulnerability
	FixedCount      int
	UpdatedFiles    []string
	Errors          []error
}

type Scanner struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewScanner(provider llm.Provider, model, workDir string) *Scanner {
	return &Scanner{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (s *Scanner) ScanAndFix(ctx context.Context, autofix bool) (*SecurityReport, error) {
	lang := langdetect.DetectLanguage(s.workDir)

	vulns, err := s.scanVulnerabilities(lang)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	report := &SecurityReport{
		Language:        string(lang),
		Vulnerabilities: vulns,
	}

	if !autofix || len(vulns) == 0 {
		return report, nil
	}

	for _, vuln := range vulns {
		if err := s.fixVulnerability(ctx, vuln, lang); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("failed to fix %s: %w", vuln.ID, err))
		} else {
			report.FixedCount++
			if vuln.File != "" && !contains(report.UpdatedFiles, vuln.File) {
				report.UpdatedFiles = append(report.UpdatedFiles, vuln.File)
			}
		}
	}

	return report, nil
}

func (s *Scanner) scanVulnerabilities(lang langdetect.Language) ([]Vulnerability, error) {
	switch lang {
	case langdetect.Go:
		return s.scanGo()
	case langdetect.TypeScript:
		return s.scanNode()
	case langdetect.Python:
		return s.scanPython()
	case langdetect.Ruby:
		return s.scanRuby()
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func (s *Scanner) scanGo() ([]Vulnerability, error) {
	cmd := exec.Command("govulncheck", "-json", "./...")
	cmd.Dir = s.workDir
	output, err := cmd.Output()

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && len(exitErr.Stderr) > 0 {
			output = exitErr.Stderr
		}
	}

	return s.parseGovulncheck(string(output))
}

func (s *Scanner) parseGovulncheck(output string) ([]Vulnerability, error) {
	var vulns []Vulnerability

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" || !strings.Contains(line, "vulnerability") {
			continue
		}

		if strings.Contains(line, "GO-") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				vuln := Vulnerability{
					ID:          extractID(line, "GO-"),
					Severity:    extractSeverity(line),
					Package:     extractPackage(line),
					Description: line,
				}
				vulns = append(vulns, vuln)
			}
		}
	}

	return vulns, nil
}

func (s *Scanner) scanNode() ([]Vulnerability, error) {
	cmd := exec.Command("npm", "audit", "--json")
	cmd.Dir = s.workDir
	output, err := cmd.Output()

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			output = exitErr.Stderr
		}
	}

	return s.parseNpmAudit(string(output))
}

func (s *Scanner) parseNpmAudit(output string) ([]Vulnerability, error) {
	var vulns []Vulnerability

	if !strings.Contains(output, "vulnerabilities") {
		return vulns, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "severity") {
			vuln := Vulnerability{
				Severity:    extractSeverity(line),
				Description: line,
			}
			vulns = append(vulns, vuln)
		}
	}

	return vulns, nil
}

func (s *Scanner) scanPython() ([]Vulnerability, error) {
	cmd := exec.Command("safety", "check", "--json")
	cmd.Dir = s.workDir
	output, err := cmd.Output()

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			output = exitErr.Stderr
		}
	}

	return s.parseSafety(string(output))
}

func (s *Scanner) parseSafety(output string) ([]Vulnerability, error) {
	var vulns []Vulnerability

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "vulnerability") || strings.Contains(line, "CVE") {
			vuln := Vulnerability{
				CVE:         extractCVE(line),
				Description: line,
			}
			vulns = append(vulns, vuln)
		}
	}

	return vulns, nil
}

func (s *Scanner) scanRuby() ([]Vulnerability, error) {
	cmd := exec.Command("bundle", "audit", "check")
	cmd.Dir = s.workDir
	output, err := cmd.Output()

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			output = exitErr.Stderr
		}
	}

	return s.parseBundleAudit(string(output))
}

func (s *Scanner) parseBundleAudit(output string) ([]Vulnerability, error) {
	var vulns []Vulnerability

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "CVE") || strings.Contains(line, "vulnerability") {
			vuln := Vulnerability{
				CVE:         extractCVE(line),
				Description: line,
			}
			vulns = append(vulns, vuln)
		}
	}

	return vulns, nil
}

func (s *Scanner) fixVulnerability(ctx context.Context, vuln Vulnerability, lang langdetect.Language) error {
	switch lang {
	case langdetect.Go:
		return s.fixGoVulnerability(ctx, vuln)
	case langdetect.TypeScript:
		return s.fixNodeVulnerability(vuln)
	case langdetect.Python:
		return s.fixPythonVulnerability(vuln)
	case langdetect.Ruby:
		return s.fixRubyVulnerability(vuln)
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}
}

func (s *Scanner) fixGoVulnerability(ctx context.Context, vuln Vulnerability) error {
	if vuln.Package == "" {
		return fmt.Errorf("no package specified")
	}

	cmd := exec.Command("go", "get", "-u", vuln.Package)
	cmd.Dir = s.workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		return s.fixWithLLM(ctx, vuln, string(output))
	}

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = s.workDir
	return cmd.Run()
}

func (s *Scanner) fixNodeVulnerability(vuln Vulnerability) error {
	cmd := exec.Command("npm", "audit", "fix")
	cmd.Dir = s.workDir
	return cmd.Run()
}

func (s *Scanner) fixPythonVulnerability(vuln Vulnerability) error {
	if vuln.Package == "" {
		return fmt.Errorf("no package specified")
	}

	cmd := exec.Command("pip", "install", "--upgrade", vuln.Package)
	cmd.Dir = s.workDir
	return cmd.Run()
}

func (s *Scanner) fixRubyVulnerability(vuln Vulnerability) error {
	cmd := exec.Command("bundle", "update")
	cmd.Dir = s.workDir
	return cmd.Run()
}

func (s *Scanner) fixWithLLM(ctx context.Context, vuln Vulnerability, errorOutput string) error {
	files, err := s.findVulnerableFiles(vuln)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no vulnerable files found")
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		prompt := fmt.Sprintf(`Fix security vulnerability in code:

Vulnerability: %s
ID: %s
Severity: %s
Package: %s
Description: %s

Error when trying to update: %s

File: %s
Content:
%s

Provide a secure fix. Return ONLY the complete updated file content.`,
			vuln.ID, vuln.ID, vuln.Severity, vuln.Package, vuln.Description,
			errorOutput, file, string(content))

		resp, err := s.provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: "You are a security expert that fixes vulnerabilities safely and correctly.",
			UserPrompt:   prompt,
			Model:        s.model,
		})

		if err != nil {
			return err
		}

		updated := s.extractCode(resp.Text)
		if err := os.WriteFile(file, []byte(updated), 0644); err != nil {
			return err
		}

		vuln.File = file
	}

	return nil
}

func (s *Scanner) findVulnerableFiles(vuln Vulnerability) ([]string, error) {
	if vuln.Package == "" {
		return nil, nil
	}

	pkgName := filepath.Base(vuln.Package)

	cmd := exec.Command("grep", "-rl", "--include=*.go", pkgName, s.workDir)
	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, f := range files {
		if f != "" && !strings.Contains(f, "vendor/") {
			result = append(result, f)
		}
	}

	return result, nil
}

func (s *Scanner) extractCode(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "```go") {
		text = strings.TrimPrefix(text, "```go\n")
		text = strings.TrimSuffix(text, "```")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```\n")
		text = strings.TrimSuffix(text, "```")
	}

	return strings.TrimSpace(text)
}

func extractID(line, prefix string) string {
	if idx := strings.Index(line, prefix); idx != -1 {
		end := idx + len(prefix)
		for end < len(line) && (line[end] == '-' || (line[end] >= '0' && line[end] <= '9')) {
			end++
		}
		return line[idx:end]
	}
	return ""
}

func extractSeverity(line string) string {
	lower := strings.ToLower(line)
	severities := []string{"critical", "high", "medium", "moderate", "low"}
	for _, sev := range severities {
		if strings.Contains(lower, sev) {
			return strings.ToUpper(sev[:1]) + sev[1:]
		}
	}
	return "Unknown"
}

func extractPackage(line string) string {
	parts := strings.Fields(line)
	for i, part := range parts {
		if strings.Contains(part, "/") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func extractCVE(line string) string {
	if idx := strings.Index(line, "CVE-"); idx != -1 {
		end := idx + 4
		for end < len(line) && (line[end] == '-' || (line[end] >= '0' && line[end] <= '9')) {
			end++
		}
		return line[idx:end]
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
