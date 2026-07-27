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

type TypeChange struct {
	TypeName   string
	File       string
	ChangeType string
	OldDef     string
	NewDef     string
	Usages     []TypeUsage
	Impact     string
}

type TypeUsage struct {
	File     string
	Line     int
	Context  string
	Function string
}

type TypeRefactorResult struct {
	Changes      []TypeChange
	UpdatedFiles []string
	ImpactReport string
	Errors       []error
}

type TypeRefactor struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewTypeRefactor(provider llm.Provider, model, workDir string) *TypeRefactor {
	return &TypeRefactor{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (r *TypeRefactor) RefactorType(ctx context.Context, typeName, newDefinition string, propagate bool) (*TypeRefactorResult, error) {
	typeInfo, err := r.findTypeDefinition(typeName)
	if err != nil {
		return nil, fmt.Errorf("failed to find type: %w", err)
	}

	usages, err := r.findTypeUsages(typeName)
	if err != nil {
		return nil, fmt.Errorf("failed to find usages: %w", err)
	}

	impact, err := r.analyzeImpact(ctx, typeInfo, usages)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze impact: %w", err)
	}

	result := &TypeRefactorResult{
		ImpactReport: impact,
	}

	if !propagate {
		return result, nil
	}

	if err := r.updateTypeDefinition(ctx, typeInfo, newDefinition); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to update definition: %w", err))
	} else {
		result.UpdatedFiles = append(result.UpdatedFiles, typeInfo.File)
	}

	for _, usage := range usages {
		if err := r.updateTypeUsage(ctx, usage, typeInfo, newDefinition); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to update %s: %w", usage.File, err))
		} else {
			if !contains(result.UpdatedFiles, usage.File) {
				result.UpdatedFiles = append(result.UpdatedFiles, usage.File)
			}
		}
	}

	return result, nil
}

func (r *TypeRefactor) findTypeDefinition(typeName string) (*TypeChange, error) {
	var found *TypeChange

	err := filepath.Walk(r.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
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
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != typeName {
					continue
				}

				found = &TypeChange{
					TypeName: typeName,
					File:     path,
					OldDef:   r.formatTypeSpec(typeSpec),
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
		return nil, fmt.Errorf("type %s not found", typeName)
	}

	return found, nil
}

func (r *TypeRefactor) findTypeUsages(typeName string) ([]TypeUsage, error) {
	cmd := exec.Command("grep", "-rn", "--include=*.go", typeName, r.workDir)
	output, err := cmd.Output()
	if err != nil {
		return []TypeUsage{}, nil
	}

	var usages []TypeUsage
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		var lineNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)

		usages = append(usages, TypeUsage{
			File:    parts[0],
			Line:    lineNum,
			Context: strings.TrimSpace(parts[2]),
		})
	}

	return usages, nil
}

func (r *TypeRefactor) analyzeImpact(ctx context.Context, typeInfo *TypeChange, usages []TypeUsage) (string, error) {
	usagesList := make([]string, 0, len(usages))
	for _, u := range usages {
		usagesList = append(usagesList, fmt.Sprintf("%s:%d - %s", u.File, u.Line, u.Context))
	}

	prompt := fmt.Sprintf(`Analyze the impact of changing this type:

Type: %s
Current definition: %s
Location: %s

Found %d usage(s):
%s

Provide:
1. Impact severity (Low/Medium/High/Critical)
2. Breaking changes list
3. Required updates in consuming code
4. Migration complexity estimate
5. Testing recommendations

Be concise and actionable.`, typeInfo.TypeName, typeInfo.OldDef, typeInfo.File,
		len(usages), strings.Join(usagesList[:min(10, len(usagesList))], "\n"))

	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a type refactoring expert that analyzes impact and breaking changes.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func (r *TypeRefactor) updateTypeDefinition(ctx context.Context, typeInfo *TypeChange, newDef string) error {
	content, err := os.ReadFile(typeInfo.File)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Update type definition:

Type: %s
Current: %s
New: %s

File content:
%s

Return ONLY the complete updated file content.`,
		typeInfo.TypeName, typeInfo.OldDef, newDef, string(content))

	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a type definition expert that updates type definitions accurately.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return err
	}

	updated := r.extractCode(resp.Text)
	return os.WriteFile(typeInfo.File, []byte(updated), 0644)
}

func (r *TypeRefactor) updateTypeUsage(ctx context.Context, usage TypeUsage, typeInfo *TypeChange, newDef string) error {
	content, err := os.ReadFile(usage.File)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Update code for type change:

Type: %s changed
New definition: %s
Usage at line %d: %s

File:
%s

Update to be compatible with new type. Return ONLY the complete updated file.`,
		typeInfo.TypeName, newDef, usage.Line, usage.Context, string(content))

	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a type usage expert that updates code to be compatible with type changes.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return err
	}

	updated := r.extractCode(resp.Text)
	return os.WriteFile(usage.File, []byte(updated), 0644)
}

func (r *TypeRefactor) formatTypeSpec(spec *ast.TypeSpec) string {
	return fmt.Sprintf("type %s %s", spec.Name.Name, r.exprToString(spec.Type))
}

func (r *TypeRefactor) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + r.exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + r.exprToString(e.Elt)
	case *ast.StructType:
		return "struct{...}"
	case *ast.InterfaceType:
		return "interface{...}"
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", r.exprToString(e.Key), r.exprToString(e.Value))
	default:
		return "unknown"
	}
}

func (r *TypeRefactor) extractCode(text string) string {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
