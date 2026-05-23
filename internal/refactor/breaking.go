package refactor

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gptcode/internal/llm"
)

type BreakingChange struct {
	Type        string
	Package     string
	Symbol      string
	OldAPI      string
	NewAPI      string
	File        string
	Description string
}

type Consumer struct {
	File       string
	Package    string
	ImportPath string
	Line       int
	Usage      string
}

type BreakingChangeResult struct {
	Changes       []BreakingChange
	Consumers     map[string][]Consumer
	UpdatedFiles  []string
	MigrationPlan string
	Errors        []error
}

type BreakingCoordinator struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewBreakingCoordinator(provider llm.Provider, model, workDir string) *BreakingCoordinator {
	return &BreakingCoordinator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (c *BreakingCoordinator) DetectAndCoordinate(ctx context.Context) (*BreakingChangeResult, error) {
	changes, err := c.detectBreakingChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to detect breaking changes: %w", err)
	}

	if len(changes) == 0 {
		return &BreakingChangeResult{}, nil
	}

	consumersMap := make(map[string][]Consumer)
	for _, change := range changes {
		consumers, err := c.findConsumers(change)
		if err != nil {
			return nil, fmt.Errorf("failed to find consumers for %s: %w", change.Symbol, err)
		}
		key := fmt.Sprintf("%s.%s", change.Package, change.Symbol)
		consumersMap[key] = consumers
	}

	plan, err := c.generateMigrationPlan(ctx, changes, consumersMap)
	if err != nil {
		return nil, fmt.Errorf("failed to generate migration plan: %w", err)
	}

	var updatedFiles []string
	var errors []error

	for _, change := range changes {
		key := fmt.Sprintf("%s.%s", change.Package, change.Symbol)
		consumers := consumersMap[key]

		for _, consumer := range consumers {
			if err := c.updateConsumer(ctx, consumer, change); err != nil {
				errors = append(errors, fmt.Errorf("failed to update %s: %w", consumer.File, err))
			} else {
				if !contains(updatedFiles, consumer.File) {
					updatedFiles = append(updatedFiles, consumer.File)
				}
			}
		}
	}

	return &BreakingChangeResult{
		Changes:       changes,
		Consumers:     consumersMap,
		UpdatedFiles:  updatedFiles,
		MigrationPlan: plan,
		Errors:        errors,
	}, nil
}

func (c *BreakingCoordinator) detectBreakingChanges() ([]BreakingChange, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = c.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	var changes []BreakingChange

	for _, file := range changedFiles {
		if file == "" || !strings.HasSuffix(file, ".go") {
			continue
		}

		fullPath := filepath.Join(c.workDir, file)
		fileChanges, err := c.analyzeFileChanges(fullPath)
		if err != nil {
			continue
		}
		changes = append(changes, fileChanges...)
	}

	return changes, nil
}

func (c *BreakingCoordinator) analyzeFileChanges(file string) ([]BreakingChange, error) {
	currentContent, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(c.workDir, file)
	cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", relPath))
	cmd.Dir = c.workDir
	oldContent, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	currentAST, err := c.parseFile(currentContent)
	if err != nil {
		return nil, err
	}

	oldAST, err := c.parseFile(oldContent)
	if err != nil {
		return nil, err
	}

	return c.compareASTs(file, oldAST, currentAST), nil
}

func (c *BreakingCoordinator) parseFile(content []byte) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, "", content, parser.ParseComments)
}

func (c *BreakingCoordinator) compareASTs(file string, oldAST, newAST *ast.File) []BreakingChange {
	var changes []BreakingChange

	oldFuncs := c.extractFunctions(oldAST)
	newFuncs := c.extractFunctions(newAST)

	for name, oldFunc := range oldFuncs {
		newFunc, exists := newFuncs[name]

		if !exists {
			changes = append(changes, BreakingChange{
				Type:        "function_removed",
				Package:     oldAST.Name.Name,
				Symbol:      name,
				OldAPI:      oldFunc,
				File:        file,
				Description: fmt.Sprintf("Function %s was removed", name),
			})
			continue
		}

		if oldFunc != newFunc {
			changes = append(changes, BreakingChange{
				Type:        "signature_changed",
				Package:     oldAST.Name.Name,
				Symbol:      name,
				OldAPI:      oldFunc,
				NewAPI:      newFunc,
				File:        file,
				Description: fmt.Sprintf("Function %s signature changed", name),
			})
		}
	}

	oldTypes := c.extractTypes(oldAST)
	newTypes := c.extractTypes(newAST)

	for name, oldType := range oldTypes {
		newType, exists := newTypes[name]

		if !exists {
			changes = append(changes, BreakingChange{
				Type:        "type_removed",
				Package:     oldAST.Name.Name,
				Symbol:      name,
				OldAPI:      oldType,
				File:        file,
				Description: fmt.Sprintf("Type %s was removed", name),
			})
			continue
		}

		if oldType != newType {
			changes = append(changes, BreakingChange{
				Type:        "type_changed",
				Package:     oldAST.Name.Name,
				Symbol:      name,
				OldAPI:      oldType,
				NewAPI:      newType,
				File:        file,
				Description: fmt.Sprintf("Type %s definition changed", name),
			})
		}
	}

	return changes
}

func (c *BreakingCoordinator) extractFunctions(file *ast.File) map[string]string {
	funcs := make(map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || !fn.Name.IsExported() {
			return true
		}

		sig := c.formatFuncSignature(fn)
		funcs[fn.Name.Name] = sig
		return true
	})

	return funcs
}

func (c *BreakingCoordinator) extractTypes(file *ast.File) map[string]string {
	types := make(map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !typeSpec.Name.IsExported() {
				continue
			}

			typeDef := c.formatTypeSpec(typeSpec)
			types[typeSpec.Name.Name] = typeDef
		}
		return true
	})

	return types
}

func (c *BreakingCoordinator) formatFuncSignature(fn *ast.FuncDecl) string {
	var params []string
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeStr := c.exprToString(field.Type)
			if len(field.Names) > 0 {
				for _, name := range field.Names {
					params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
				}
			} else {
				params = append(params, typeStr)
			}
		}
	}

	var results []string
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			results = append(results, c.exprToString(field.Type))
		}
	}

	sig := fmt.Sprintf("func %s(%s)", fn.Name.Name, strings.Join(params, ", "))
	if len(results) > 0 {
		if len(results) == 1 {
			sig += " " + results[0]
		} else {
			sig += " (" + strings.Join(results, ", ") + ")"
		}
	}
	return sig
}

func (c *BreakingCoordinator) formatTypeSpec(spec *ast.TypeSpec) string {
	return fmt.Sprintf("type %s %s", spec.Name.Name, c.exprToString(spec.Type))
}

func (c *BreakingCoordinator) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + c.exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + c.exprToString(e.Elt)
	case *ast.SelectorExpr:
		return c.exprToString(e.X) + "." + e.Sel.Name
	case *ast.StructType:
		return "struct{...}"
	case *ast.InterfaceType:
		return "interface{...}"
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", c.exprToString(e.Key), c.exprToString(e.Value))
	case *ast.FuncType:
		return "func(...)"
	case *ast.Ellipsis:
		return "..." + c.exprToString(e.Elt)
	default:
		return "unknown"
	}
}

func (c *BreakingCoordinator) findConsumers(change BreakingChange) ([]Consumer, error) {
	cmd := exec.Command("grep", "-rn", "--include=*.go", change.Symbol, c.workDir)
	output, err := cmd.Output()
	if err != nil {
		return []Consumer{}, nil
	}

	var consumers []Consumer
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		file := parts[0]
		if file == change.File {
			continue
		}

		var lineNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)

		pkg, importPath := c.getPackageInfo(file)

		consumers = append(consumers, Consumer{
			File:       file,
			Package:    pkg,
			ImportPath: importPath,
			Line:       lineNum,
			Usage:      strings.TrimSpace(parts[2]),
		})
	}

	return consumers, nil
}

func (c *BreakingCoordinator) getPackageInfo(file string) (string, string) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", ""
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, content, parser.PackageClauseOnly)
	if err != nil {
		return "", ""
	}

	return node.Name.Name, ""
}

func (c *BreakingCoordinator) generateMigrationPlan(ctx context.Context, changes []BreakingChange, consumers map[string][]Consumer) (string, error) {
	var changeDesc strings.Builder
	for i, change := range changes {
		fmt.Fprintf(&changeDesc, "%d. %s: %s\n", i+1, change.Type, change.Description)
		fmt.Fprintf(&changeDesc, "   Old: %s\n", change.OldAPI)
		if change.NewAPI != "" {
			fmt.Fprintf(&changeDesc, "   New: %s\n", change.NewAPI)
		}

		key := fmt.Sprintf("%s.%s", change.Package, change.Symbol)
		if cons, ok := consumers[key]; ok {
			fmt.Fprintf(&changeDesc, "   Affected files: %d\n", len(cons))
		}
	}

	prompt := fmt.Sprintf(`Generate a migration plan for these breaking changes:

%s

Provide:
1. Migration strategy (gradual vs immediate)
2. Deprecation timeline if applicable
3. Code update patterns
4. Testing strategy
5. Rollback plan

Format as markdown, be concise.`, changeDesc.String())

	resp, err := c.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a breaking change expert that creates detailed migration plans.",
		UserPrompt:   prompt,
		Model:        c.model,
	})

	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func (c *BreakingCoordinator) updateConsumer(ctx context.Context, consumer Consumer, change BreakingChange) error {
	content, err := os.ReadFile(consumer.File)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Update code to handle breaking change:

Change type: %s
Symbol: %s
Old API: %s
New API: %s
Description: %s

Usage at line %d: %s

File content:
%s

Update the code to use the new API. Return ONLY the complete updated file content.`,
		change.Type, change.Symbol, change.OldAPI, change.NewAPI, change.Description,
		consumer.Line, consumer.Usage, string(content))

	resp, err := c.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a code update expert that adapts code to new API changes.",
		UserPrompt:   prompt,
		Model:        c.model,
	})

	if err != nil {
		return err
	}

	updated := c.extractCode(resp.Text)
	return os.WriteFile(consumer.File, []byte(updated), 0644)
}

func (c *BreakingCoordinator) extractCode(text string) string {
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
