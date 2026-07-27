package compat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type DeprecatedAPI struct {
	Name        string
	Version     string
	File        string
	Line        int
	Replacement string
	Reason      string
}

type CompatibilityReport struct {
	DeprecatedAPIs []DeprecatedAPI
	WrapperCode    string
	MigrationGuide string
	BreakingIn     string
}

type CompatManager struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewCompatManager(provider llm.Provider, model, workDir string) *CompatManager {
	return &CompatManager{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (m *CompatManager) AddDeprecation(ctx context.Context, oldAPI, newAPI, version, reason string) (*CompatibilityReport, error) {
	wrapperCode, err := m.generateWrapperCode(ctx, oldAPI, newAPI, version, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to generate wrapper: %w", err)
	}

	migrationGuide, err := m.generateMigrationGuide(ctx, oldAPI, newAPI, version, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to generate migration guide: %w", err)
	}

	report := &CompatibilityReport{
		DeprecatedAPIs: []DeprecatedAPI{
			{
				Name:        oldAPI,
				Replacement: newAPI,
				Version:     version,
				Reason:      reason,
			},
		},
		WrapperCode:    wrapperCode,
		MigrationGuide: migrationGuide,
		BreakingIn:     m.calculateBreakingVersion(version),
	}

	return report, nil
}

func (m *CompatManager) generateWrapperCode(ctx context.Context, oldAPI, newAPI, version, reason string) (string, error) {
	prompt := fmt.Sprintf(`Generate backward compatibility wrapper code:

Old API: %s
New API: %s
Deprecated in: %s
Reason: %s

Requirements:
1. Keep old API working
2. Add deprecation warnings/comments
3. Internally call new API
4. Include version info
5. Suggest migration path

Return ONLY the wrapper code with comments.`, oldAPI, newAPI, version, reason)

	resp, err := m.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that generates backward compatibility wrapper code.",
		UserPrompt:   prompt,
		Model:        m.model,
	})

	if err != nil {
		return "", err
	}

	return m.extractCode(resp.Text), nil
}

func (m *CompatManager) generateMigrationGuide(ctx context.Context, oldAPI, newAPI, version, reason string) (string, error) {
	prompt := fmt.Sprintf(`Generate migration guide:

From: %s
To: %s
Version: %s
Reason: %s

Include:
1. What changed and why
2. Step-by-step migration
3. Code examples (before/after)
4. Timeline (deprecation → removal)
5. Testing recommendations

Format as Markdown.`, oldAPI, newAPI, version, reason)

	resp, err := m.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that writes clear migration guides.",
		UserPrompt:   prompt,
		Model:        m.model,
	})

	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func (m *CompatManager) calculateBreakingVersion(currentVersion string) string {
	parts := strings.Split(currentVersion, ".")
	if len(parts) < 2 {
		return "next major version"
	}

	return fmt.Sprintf("%s.0.0 (next major)", parts[0])
}

func (m *CompatManager) SaveCompatibilityFiles(report *CompatibilityReport) error {
	compatDir := filepath.Join(m.workDir, "compat")
	if err := os.MkdirAll(compatDir, 0755); err != nil {
		return err
	}

	wrapperFile := filepath.Join(compatDir, "deprecated.go")
	if err := os.WriteFile(wrapperFile, []byte(report.WrapperCode), 0644); err != nil {
		return err
	}

	guideFile := filepath.Join(m.workDir, "MIGRATION.md")
	if err := os.WriteFile(guideFile, []byte(report.MigrationGuide), 0644); err != nil {
		return err
	}

	return nil
}

func (m *CompatManager) extractCode(text string) string {
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
