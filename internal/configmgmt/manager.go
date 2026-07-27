package configmgmt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type ConfigFile struct {
	Path        string
	Format      string
	Environment string
	Content     string
}

type ConfigChange struct {
	File        string
	Environment string
	Key         string
	OldValue    string
	NewValue    string
	Reason      string
}

type ConfigReport struct {
	Detected     []ConfigFile
	Changes      []ConfigChange
	UpdatedFiles []string
	Errors       []error
}

type Manager struct {
	provider llm.Provider
	model    string
	workDir  string
}

func NewManager(provider llm.Provider, model, workDir string) *Manager {
	return &Manager{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (m *Manager) DetectAndUpdate(ctx context.Context, env, key, value string, autoApply bool) (*ConfigReport, error) {
	lang := langdetect.DetectLanguage(m.workDir)

	configs, err := m.findConfigFiles(lang)
	if err != nil {
		return nil, fmt.Errorf("failed to find config files: %w", err)
	}

	report := &ConfigReport{
		Detected: configs,
	}

	if key == "" || value == "" {
		return report, nil
	}

	for _, cfg := range configs {
		if env != "" && cfg.Environment != env && cfg.Environment != "all" {
			continue
		}

		change, err := m.updateConfig(ctx, cfg, key, value, autoApply)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", cfg.Path, err))
		} else if change != nil {
			report.Changes = append(report.Changes, *change)
			if !contains(report.UpdatedFiles, cfg.Path) {
				report.UpdatedFiles = append(report.UpdatedFiles, cfg.Path)
			}
		}
	}

	return report, nil
}

func (m *Manager) findConfigFiles(lang langdetect.Language) ([]ConfigFile, error) {
	var configs []ConfigFile

	patterns := m.getConfigPatterns(lang)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(m.workDir, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil {
				continue
			}

			relPath, _ := filepath.Rel(m.workDir, match)

			cfg := ConfigFile{
				Path:        relPath,
				Format:      detectFormat(relPath),
				Environment: detectEnvironment(relPath),
				Content:     string(content),
			}
			configs = append(configs, cfg)
		}
	}

	return configs, nil
}

func (m *Manager) getConfigPatterns(lang langdetect.Language) []string {
	patterns := []string{
		".env",
		".env.*",
		"config.json",
		"config/*.json",
		"config.yaml",
		"config.yml",
		"config/*.yaml",
		"config/*.yml",
	}

	switch lang {
	case langdetect.Go:
		patterns = append(patterns, "config.toml", "*.toml")
	case langdetect.TypeScript:
		patterns = append(patterns, "tsconfig.json", "package.json")
	case langdetect.Python:
		patterns = append(patterns, "setup.cfg", "pyproject.toml", "*.ini")
	case langdetect.Ruby:
		patterns = append(patterns, "config/*.rb", "config/environments/*.rb")
	case langdetect.Elixir:
		patterns = append(patterns, "config/*.exs")
	}

	return patterns
}

func (m *Manager) updateConfig(ctx context.Context, cfg ConfigFile, key, value string, apply bool) (*ConfigChange, error) {
	prompt := fmt.Sprintf(`Update configuration file:

File: %s
Format: %s
Environment: %s

Key to update: %s
New value: %s

Current content:
%s

Update the configuration. Return ONLY the complete updated file content, no explanations.`,
		cfg.Path, cfg.Format, cfg.Environment, key, value, cfg.Content)

	resp, err := m.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that updates configuration files accurately.",
		UserPrompt:   prompt,
		Model:        m.model,
	})

	if err != nil {
		return nil, err
	}

	updated := m.extractCode(resp.Text)

	oldValue := m.extractValue(cfg.Content, key)

	change := &ConfigChange{
		File:        cfg.Path,
		Environment: cfg.Environment,
		Key:         key,
		OldValue:    oldValue,
		NewValue:    value,
		Reason:      "User-requested update",
	}

	if apply {
		fullPath := filepath.Join(m.workDir, cfg.Path)
		if err := os.WriteFile(fullPath, []byte(updated), 0644); err != nil {
			return nil, err
		}
	}

	return change, nil
}

func (m *Manager) extractValue(content, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
			parts = strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func (m *Manager) extractCode(text string) string {
	text = strings.TrimSpace(text)

	for _, marker := range []string{"```yaml", "```json", "```toml", "```env", "```"} {
		if strings.HasPrefix(text, marker) {
			text = strings.TrimPrefix(text, marker)
			text = strings.TrimPrefix(text, "\n")
			text = strings.TrimSuffix(text, "```")
			break
		}
	}

	return strings.TrimSpace(text)
}

func detectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".env":
		return "ENV"
	case ".ini":
		return "INI"
	case ".rb":
		return "Ruby"
	case ".exs":
		return "Elixir"
	default:
		return "Unknown"
	}
}

func detectEnvironment(path string) string {
	lower := strings.ToLower(path)

	if strings.Contains(lower, "prod") || strings.Contains(lower, "production") {
		return "production"
	}
	if strings.Contains(lower, "dev") || strings.Contains(lower, "development") {
		return "development"
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "testing") {
		return "test"
	}
	if strings.Contains(lower, "stag") {
		return "staging"
	}

	return "all"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
