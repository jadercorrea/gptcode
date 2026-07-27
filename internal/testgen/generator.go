package testgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type TestGenerator struct {
	provider   llm.Provider
	model      string
	workDir    string
	queryAgent *agents.QueryAgent
	language   langdetect.Language
}

type GenerateResult struct {
	TestFile    string
	TestContent string
	SourceFile  string
	Valid       bool
	Error       error
}

func NewTestGenerator(provider llm.Provider, model, workDir string) (*TestGenerator, error) {
	lang := langdetect.DetectLanguage(workDir)
	if lang == langdetect.Unknown {
		lang = langdetect.Go
	}

	queryAgent := agents.NewQuery(provider, workDir, model)

	return &TestGenerator{
		provider:   provider,
		model:      model,
		workDir:    workDir,
		queryAgent: queryAgent,
		language:   lang,
	}, nil
}

func (tg *TestGenerator) GenerateUnitTests(ctx context.Context, sourceFile string) (*GenerateResult, error) {
	result := &GenerateResult{
		SourceFile: sourceFile,
	}

	absPath := filepath.Join(tg.workDir, sourceFile)
	content, err := os.ReadFile(absPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read source file: %w", err)
		return result, result.Error
	}

	testFile := tg.getTestFilePath(sourceFile)
	result.TestFile = testFile

	prompt := tg.buildUnitTestPrompt(sourceFile, string(content))

	response, err := tg.queryAgent.Execute(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		result.Error = fmt.Errorf("LLM failed to generate tests: %w", err)
		return result, result.Error
	}

	testCode := extractCode(response)
	testCode = tg.cleanTestCode(testCode)
	result.TestContent = testCode

	testPath := filepath.Join(tg.workDir, testFile)
	if err := os.MkdirAll(filepath.Dir(testPath), 0755); err != nil {
		result.Error = fmt.Errorf("failed to create test directory: %w", err)
		return result, result.Error
	}

	if err := os.WriteFile(testPath, []byte(testCode), 0644); err != nil {
		result.Error = fmt.Errorf("failed to write test file: %w", err)
		return result, result.Error
	}

	validator := NewValidator(tg.workDir, tg.language)
	result.Valid = validator.Validate(testFile)

	if !result.Valid {
		result.Error = fmt.Errorf("generated test does not compile")
	}

	return result, nil
}

func (tg *TestGenerator) buildUnitTestPrompt(sourceFile, content string) string {
	switch tg.language {
	case langdetect.Go:
		return fmt.Sprintf(`Generate comprehensive unit tests for this Go file.

File: %s
Content:
%s

Requirements:
1. Test ALL exported functions and methods
2. Use table-driven tests where appropriate
3. Include edge cases: nil inputs, empty values, boundaries
4. Test both success and error paths
5. Use testify/assert if it helps readability
6. Follow Go testing conventions

Generate ONLY the complete test file content, ready to save as %s.
Include package declaration and all necessary imports.`, sourceFile, content, tg.getTestFilePath(sourceFile))

	case langdetect.TypeScript:
		return fmt.Sprintf(`Generate comprehensive unit tests for this TypeScript/JavaScript file.

File: %s
Content:
%s

Requirements:
1. Test all exported functions and classes
2. Use Jest or your testing framework
3. Include edge cases and error scenarios
4. Mock external dependencies
5. Follow TypeScript/Jest best practices

Generate ONLY the complete test file content.`, sourceFile, content)

	case langdetect.Python:
		return fmt.Sprintf(`Generate comprehensive unit tests for this Python file.

File: %s
Content:
%s

Requirements:
1. Use pytest
2. Test all public functions and methods
3. Include edge cases and error scenarios  
4. Use fixtures where appropriate
5. Follow pytest conventions

Generate ONLY the complete test file content.`, sourceFile, content)

	default:
		return fmt.Sprintf(`Generate unit tests for: %s\n\n%s`, sourceFile, content)
	}
}

func (tg *TestGenerator) getTestFilePath(sourceFile string) string {
	ext := filepath.Ext(sourceFile)
	base := strings.TrimSuffix(sourceFile, ext)

	switch tg.language {
	case langdetect.Go:
		return base + "_test.go"
	case langdetect.TypeScript:
		if filepath.Ext(sourceFile) == ".js" || filepath.Ext(sourceFile) == ".jsx" {
			return base + ".test.js"
		}
		return base + ".test.ts"
	case langdetect.Python:
		dir := filepath.Dir(sourceFile)
		name := filepath.Base(sourceFile)
		return filepath.Join(dir, "test_"+name)
	case langdetect.Elixir:
		return strings.Replace(sourceFile, "/lib/", "/test/", 1)
	case langdetect.Ruby:
		return strings.Replace(sourceFile, "/lib/", "/spec/", 1) + "_spec.rb"
	default:
		return base + "_test" + ext
	}
}

func (tg *TestGenerator) cleanTestCode(code string) string {
	lines := strings.Split(code, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```" || strings.HasPrefix(trimmed, "```") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}
