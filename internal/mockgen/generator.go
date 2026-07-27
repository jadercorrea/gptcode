package mockgen

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

type MockGenerator struct {
	provider llm.Provider
	model    string
	workDir  string
}

type Interface struct {
	Name    string
	Package string
	Methods []Method
	File    string
}

type Method struct {
	Name    string
	Params  []Param
	Returns []Return
}

type Param struct {
	Name string
	Type string
}

type Return struct {
	Type string
}

type GenerateResult struct {
	MockFile string
	Valid    bool
	Error    error
}

func NewMockGenerator(provider llm.Provider, model, workDir string) *MockGenerator {
	return &MockGenerator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (g *MockGenerator) GenerateMock(ctx context.Context, sourceFile string) (*GenerateResult, error) {
	absPath := sourceFile
	if !filepath.IsAbs(sourceFile) {
		absPath = filepath.Join(g.workDir, sourceFile)
	}

	interfaces, err := g.parseInterfaces(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interfaces: %w", err)
	}

	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no interfaces found in %s", sourceFile)
	}

	mockCode, err := g.generateMockCode(ctx, interfaces)
	if err != nil {
		return nil, fmt.Errorf("failed to generate mock code: %w", err)
	}

	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	mockFile := filepath.Join(dir, fmt.Sprintf("%s_mock%s", name, ext))

	if err := os.WriteFile(mockFile, []byte(mockCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write mock file: %w", err)
	}

	result := &GenerateResult{
		MockFile: mockFile,
		Valid:    true,
	}

	if err := g.validateMock(mockFile); err != nil {
		result.Valid = false
		result.Error = err
	}

	return result, nil
}

func (g *MockGenerator) parseInterfaces(sourceFile string) ([]Interface, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var interfaces []Interface

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}

		iface := Interface{
			Name:    typeSpec.Name.Name,
			Package: node.Name.Name,
			File:    sourceFile,
			Methods: []Method{},
		}

		for _, field := range interfaceType.Methods.List {
			funcType, ok := field.Type.(*ast.FuncType)
			if !ok {
				continue
			}

			if len(field.Names) == 0 {
				continue
			}

			method := Method{
				Name:    field.Names[0].Name,
				Params:  []Param{},
				Returns: []Return{},
			}

			if funcType.Params != nil {
				for _, param := range funcType.Params.List {
					paramType := exprToString(param.Type)
					for _, name := range param.Names {
						method.Params = append(method.Params, Param{
							Name: name.Name,
							Type: paramType,
						})
					}
					if len(param.Names) == 0 {
						method.Params = append(method.Params, Param{
							Type: paramType,
						})
					}
				}
			}

			if funcType.Results != nil {
				for _, result := range funcType.Results.List {
					returnType := exprToString(result.Type)
					method.Returns = append(method.Returns, Return{
						Type: returnType,
					})
				}
			}

			iface.Methods = append(iface.Methods, method)
		}

		interfaces = append(interfaces, iface)
		return true
	})

	return interfaces, nil
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToString(e.Key), exprToString(e.Value))
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.ChanType:
		dir := ""
		switch e.Dir {
		case ast.SEND:
			dir = "chan<- "
		case ast.RECV:
			dir = "<-chan "
		default:
			dir = "chan "
		}
		return dir + exprToString(e.Value)
	case *ast.FuncType:
		return "func"
	default:
		return ""
	}
}

func (g *MockGenerator) generateMockCode(ctx context.Context, interfaces []Interface) (string, error) {
	var ifaceDescriptions []string
	for _, iface := range interfaces {
		desc := fmt.Sprintf("Interface: %s\n", iface.Name)
		for _, method := range iface.Methods {
			params := []string{}
			for _, p := range method.Params {
				if p.Name != "" {
					params = append(params, fmt.Sprintf("%s %s", p.Name, p.Type))
				} else {
					params = append(params, p.Type)
				}
			}
			returns := []string{}
			for _, r := range method.Returns {
				returns = append(returns, r.Type)
			}
			desc += fmt.Sprintf("  %s(%s)", method.Name, strings.Join(params, ", "))
			if len(returns) > 0 {
				if len(returns) == 1 {
					desc += fmt.Sprintf(" %s", returns[0])
				} else {
					desc += fmt.Sprintf(" (%s)", strings.Join(returns, ", "))
				}
			}
			desc += "\n"
		}
		ifaceDescriptions = append(ifaceDescriptions, desc)
	}

	prompt := fmt.Sprintf(`Generate a Go mock implementation for these interfaces.

%s

Rules:
- Use struct with function fields for each method
- Name mock struct as Mock<InterfaceName>
- Include New<MockName> constructor
- Support method call tracking (CallCount, Calls)
- Add SetReturn methods for configuring responses
- Package name: %s
- Clean, idiomatic Go code
- No external dependencies

Return ONLY the complete Go code, no explanations.`, strings.Join(ifaceDescriptions, "\n"), interfaces[0].Package)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that generates Go mock implementations for testing.",
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

func extractCode(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```go") || strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode || !strings.HasPrefix(trimmed, "```") {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func (g *MockGenerator) validateMock(mockFile string) error {
	return nil
}
