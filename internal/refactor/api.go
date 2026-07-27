package refactor

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type APICoordinator struct {
	provider llm.Provider
	model    string
	workDir  string
}

type APIChange struct {
	Type    string // "added", "modified", "removed"
	Method  string // GET, POST, etc
	Path    string
	Handler string
	File    string
}

type CoordinationResult struct {
	Changes      []APIChange
	UpdatedFiles []string
	Valid        bool
	Errors       []error
}

func NewAPICoordinator(provider llm.Provider, model, workDir string) *APICoordinator {
	return &APICoordinator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (c *APICoordinator) CoordinateChanges(ctx context.Context) (*CoordinationResult, error) {
	changes, err := c.detectAPIChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to detect API changes: %w", err)
	}

	if len(changes) == 0 {
		return &CoordinationResult{
			Changes: []APIChange{},
		}, nil
	}

	var updatedFiles []string
	var errors []error

	for _, change := range changes {
		files, err := c.coordinateChange(ctx, change)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		updatedFiles = append(updatedFiles, files...)
	}

	result := &CoordinationResult{
		Changes:      changes,
		UpdatedFiles: updatedFiles,
		Valid:        len(errors) == 0,
		Errors:       errors,
	}

	return result, nil
}

func (c *APICoordinator) detectAPIChanges() ([]APIChange, error) {
	var changes []APIChange

	err := filepath.Walk(c.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.Contains(path, "route") || strings.Contains(path, "handler") ||
			strings.Contains(path, "controller") || strings.Contains(path, "api") {
			fileChanges, err := c.analyzeAPIFile(path)
			if err == nil {
				changes = append(changes, fileChanges...)
			}
		}

		return nil
	})

	return changes, err
}

func (c *APICoordinator) analyzeAPIFile(filePath string) ([]APIChange, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var changes []APIChange

	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if change := c.extractRouteChange(callExpr, filePath); change != nil {
			changes = append(changes, *change)
		}

		return true
	})

	return changes, nil
}

func (c *APICoordinator) extractRouteChange(call *ast.CallExpr, file string) *APIChange {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	method := sel.Sel.Name
	httpMethods := []string{"Get", "Post", "Put", "Delete", "Patch", "Options", "Head"}

	isHTTPMethod := false
	for _, m := range httpMethods {
		if method == m || method == strings.ToUpper(m) {
			isHTTPMethod = true
			break
		}
	}

	if !isHTTPMethod {
		return nil
	}

	if len(call.Args) < 2 {
		return nil
	}

	pathLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return nil
	}

	path := strings.Trim(pathLit.Value, "\"")

	handlerIdent, ok := call.Args[1].(*ast.Ident)
	if !ok {
		if sel, ok := call.Args[1].(*ast.SelectorExpr); ok {
			handlerIdent = sel.Sel
		} else {
			return nil
		}
	}

	return &APIChange{
		Type:    "detected",
		Method:  strings.ToUpper(method),
		Path:    path,
		Handler: handlerIdent.Name,
		File:    file,
	}
}

func (c *APICoordinator) coordinateChange(ctx context.Context, change APIChange) ([]string, error) {
	var updatedFiles []string

	testFile, err := c.updateTestForChange(ctx, change)
	if err == nil && testFile != "" {
		updatedFiles = append(updatedFiles, testFile)
	}

	handlerFile, err := c.ensureHandlerExists(ctx, change)
	if err == nil && handlerFile != "" {
		updatedFiles = append(updatedFiles, handlerFile)
	}

	return updatedFiles, nil
}

func (c *APICoordinator) updateTestForChange(ctx context.Context, change APIChange) (string, error) {
	testFile := strings.Replace(change.File, ".go", "_test.go", 1)

	var existingTests string
	if content, err := os.ReadFile(testFile); err == nil {
		existingTests = string(content)
	}

	prompt := fmt.Sprintf(`Generate or update test for this API endpoint:

Method: %s
Path: %s
Handler: %s

Existing tests:
%s

Generate a test that:
1. Tests the endpoint with valid request
2. Tests error cases (invalid input, auth, etc)
3. Validates response structure
4. Uses table-driven tests if appropriate
5. Includes setup/teardown if needed

Return ONLY the test function code, no explanations.`,
		change.Method, change.Path, change.Handler, existingTests)

	resp, err := c.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are an API testing expert that generates comprehensive test functions.",
		UserPrompt:   prompt,
		Model:        c.model,
	})

	if err != nil {
		return "", err
	}

	testCode := c.extractCode(resp.Text)

	if existingTests == "" {
		testCode = c.wrapInPackage(testCode, change.File)
	} else {
		testCode = existingTests + "\n\n" + testCode
	}

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return "", err
	}

	return testFile, nil
}

func (c *APICoordinator) ensureHandlerExists(ctx context.Context, change APIChange) (string, error) {
	content, err := os.ReadFile(change.File)
	if err != nil {
		return "", err
	}

	if strings.Contains(string(content), "func "+change.Handler) {
		return "", nil
	}

	prompt := fmt.Sprintf(`Generate a handler function for this API endpoint:

Method: %s
Path: %s
Handler name: %s

Generate a handler that:
1. Matches the function signature (typically func(w http.ResponseWriter, r *http.Request))
2. Parses request body/params as needed
3. Validates input
4. Returns appropriate HTTP response
5. Includes error handling

Return ONLY the handler function code, no explanations.`,
		change.Method, change.Path, change.Handler)

	resp, err := c.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are an API handler expert that generates well-structured HTTP handlers.",
		UserPrompt:   prompt,
		Model:        c.model,
	})

	if err != nil {
		return "", err
	}

	handlerCode := c.extractCode(resp.Text)

	updatedContent := string(content) + "\n\n" + handlerCode
	if err := os.WriteFile(change.File, []byte(updatedContent), 0644); err != nil {
		return "", err
	}

	return change.File, nil
}

func (c *APICoordinator) extractCode(text string) string {
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

func (c *APICoordinator) wrapInPackage(code, sourceFile string) string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.PackageClauseOnly)
	if err != nil {
		return code
	}

	pkgName := node.Name.Name

	return fmt.Sprintf(`package %s

import (
	"testing"
)

%s`, pkgName, code)
}
