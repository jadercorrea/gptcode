package docs

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gptcode/internal/langdetect"
	"gptcode/internal/llm"
)

type APIEndpoint struct {
	Method      string
	Path        string
	Handler     string
	Description string
	Params      []string
	Returns     string
	File        string
	Line        int
}

type APIDocGenerator struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewAPIDocGenerator(provider llm.Provider, model, workDir string) *APIDocGenerator {
	return &APIDocGenerator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (g *APIDocGenerator) Generate(ctx context.Context, format string) (string, error) {
	lang := langdetect.DetectLanguage(g.workDir)

	endpoints, err := g.discoverEndpoints(lang)
	if err != nil {
		return "", fmt.Errorf("failed to discover endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		return "", fmt.Errorf("no API endpoints found")
	}

	doc, err := g.generateDocumentation(ctx, endpoints, format)
	if err != nil {
		return "", err
	}

	filename := g.getOutputFilename(format)
	if err := os.WriteFile(filename, []byte(doc), 0644); err != nil {
		return "", fmt.Errorf("failed to write documentation: %w", err)
	}

	return filename, nil
}

func (g *APIDocGenerator) discoverEndpoints(lang langdetect.Language) ([]APIEndpoint, error) {
	var endpoints []APIEndpoint

	err := filepath.Walk(g.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if strings.Contains(path, "vendor/") || strings.Contains(path, "node_modules/") {
			return nil
		}

		switch lang {
		case langdetect.Go:
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				eps, _ := g.parseGoFile(path)
				endpoints = append(endpoints, eps...)
			}
		case langdetect.TypeScript:
			if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js") {
				eps, _ := g.parseTypeScriptFile(path)
				endpoints = append(endpoints, eps...)
			}
		}

		return nil
	})

	return endpoints, err
}

func (g *APIDocGenerator) parseGoFile(path string) ([]APIEndpoint, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var endpoints []APIEndpoint
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		method := strings.ToUpper(sel.Sel.Name)
		if !contains(httpMethods, method) {
			return true
		}

		if len(call.Args) < 2 {
			return true
		}

		pathLit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}

		handlerIdent, ok := call.Args[1].(*ast.Ident)
		if !ok {
			return true
		}

		endpoint := APIEndpoint{
			Method:  method,
			Path:    strings.Trim(pathLit.Value, "\""),
			Handler: handlerIdent.Name,
			File:    path,
			Line:    fset.Position(call.Pos()).Line,
		}

		endpoints = append(endpoints, endpoint)
		return true
	})

	return endpoints, nil
}

func (g *APIDocGenerator) parseTypeScriptFile(path string) ([]APIEndpoint, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var endpoints []APIEndpoint
	lines := strings.Split(string(content), "\n")

	httpMethods := []string{"get", "post", "put", "delete", "patch"}
	c := cases.Title(language.English)

	for i, line := range lines {
		for _, method := range httpMethods {
			if strings.Contains(line, fmt.Sprintf(".%s(", method)) ||
				strings.Contains(line, fmt.Sprintf("@%s(", c.String(method))) {

				pathStart := strings.Index(line, "\"")
				if pathStart == -1 {
					pathStart = strings.Index(line, "'")
				}
				if pathStart == -1 {
					continue
				}

				remaining := line[pathStart+1:]
				pathEnd := strings.IndexAny(remaining, "\"'")
				if pathEnd == -1 {
					continue
				}

				endpoint := APIEndpoint{
					Method: strings.ToUpper(method),
					Path:   remaining[:pathEnd],
					File:   path,
					Line:   i + 1,
				}
				endpoints = append(endpoints, endpoint)
				break
			}
		}
	}

	return endpoints, nil
}

func (g *APIDocGenerator) generateDocumentation(ctx context.Context, endpoints []APIEndpoint, format string) (string, error) {
	endpointList := g.formatEndpointList(endpoints)

	var formatInstruction string
	switch format {
	case "openapi":
		formatInstruction = "Generate OpenAPI 3.0 specification in YAML format"
	case "markdown":
		formatInstruction = "Generate Markdown documentation with clear sections"
	case "postman":
		formatInstruction = "Generate Postman Collection v2.1 in JSON format"
	default:
		formatInstruction = "Generate Markdown documentation"
	}

	prompt := fmt.Sprintf(`Generate API documentation for these endpoints:

%s

%s.

Include:
- Full endpoint details
- Request/response examples
- Authentication requirements (if detected)
- Error responses

Return ONLY the documentation, no explanations.`, endpointList, formatInstruction)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are an API documentation expert that generates comprehensive, professional documentation.",
		UserPrompt:   prompt,
		Model:        g.model,
	})

	if err != nil {
		return "", err
	}

	return g.extractDoc(resp.Text, format), nil
}

func (g *APIDocGenerator) formatEndpointList(endpoints []APIEndpoint) string {
	var builder strings.Builder

	for i, ep := range endpoints {
		fmt.Fprintf(&builder, "%d. %s %s\n", i+1, ep.Method, ep.Path)
		if ep.Handler != "" {
			fmt.Fprintf(&builder, "   Handler: %s\n", ep.Handler)
		}
		fmt.Fprintf(&builder, "   Location: %s:%d\n", ep.File, ep.Line)
		builder.WriteString("\n")
	}

	return builder.String()
}

func (g *APIDocGenerator) getOutputFilename(format string) string {
	switch format {
	case "openapi":
		return filepath.Join(g.workDir, "api-spec.yaml")
	case "postman":
		return filepath.Join(g.workDir, "api-collection.json")
	default:
		return filepath.Join(g.workDir, "API.md")
	}
}

func (g *APIDocGenerator) extractDoc(text, format string) string {
	text = strings.TrimSpace(text)

	markers := map[string][]string{
		"openapi":  {"```yaml", "```yml", "```"},
		"postman":  {"```json", "```"},
		"markdown": {"```markdown", "```md", "```"},
	}

	if prefixes, ok := markers[format]; ok {
		for _, marker := range prefixes {
			if strings.HasPrefix(text, marker) {
				text = strings.TrimPrefix(text, marker)
				text = strings.TrimPrefix(text, "\n")
				text = strings.TrimSuffix(text, "```")
				break
			}
		}
	}

	return strings.TrimSpace(text)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
