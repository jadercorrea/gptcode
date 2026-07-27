package testgen

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type IntegrationTestGenerator struct {
	provider llm.Provider
	model    string
	workDir  string
}

type Component struct {
	Name         string
	File         string
	Type         string // handler, service, repository, client
	Dependencies []string
	Methods      []string
}

type IntegrationResult struct {
	TestFile string
	Valid    bool
	Error    error
}

func NewIntegrationTestGenerator(provider llm.Provider, model, workDir string) *IntegrationTestGenerator {
	return &IntegrationTestGenerator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (g *IntegrationTestGenerator) GenerateIntegrationTests(ctx context.Context, packagePath string) (*IntegrationResult, error) {
	absPath := packagePath
	if !filepath.IsAbs(packagePath) {
		absPath = filepath.Join(g.workDir, packagePath)
	}

	lang := g.detectLanguage(absPath)
	if lang != langdetect.Go {
		return nil, fmt.Errorf("integration test generation currently only supports Go")
	}

	components, err := g.analyzeComponents(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze components: %w", err)
	}

	if len(components) == 0 {
		return nil, fmt.Errorf("no components found in %s", packagePath)
	}

	testCode, err := g.generateIntegrationTestCode(ctx, components, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate test code: %w", err)
	}

	testFile := filepath.Join(absPath, "integration_test.go")
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write test file: %w", err)
	}

	result := &IntegrationResult{
		TestFile: testFile,
		Valid:    true,
	}

	validator := NewValidator(g.workDir, langdetect.Go)
	if !validator.Validate(testFile) {
		result.Valid = false
		result.Error = fmt.Errorf("validation failed")
	}

	return result, nil
}

func (g *IntegrationTestGenerator) detectLanguage(path string) langdetect.Language {
	detector := langdetect.NewDetector(path)
	breakdown, err := detector.Detect()
	if err != nil {
		return langdetect.Unknown
	}
	if breakdown.Primary == "" {
		return langdetect.Unknown
	}

	switch breakdown.Primary {
	case "Go":
		return langdetect.Go
	case "TypeScript", "JavaScript":
		return langdetect.TypeScript
	case "Python":
		return langdetect.Python
	default:
		return langdetect.Unknown
	}
}

func (g *IntegrationTestGenerator) analyzeComponents(pkgPath string) ([]Component, error) {
	var components []Component

	err := filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		comp, err := g.analyzeFile(path)
		if err != nil {
			return nil
		}

		if comp != nil {
			components = append(components, *comp)
		}

		return nil
	})

	return components, err
}

func (g *IntegrationTestGenerator) analyzeFile(filePath string) (*Component, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var comp *Component

	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.TYPE {
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					name := typeSpec.Name.Name
					compType := g.inferComponentType(name, structType)

					if compType != "" {
						comp = &Component{
							Name:         name,
							File:         filePath,
							Type:         compType,
							Dependencies: g.extractDependencies(structType),
							Methods:      []string{},
						}
					}
				}
			}

		case *ast.FuncDecl:
			if comp != nil && decl.Recv != nil {
				comp.Methods = append(comp.Methods, decl.Name.Name)
			}
		}
		return true
	})

	return comp, nil
}

func (g *IntegrationTestGenerator) inferComponentType(name string, structType *ast.StructType) string {
	nameLower := strings.ToLower(name)

	if strings.Contains(nameLower, "handler") || strings.Contains(nameLower, "controller") {
		return "handler"
	}
	if strings.Contains(nameLower, "service") {
		return "service"
	}
	if strings.Contains(nameLower, "repository") || strings.Contains(nameLower, "store") {
		return "repository"
	}
	if strings.Contains(nameLower, "client") {
		return "client"
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		fieldName := strings.ToLower(field.Names[0].Name)
		if strings.Contains(fieldName, "db") || strings.Contains(fieldName, "store") {
			return "service"
		}
	}

	return ""
}

func (g *IntegrationTestGenerator) extractDependencies(structType *ast.StructType) []string {
	var deps []string

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		fieldType := g.exprToString(field.Type)
		if fieldType != "" && !isBasicType(fieldType) {
			deps = append(deps, fieldType)
		}
	}

	return deps
}

func (g *IntegrationTestGenerator) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return g.exprToString(e.X)
	case *ast.SelectorExpr:
		return g.exprToString(e.X) + "." + e.Sel.Name
	default:
		return ""
	}
}

func isBasicType(typ string) bool {
	basic := []string{"string", "int", "int64", "float64", "bool", "byte", "rune"}
	for _, b := range basic {
		if typ == b {
			return true
		}
	}
	return false
}

func (g *IntegrationTestGenerator) generateIntegrationTestCode(ctx context.Context, components []Component, pkgPath string) (string, error) {
	pkgName := filepath.Base(pkgPath)

	var compDescriptions []string
	for _, comp := range components {
		desc := fmt.Sprintf("- %s (%s): %v", comp.Name, comp.Type, comp.Methods)
		if len(comp.Dependencies) > 0 {
			desc += fmt.Sprintf("\n  Dependencies: %v", comp.Dependencies)
		}
		compDescriptions = append(compDescriptions, desc)
	}

	prompt := fmt.Sprintf(`Generate Go integration tests for these components:

Package: %s

Components:
%s

Create integration tests that:
1. Test interactions between components (not isolated units)
2. Use real dependencies where possible, test doubles where needed
3. Include setup/teardown for resources
4. Test complete workflows end-to-end
5. Handle errors and edge cases
6. Use table-driven tests where appropriate

Requirements:
- Package name: %s
- Build tag: //go:build integration
- Use testing.T
- Include TestMain for setup/teardown if needed
- Add cleanup with t.Cleanup()
- Clear test names describing scenarios

Return ONLY the complete Go test code, no explanations.`, pkgName, strings.Join(compDescriptions, "\n"), pkgName)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are an integration testing expert that generates comprehensive end-to-end tests.",
		UserPrompt:   prompt,
		Model:        g.model,
	})

	if err != nil {
		return "", err
	}

	code := strings.TrimSpace(resp.Text)
	code = extractCode(code)

	return code, nil
}
