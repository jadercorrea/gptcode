package migration

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
	"time"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type MigrationGenerator struct {
	provider llm.Provider
	model    string
	workDir  string
}

type ModelChange struct {
	Type      string // "added", "modified", "removed"
	ModelName string
	Field     string
	OldType   string
	NewType   string
}

type MigrationResult struct {
	MigrationFile string
	Changes       []ModelChange
	Valid         bool
	Error         error
}

func NewMigrationGenerator(provider llm.Provider, model, workDir string) *MigrationGenerator {
	return &MigrationGenerator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (g *MigrationGenerator) GenerateMigration(ctx context.Context, name string) (*MigrationResult, error) {
	changes, err := g.detectModelChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	if len(changes) == 0 {
		return &MigrationResult{
			Changes: []ModelChange{},
		}, nil
	}

	migrationCode, err := g.generateMigrationCode(ctx, name, changes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate migration: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", timestamp, strings.ReplaceAll(name, " ", "_"))

	migrationsDir := filepath.Join(g.workDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create migrations directory: %w", err)
	}

	migrationPath := filepath.Join(migrationsDir, filename)
	if err := os.WriteFile(migrationPath, []byte(migrationCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write migration: %w", err)
	}

	result := &MigrationResult{
		MigrationFile: migrationPath,
		Changes:       changes,
		Valid:         true,
	}

	return result, nil
}

func (g *MigrationGenerator) detectModelChanges() ([]ModelChange, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = g.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var changes []ModelChange
	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, file := range files {
		if !strings.HasSuffix(file, ".go") {
			continue
		}

		if strings.Contains(file, "model") || strings.Contains(file, "entity") ||
			strings.Contains(file, "schema") {
			fileChanges, err := g.analyzeFileChanges(filepath.Join(g.workDir, file))
			if err == nil {
				changes = append(changes, fileChanges...)
			}
		}
	}

	return changes, nil
}

func (g *MigrationGenerator) analyzeFileChanges(filePath string) ([]ModelChange, error) {
	currentContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", filePath))
	cmd.Dir = g.workDir
	oldContent, err := cmd.Output()
	if err != nil {
		oldContent = []byte{}
	}

	currentModels := g.parseModels(string(currentContent))
	oldModels := g.parseModels(string(oldContent))

	var changes []ModelChange

	for modelName, currentFields := range currentModels {
		oldFields, existed := oldModels[modelName]

		if !existed {
			changes = append(changes, ModelChange{
				Type:      "added",
				ModelName: modelName,
			})
			continue
		}

		for fieldName, fieldType := range currentFields {
			oldType, had := oldFields[fieldName]
			if !had {
				changes = append(changes, ModelChange{
					Type:      "added",
					ModelName: modelName,
					Field:     fieldName,
					NewType:   fieldType,
				})
			} else if oldType != fieldType {
				changes = append(changes, ModelChange{
					Type:      "modified",
					ModelName: modelName,
					Field:     fieldName,
					OldType:   oldType,
					NewType:   fieldType,
				})
			}
		}

		for fieldName, oldType := range oldFields {
			if _, exists := currentFields[fieldName]; !exists {
				changes = append(changes, ModelChange{
					Type:      "removed",
					ModelName: modelName,
					Field:     fieldName,
					OldType:   oldType,
				})
			}
		}
	}

	for modelName := range oldModels {
		if _, exists := currentModels[modelName]; !exists {
			changes = append(changes, ModelChange{
				Type:      "removed",
				ModelName: modelName,
			})
		}
	}

	return changes, nil
}

func (g *MigrationGenerator) parseModels(content string) map[string]map[string]string {
	models := make(map[string]map[string]string)

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return models
	}

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		modelName := typeSpec.Name.Name
		fields := make(map[string]string)

		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}

			fieldName := field.Names[0].Name
			fieldType := g.exprToString(field.Type)

			if field.Tag != nil {
				tag := field.Tag.Value
				if strings.Contains(tag, "gorm:") || strings.Contains(tag, "db:") ||
					strings.Contains(tag, "json:") {
					fields[fieldName] = fieldType
				}
			} else if !ast.IsExported(fieldName) {
				continue
			} else {
				fields[fieldName] = fieldType
			}
		}

		if len(fields) > 0 {
			models[modelName] = fields
		}

		return true
	})

	return models
}

func (g *MigrationGenerator) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + g.exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + g.exprToString(e.Elt)
	case *ast.SelectorExpr:
		return g.exprToString(e.X) + "." + e.Sel.Name
	default:
		return "unknown"
	}
}

func (g *MigrationGenerator) generateMigrationCode(ctx context.Context, name string, changes []ModelChange) (string, error) {
	var changeDescriptions []string
	for _, change := range changes {
		switch change.Type {
		case "added":
			if change.Field == "" {
				changeDescriptions = append(changeDescriptions,
					fmt.Sprintf("- Added model: %s", change.ModelName))
			} else {
				changeDescriptions = append(changeDescriptions,
					fmt.Sprintf("- Added field %s.%s (%s)", change.ModelName, change.Field, change.NewType))
			}
		case "modified":
			changeDescriptions = append(changeDescriptions,
				fmt.Sprintf("- Modified %s.%s: %s -> %s", change.ModelName, change.Field, change.OldType, change.NewType))
		case "removed":
			if change.Field == "" {
				changeDescriptions = append(changeDescriptions,
					fmt.Sprintf("- Removed model: %s", change.ModelName))
			} else {
				changeDescriptions = append(changeDescriptions,
					fmt.Sprintf("- Removed field %s.%s", change.ModelName, change.Field))
			}
		}
	}

	prompt := fmt.Sprintf(`Generate a SQL migration for these model changes:

Migration name: %s

Changes:
%s

Generate SQL migration with:
1. -- Up migration (apply changes)
2. -- Down migration (rollback changes)
3. Use standard SQL (PostgreSQL compatible)
4. Handle data types appropriately
5. Include constraints where needed
6. Add comments explaining changes

Return ONLY the SQL migration code, no explanations.`, name, strings.Join(changeDescriptions, "\n"))

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a database migration expert that generates safe, reversible SQL migrations.",
		UserPrompt:   prompt,
		Model:        g.model,
	})

	if err != nil {
		return "", err
	}

	code := strings.TrimSpace(resp.Text)

	if strings.HasPrefix(code, "```sql") {
		code = strings.TrimPrefix(code, "```sql\n")
		code = strings.TrimSuffix(code, "```")
	} else if strings.HasPrefix(code, "```") {
		code = strings.TrimPrefix(code, "```\n")
		code = strings.TrimSuffix(code, "```")
	}

	return strings.TrimSpace(code), nil
}
