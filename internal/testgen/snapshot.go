package testgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type SnapshotGenerator struct {
	provider llm.Provider
	model    string
}

func NewSnapshotGenerator(provider llm.Provider, model string) *SnapshotGenerator {
	return &SnapshotGenerator{
		provider: provider,
		model:    model,
	}
}

func (g *SnapshotGenerator) Generate(ctx context.Context, sourceFile string) (string, error) {
	lang := langdetect.DetectFromFilename(sourceFile)

	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	testContent, err := g.generateSnapshotTests(ctx, string(content), lang, sourceFile)
	if err != nil {
		return "", err
	}

	testFile := g.getTestFilename(sourceFile, lang)

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write test file: %w", err)
	}

	if err := g.initializeSnapshotInfra(filepath.Dir(testFile), lang); err != nil {
		return "", fmt.Errorf("failed to initialize snapshot infrastructure: %w", err)
	}

	return testFile, nil
}

func (g *SnapshotGenerator) generateSnapshotTests(ctx context.Context, sourceCode string, lang langdetect.Language, filename string) (string, error) {
	instructions := g.getLanguageInstructions(lang)

	prompt := fmt.Sprintf(`Generate snapshot tests for this code:

Language: %s
File: %s

%s

Source code:
%s

Generate comprehensive snapshot tests following best practices.
Return ONLY the test file content, no explanations.`,
		lang, filename, instructions, sourceCode)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a snapshot testing expert that generates regression tests.",
		UserPrompt:   prompt,
		Model:        g.model,
	})

	if err != nil {
		return "", err
	}

	return g.extractCode(resp.Text), nil
}

func (g *SnapshotGenerator) getLanguageInstructions(lang langdetect.Language) string {
	switch lang {
	case langdetect.Go:
		return `Use github.com/bradleyjkemp/cupaloy for Go snapshot testing.
Tests should:
- Import "github.com/bradleyjkemp/cupaloy/v2"
- Use cupaloy.SnapshotT(t, actualOutput) for assertions
- Include table-driven tests for multiple scenarios
- Snapshots stored in __snapshots__/ directory`

	case langdetect.TypeScript:
		return `Use Jest snapshot testing.
Tests should:
- Import from '@testing-library/react' if React components
- Use expect(result).toMatchSnapshot()
- Use expect(result).toMatchInlineSnapshot() for inline snapshots
- Include describe() blocks for organization
- Snapshots stored in __snapshots__/ directory`

	case langdetect.Python:
		return `Use syrupy or pytest-snapshot for Python snapshot testing.
Tests should:
- Import snapshot fixture from syrupy
- Use assert result == snapshot for assertions
- Include parameterized tests for multiple scenarios
- Snapshots stored in __snapshots__/ directory`

	case langdetect.Ruby:
		return `Use rspec-snapshot or minitest-snapshots for Ruby.
Tests should:
- Use expect(result).to match_snapshot
- Include context blocks for organization
- Snapshots stored in spec/snapshots/ directory`

	default:
		return "Use standard snapshot testing practices for the language."
	}
}

func (g *SnapshotGenerator) getTestFilename(sourceFile string, lang langdetect.Language) string {
	dir := filepath.Dir(sourceFile)
	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	switch lang {
	case langdetect.Go:
		return filepath.Join(dir, nameWithoutExt+"_test.go")
	case langdetect.TypeScript:
		if strings.HasSuffix(sourceFile, ".tsx") {
			return filepath.Join(dir, nameWithoutExt+".test.tsx")
		}
		return filepath.Join(dir, nameWithoutExt+".test.ts")
	case langdetect.Python:
		return filepath.Join(dir, "test_"+nameWithoutExt+".py")
	case langdetect.Ruby:
		return filepath.Join(dir, nameWithoutExt+"_spec.rb")
	default:
		return filepath.Join(dir, nameWithoutExt+"_test"+ext)
	}
}

func (g *SnapshotGenerator) initializeSnapshotInfra(testDir string, lang langdetect.Language) error {
	switch lang {
	case langdetect.Go:
		return g.initGoSnapshots(testDir)
	case langdetect.TypeScript:
		return g.initJestSnapshots(testDir)
	case langdetect.Python:
		return g.initPythonSnapshots(testDir)
	case langdetect.Ruby:
		return g.initRubySnapshots(testDir)
	}
	return nil
}

func (g *SnapshotGenerator) initGoSnapshots(testDir string) error {
	snapshotDir := filepath.Join(testDir, "__snapshots__")
	return os.MkdirAll(snapshotDir, 0755)
}

func (g *SnapshotGenerator) initJestSnapshots(testDir string) error {
	snapshotDir := filepath.Join(testDir, "__snapshots__")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return err
	}

	jestConfig := filepath.Join(testDir, "..", "jest.config.js")
	if _, err := os.Stat(jestConfig); os.IsNotExist(err) {
		config := `module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  snapshotResolver: undefined,
};
`
		return os.WriteFile(jestConfig, []byte(config), 0644)
	}
	return nil
}

func (g *SnapshotGenerator) initPythonSnapshots(testDir string) error {
	snapshotDir := filepath.Join(testDir, "__snapshots__")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return err
	}

	pytestIni := filepath.Join(testDir, "..", "pytest.ini")
	if _, err := os.Stat(pytestIni); os.IsNotExist(err) {
		config := `[pytest]
testpaths = .
python_files = test_*.py
python_classes = Test*
python_functions = test_*
`
		return os.WriteFile(pytestIni, []byte(config), 0644)
	}
	return nil
}

func (g *SnapshotGenerator) initRubySnapshots(testDir string) error {
	snapshotDir := filepath.Join(testDir, "snapshots")
	return os.MkdirAll(snapshotDir, 0755)
}

func (g *SnapshotGenerator) extractCode(text string) string {
	text = strings.TrimSpace(text)

	markers := []string{"```go", "```typescript", "```python", "```ruby", "```ts", "```"}
	for _, marker := range markers {
		if strings.HasPrefix(text, marker) {
			text = strings.TrimPrefix(text, marker)
			text = strings.TrimPrefix(text, "\n")
			text = strings.TrimSuffix(text, "```")
			break
		}
	}

	return strings.TrimSpace(text)
}
