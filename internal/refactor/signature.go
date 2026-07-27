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

	"github.com/jadercorrea/gptcode/internal/llm"
)

type SignatureRefactor struct {
	provider llm.Provider
	model    string
	workDir  string
}

type FunctionSignature struct {
	Package  string
	Function string
	File     string
	Params   []string
	Returns  []string
}

type FunctionUsage struct {
	File     string
	Line     int
	CallSite string
}

type RefactorResult struct {
	Function     string
	OldSignature string
	NewSignature string
	UpdatedFiles []string
	Errors       []error
}

func NewSignatureRefactor(provider llm.Provider, model, workDir string) *SignatureRefactor {
	return &SignatureRefactor{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (r *SignatureRefactor) RefactorSignature(ctx context.Context, funcName, newSignature string) (*RefactorResult, error) {
	funcDef, err := r.findFunction(funcName)
	if err != nil {
		return nil, fmt.Errorf("failed to find function: %w", err)
	}

	usages, err := r.findUsages(funcName)
	if err != nil {
		return nil, fmt.Errorf("failed to find usages: %w", err)
	}

	oldSig := r.formatSignature(funcDef)

	var updatedFiles []string
	var errors []error

	if err := r.updateFunctionDefinition(ctx, funcDef, newSignature); err != nil {
		errors = append(errors, fmt.Errorf("failed to update definition: %w", err))
	} else {
		updatedFiles = append(updatedFiles, funcDef.File)
	}

	for _, usage := range usages {
		if err := r.updateCallSite(ctx, usage, funcDef, newSignature); err != nil {
			errors = append(errors, fmt.Errorf("failed to update %s:%d: %w", usage.File, usage.Line, err))
		} else {
			if !contains(updatedFiles, usage.File) {
				updatedFiles = append(updatedFiles, usage.File)
			}
		}
	}

	result := &RefactorResult{
		Function:     funcName,
		OldSignature: oldSig,
		NewSignature: newSignature,
		UpdatedFiles: updatedFiles,
		Errors:       errors,
	}

	return result, nil
}

func (r *SignatureRefactor) findFunction(funcName string) (*FunctionSignature, error) {
	var found *FunctionSignature

	err := filepath.Walk(r.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, content, 0)
		if err != nil {
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			if funcDecl.Name.Name == funcName {
				found = &FunctionSignature{
					Package:  node.Name.Name,
					Function: funcName,
					File:     path,
					Params:   r.extractParams(funcDecl),
					Returns:  r.extractReturns(funcDecl),
				}
				return false
			}
			return true
		})

		if found != nil {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if found == nil {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	return found, nil
}

func (r *SignatureRefactor) extractParams(funcDecl *ast.FuncDecl) []string {
	if funcDecl.Type.Params == nil {
		return nil
	}

	var params []string
	for _, field := range funcDecl.Type.Params.List {
		typeStr := r.exprToString(field.Type)
		for _, name := range field.Names {
			params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
		}
		if len(field.Names) == 0 {
			params = append(params, typeStr)
		}
	}
	return params
}

func (r *SignatureRefactor) extractReturns(funcDecl *ast.FuncDecl) []string {
	if funcDecl.Type.Results == nil {
		return nil
	}

	var returns []string
	for _, field := range funcDecl.Type.Results.List {
		returns = append(returns, r.exprToString(field.Type))
	}
	return returns
}

func (r *SignatureRefactor) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + r.exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + r.exprToString(e.Elt)
	case *ast.SelectorExpr:
		return r.exprToString(e.X) + "." + e.Sel.Name
	case *ast.Ellipsis:
		return "..." + r.exprToString(e.Elt)
	default:
		return ""
	}
}

func (r *SignatureRefactor) formatSignature(fn *FunctionSignature) string {
	params := strings.Join(fn.Params, ", ")
	returns := ""
	if len(fn.Returns) > 0 {
		if len(fn.Returns) == 1 {
			returns = " " + fn.Returns[0]
		} else {
			returns = " (" + strings.Join(fn.Returns, ", ") + ")"
		}
	}
	return fmt.Sprintf("func %s(%s)%s", fn.Function, params, returns)
}

func (r *SignatureRefactor) findUsages(funcName string) ([]FunctionUsage, error) {
	cmd := exec.Command("grep", "-rn", "--include=*.go", funcName, r.workDir)
	output, err := cmd.Output()
	if err != nil {
		return []FunctionUsage{}, nil
	}

	var usages []FunctionUsage
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
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		var lineNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)

		if strings.Contains(parts[2], funcName+"(") {
			usages = append(usages, FunctionUsage{
				File:     file,
				Line:     lineNum,
				CallSite: strings.TrimSpace(parts[2]),
			})
		}
	}

	return usages, nil
}

func (r *SignatureRefactor) updateFunctionDefinition(ctx context.Context, fn *FunctionSignature, newSig string) error {
	content, err := os.ReadFile(fn.File)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Update this function definition to the new signature:

Current: %s
New: func %s%s

Source file content:
%s

Return ONLY the complete updated file content, no explanations.`,
		r.formatSignature(fn), fn.Function, newSig, string(content))

	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a function signature refactoring expert that updates function definitions accurately.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return err
	}

	updated := r.extractCode(resp.Text)
	return os.WriteFile(fn.File, []byte(updated), 0644)
}

func (r *SignatureRefactor) updateCallSite(ctx context.Context, usage FunctionUsage, fn *FunctionSignature, newSig string) error {
	content, err := os.ReadFile(usage.File)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Update function call site to match new signature:

Function: %s
New signature: func %s%s
Call site at line %d: %s

Source file:
%s

Update the call site to match the new signature. Return ONLY the complete updated file content.`,
		fn.Function, fn.Function, newSig, usage.Line, usage.CallSite, string(content))

	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a function call site expert that updates function calls to match new signatures.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return err
	}

	updated := r.extractCode(resp.Text)
	return os.WriteFile(usage.File, []byte(updated), 0644)
}

func (r *SignatureRefactor) extractCode(text string) string {
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
