package maestro

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

func requiresStablePublicAPI(task string) bool {
	lower := strings.ToLower(task)
	return strings.Contains(lower, "without changing its public api") ||
		strings.Contains(lower, "without changing the public api") ||
		strings.Contains(lower, "without changing public api") ||
		strings.Contains(lower, "sem mudar a api pública") ||
		strings.Contains(lower, "sem alterar a api pública")
}

func snapshotGoPublicAPI(root string) (map[string]string, error) {
	api := make(map[string]string)
	files := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor", "node_modules":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, declaration := range parsed.Decls {
			switch node := declaration.(type) {
			case *ast.FuncDecl:
				if !node.Name.IsExported() {
					continue
				}
				receiver := ""
				if node.Recv != nil && len(node.Recv.List) > 0 {
					receiver = receiverName(node.Recv.List[0].Type) + "."
				}
				api[filepath.ToSlash(relative)+":func:"+receiver+node.Name.Name] = renderNode(files, node.Type)
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok && typeSpec.Name.IsExported() {
						api[filepath.ToSlash(relative)+":type:"+typeSpec.Name.Name] = renderPublicType(files, typeSpec.Type)
					}
				}
			}
		}
		return nil
	})
	return api, err
}

func renderPublicType(files *token.FileSet, expression ast.Expr) string {
	structure, ok := expression.(*ast.StructType)
	if !ok {
		return renderNode(files, expression)
	}

	var fields []string
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.IsExported() {
				fields = append(fields, name.Name+" "+renderNode(files, field.Type))
			}
		}
	}
	return "struct{" + strings.Join(fields, ";") + "}"
}

func receiverName(expression ast.Expr) string {
	switch node := expression.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return receiverName(node.X)
	case *ast.IndexExpr:
		return receiverName(node.X)
	case *ast.IndexListExpr:
		return receiverName(node.X)
	default:
		return renderNode(token.NewFileSet(), expression)
	}
}

func renderNode(files *token.FileSet, node any) string {
	var output bytes.Buffer
	_ = printer.Fprint(&output, files, node)
	return output.String()
}

func publicAPIChanges(before, after map[string]string) []string {
	var changes []string
	for symbol, signature := range before {
		current, exists := after[symbol]
		switch {
		case !exists:
			changes = append(changes, "removed "+symbol)
		case current != signature:
			changes = append(changes, "changed "+symbol)
		}
	}
	for symbol := range after {
		if _, exists := before[symbol]; !exists {
			changes = append(changes, "added "+symbol)
		}
	}
	sort.Strings(changes)
	return changes
}
