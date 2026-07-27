package coverage

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type CoverageAnalyzer struct {
	provider llm.Provider
	model    string
	workDir  string
}

type Gap struct {
	File       string
	Function   string
	Line       int
	Coverage   float64
	Suggestion string
}

type AnalysisResult struct {
	TotalCoverage float64
	Gaps          []Gap
	Report        string
}

func NewCoverageAnalyzer(provider llm.Provider, model, workDir string) *CoverageAnalyzer {
	return &CoverageAnalyzer{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (a *CoverageAnalyzer) Analyze(ctx context.Context, packagePath string) (*AnalysisResult, error) {
	absPath := packagePath
	if !filepath.IsAbs(packagePath) {
		absPath = filepath.Join(a.workDir, packagePath)
	}

	coverageData, err := a.runCoverage(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to run coverage: %w", err)
	}

	gaps, err := a.identifyGaps(coverageData)
	if err != nil {
		return nil, fmt.Errorf("failed to identify gaps: %w", err)
	}

	report, err := a.generateReport(ctx, gaps, coverageData)
	if err != nil {
		report = a.generateBasicReport(gaps, coverageData)
	}

	result := &AnalysisResult{
		TotalCoverage: coverageData.Total,
		Gaps:          gaps,
		Report:        report,
	}

	return result, nil
}

type coverageData struct {
	Total     float64
	ByFile    map[string]float64
	Functions map[string]functionCoverage
	RawOutput string
}

type functionCoverage struct {
	File     string
	Name     string
	Line     int
	Coverage float64
	Covered  int
	Total    int
}

func (a *CoverageAnalyzer) runCoverage(pkgPath string) (*coverageData, error) {
	coverFile := filepath.Join(a.workDir, "coverage.out")
	defer os.Remove(coverFile)

	cmd := exec.Command("go", "test", "-coverprofile="+coverFile, pkgPath)
	cmd.Dir = a.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go test failed: %w\nOutput: %s", err, string(output))
	}

	cmd = exec.Command("go", "tool", "cover", "-func="+coverFile)
	cmd.Dir = a.workDir
	funcOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go tool cover failed: %w", err)
	}

	return a.parseCoverageOutput(string(funcOutput))
}

var funcCoveragePattern = regexp.MustCompile(`^(.+):(\d+):\s+(\S+)\s+([\d.]+)%`)

func (a *CoverageAnalyzer) parseCoverageOutput(output string) (*coverageData, error) {
	data := &coverageData{
		ByFile:    make(map[string]float64),
		Functions: make(map[string]functionCoverage),
		RawOutput: output,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "total:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				totalStr := strings.TrimSuffix(parts[2], "%")
				if total, err := strconv.ParseFloat(totalStr, 64); err == nil {
					data.Total = total
				}
			}
			continue
		}

		matches := funcCoveragePattern.FindStringSubmatch(line)
		if len(matches) == 5 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			funcName := matches[3]
			coverage, _ := strconv.ParseFloat(matches[4], 64)

			fc := functionCoverage{
				File:     file,
				Name:     funcName,
				Line:     lineNum,
				Coverage: coverage,
			}

			data.Functions[funcName] = fc

			if _, ok := data.ByFile[file]; !ok {
				data.ByFile[file] = coverage
			}
		}
	}

	return data, nil
}

func (a *CoverageAnalyzer) identifyGaps(data *coverageData) ([]Gap, error) {
	var gaps []Gap

	threshold := 70.0

	for _, fc := range data.Functions {
		if fc.Coverage < threshold {
			gap := Gap{
				File:     fc.File,
				Function: fc.Name,
				Line:     fc.Line,
				Coverage: fc.Coverage,
			}
			gaps = append(gaps, gap)
		}
	}

	for i := range gaps {
		suggestion, err := a.analyzeFunctionForTestSuggestion(gaps[i].File, gaps[i].Function)
		if err == nil {
			gaps[i].Suggestion = suggestion
		}
	}

	return gaps, nil
}

func (a *CoverageAnalyzer) analyzeFunctionForTestSuggestion(file, funcName string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	var funcDecl *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			if fd.Name.Name == funcName {
				funcDecl = fd
				return false
			}
		}
		return true
	})

	if funcDecl == nil {
		return "Add tests covering all code paths", nil
	}

	complexity := a.estimateComplexity(funcDecl)
	if complexity > 5 {
		return "Complex function - test edge cases, error paths, and boundary conditions", nil
	} else if complexity > 2 {
		return "Test main paths and error handling", nil
	}

	return "Add basic test coverage", nil
}

func (a *CoverageAnalyzer) estimateComplexity(funcDecl *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.CaseClause:
			complexity++
		}
		return true
	})
	return complexity
}

func (a *CoverageAnalyzer) generateReport(ctx context.Context, gaps []Gap, data *coverageData) (string, error) {
	if len(gaps) == 0 {
		return fmt.Sprintf("✅ Excellent coverage: %.1f%%\n\nAll functions meet the coverage threshold.", data.Total), nil
	}

	gapsSummary := []string{}
	for _, gap := range gaps {
		gapsSummary = append(gapsSummary, fmt.Sprintf("- %s:%d %s (%.1f%% coverage) - %s",
			gap.File, gap.Line, gap.Function, gap.Coverage, gap.Suggestion))
	}

	prompt := fmt.Sprintf(`Generate a concise coverage report.

Total Coverage: %.1f%%
Functions with low coverage (%d):
%s

Create a brief report with:
1. Overall assessment
2. Priority areas (most critical gaps)
3. Recommended next steps

Keep it under 10 lines.`, data.Total, len(gaps), strings.Join(gapsSummary, "\n"))

	resp, err := a.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a test coverage expert that provides actionable recommendations.",
		UserPrompt:   prompt,
		Model:        a.model,
	})

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text), nil
}

func (a *CoverageAnalyzer) generateBasicReport(gaps []Gap, data *coverageData) string {
	var sb strings.Builder

	sb.WriteString("Coverage Report\n")
	fmt.Fprintf(&sb, "Total Coverage: %.1f%%\n\n", data.Total)

	if len(gaps) == 0 {
		sb.WriteString("✅ All functions meet coverage threshold\n")
		return sb.String()
	}

	fmt.Fprintf(&sb, "⚠️  %d function(s) below 70%% coverage:\n\n", len(gaps))
	for _, gap := range gaps {
		fmt.Fprintf(&sb, "- %s:%d %s (%.1f%%)\n",
			filepath.Base(gap.File), gap.Line, gap.Function, gap.Coverage)
		if gap.Suggestion != "" {
			fmt.Fprintf(&sb, "  → %s\n", gap.Suggestion)
		}
	}

	return sb.String()
}
