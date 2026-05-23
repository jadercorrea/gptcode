package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Generator struct {
	cwd      string
	commands []Command
}

type Command struct {
	Name     string
	Short    string
	Long     string
	Flags    []Flag
	Examples []Example
}

type Flag struct {
	Name        string
	Short       string
	Default     string
	Description string
}

type Example struct {
	Command string
	Output  string
}

func NewGenerator(cwd string) *Generator {
	return &Generator{cwd: cwd}
}

func (g *Generator) AddCommand(cmd Command) {
	g.commands = append(g.commands, cmd)
}

func (g *Generator) GenerateREADME() string {
	var sb strings.Builder

	sb.WriteString("# GT CLI\n\n")
	sb.WriteString("Autonomous coding assistant powered by AI.\n\n")

	sb.WriteString("## Installation\n\n")
	sb.WriteString("```bash\ngo install github.com/gptcode-cloud/cli@latest\n```\n\n")

	sb.WriteString("## Quick Start\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Run a task\n")
	sb.WriteString("gt run \"fix bug in auth\"\n\n")
	sb.WriteString("# Autonomous mode\n")
	sb.WriteString("gt do \"implement login\"\n\n")
	sb.WriteString("# Create PR\n")
	sb.WriteString("gt pr create\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Commands\n\n")

	for _, cmd := range g.commands {
		fmt.Fprintf(&sb, "### %s\n\n", cmd.Name)
		sb.WriteString(cmd.Short + "\n\n")

		if len(cmd.Flags) > 0 {
			sb.WriteString("**Flags:**\n\n")
			for _, flag := range cmd.Flags {
				fmt.Fprintf(&sb, "- `--%s`", flag.Name)
				if flag.Short != "" {
					fmt.Fprintf(&sb, " (`-%s`)", flag.Short)
				}
				fmt.Fprintf(&sb, ": %s", flag.Description)
				if flag.Default != "" {
					fmt.Fprintf(&sb, " (default: `%s`)", flag.Default)
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		if len(cmd.Examples) > 0 {
			sb.WriteString("**Examples:**\n\n")
			for _, ex := range cmd.Examples {
				fmt.Fprintf(&sb, "```bash\n%s\n```\n", ex.Command)
				if ex.Output != "" {
					fmt.Fprintf(&sb, "```\n%s\n```\n", ex.Output)
				}
			}
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

func (g *Generator) GenerateCHANGELOG() string {
	var sb strings.Builder

	sb.WriteString("# Changelog\n\n")
	sb.WriteString("All notable changes will be documented in this file.\n\n")

	sb.WriteString("## [Unreleased]\n\n")
	sb.WriteString("### Added\n")
	sb.WriteString("- Initial release\n")

	return sb.String()
}

func (g *Generator) GenerateAPIDocs() string {
	var sb strings.Builder

	sb.WriteString("# API Documentation\n\n")

	sb.WriteString("## Overview\n\n")
	sb.WriteString("GT CLI provides a programmatic API for integration.\n\n")

	sb.WriteString("## Environment Variables\n\n")
	sb.WriteString("| Variable | Description | Default |\n")
	sb.WriteString("|----------|-------------|----------|\n")
	sb.WriteString("| `OPENROUTER_API_KEY` | OpenRouter API key | - |\n")
	sb.WriteString("| `GTCODE_LIVE_URL` | Live Dashboard URL | - |\n")
	sb.WriteString("| `GTCODE_CONTEXT` | Project context | - |\n")
	sb.WriteString("\n")

	sb.WriteString("## Live Dashboard API\n\n")
	sb.WriteString("### Connect\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("curl -X POST $GTCODE_LIVE_URL/api/report/connect \\\n")
	sb.WriteString("  -H 'Content-Type: application/json' \\\n")
	sb.WriteString("  -d '{\"agent_id\": \"...\", \"task\": \"...\"}'\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### Report Step\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("curl -X POST $GTCODE_LIVE_URL/api/report/step \\\n")
	sb.WriteString("  -H 'Content-Type: application/json' \\\n")
	sb.WriteString("  -d '{\"agent_id\": \"...\", \"description\": \"...\"}'\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### Disconnect\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("curl -X POST $GTCODE_LIVE_URL/api/report/disconnect \\\n")
	sb.WriteString("  -H 'Content-Type: application/json' \\\n")
	sb.WriteString("  -d '{\"agent_id\": \"...\"}'\n")
	sb.WriteString("```\n\n")

	return sb.String()
}

func (g *Generator) GenerateContributing() string {
	var sb strings.Builder

	sb.WriteString("# Contributing\n\n")

	sb.WriteString("## Development Setup\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("git clone https://github.com/gptcode-cloud/cli.git\n")
	sb.WriteString("cd cli\n")
	sb.WriteString("go build ./...\n")
	sb.WriteString("go test ./...\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Testing\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("go test ./...\n")
	sb.WriteString("go test -v ./internal/testing/...\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Code Style\n\n")
	sb.WriteString("- Run `gofmt` before committing\n")
	sb.WriteString("- Follow Go idioms\n")
	sb.WriteString("- Add tests for new features\n")

	return sb.String()
}

func (g *Generator) WriteREADME() error {
	content := g.GenerateREADME()
	return os.WriteFile(filepath.Join(g.cwd, "README.md"), []byte(content), 0644)
}

func (g *Generator) WriteCHANGELOG() error {
	content := g.GenerateCHANGELOG()
	return os.WriteFile(filepath.Join(g.cwd, "CHANGELOG.md"), []byte(content), 0644)
}

func (g *Generator) WriteAPIDocs() error {
	content := g.GenerateAPIDocs()
	return os.WriteFile(filepath.Join(g.cwd, "API.md"), []byte(content), 0644)
}

func (g *Generator) WriteContributing() error {
	content := g.GenerateContributing()
	return os.WriteFile(filepath.Join(g.cwd, "CONTRIBUTING.md"), []byte(content), 0644)
}

func (g *Generator) WriteAll() error {
	docs := []struct {
		filename string
		generate func() string
	}{
		{"README.md", g.GenerateREADME},
		{"CHANGELOG.md", g.GenerateCHANGELOG},
		{"API.md", g.GenerateAPIDocs},
		{"CONTRIBUTING.md", g.GenerateContributing},
	}

	for _, doc := range docs {
		if err := os.WriteFile(filepath.Join(g.cwd, doc.filename), []byte(doc.generate()), 0644); err != nil {
			return err
		}
	}

	return nil
}

type MarkdownFormatter struct{}

func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

func (m *MarkdownFormatter) Heading(text string, level int) string {
	prefix := strings.Repeat("#", level)
	return fmt.Sprintf("%s %s\n\n", prefix, text)
}

func (m *MarkdownFormatter) CodeBlock(lang, code string) string {
	return fmt.Sprintf("```%s\n%s\n```\n\n", lang, code)
}

func (m *MarkdownFormatter) InlineCode(code string) string {
	return "`" + code + "`"
}

func (m *MarkdownFormatter) Link(text, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}

func (m *MarkdownFormatter) List(items []string) string {
	var sb strings.Builder
	for _, item := range items {
		fmt.Fprintf(&sb, "- %s\n", item)
	}
	sb.WriteString("\n")
	return sb.String()
}

func (m *MarkdownFormatter) Table(headers []string, rows [][]string) string {
	var sb strings.Builder

	// Headers
	sb.WriteString("| ")
	sb.WriteString(strings.Join(headers, " | "))
	sb.WriteString(" |\n")

	// Separator
	sb.WriteString("|")
	for range headers {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range rows {
		sb.WriteString("| ")
		sb.WriteString(strings.Join(row, " | "))
		sb.WriteString(" |\n")
	}

	return sb.String()
}

func (m *MarkdownFormatter) BlockQuote(text string) string {
	return "> " + strings.ReplaceAll(text, "\n", "\n> ") + "\n\n"
}

func (m *MarkdownFormatter) Bold(text string) string {
	return "**" + text + "**"
}

func (m *MarkdownFormatter) Italic(text string) string {
	return "_" + text + "_"
}
