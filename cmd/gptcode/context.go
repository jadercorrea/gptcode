package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jadercorrea/gptcode/internal/live"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage universal AI context layer",
	Long: `Universal context management for AI assistants (Warp, Cursor, Claude, etc).

The context layer stores shared knowledge about your project in .gptcode/ directory:
  - shared.md: Technical context (architecture, stack, patterns)
  - next.md: Immediate next tasks
  - roadmap.md: Long-term roadmap

This context can be automatically injected into any AI assistant session.`,
}

var contextInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .gptcode context directory",
	RunE:  runContextInit,
}

var contextAddCmd = &cobra.Command{
	Use:   "add <type> <content>",
	Short: "Add content to context (types: shared, next, roadmap)",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runContextAdd,
}

var contextShowCmd = &cobra.Command{
	Use:   "show [type]",
	Short: "Show context content (types: shared, next, roadmap, all)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runContextShow,
}

var contextSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync context to integration formats (WARP.md, .cursor/docs, etc)",
	RunE:  runContextSync,
}

var contextExportCmd = &cobra.Command{
	Use:   "export <format>",
	Short: "Export context to specific format (warp, cursor, clipboard)",
	Args:  cobra.ExactArgs(1),
	RunE:  runContextExport,
}

var contextLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Sync context with Live Dashboard (real-time)",
	Long: `Connect to GPTCode Live Dashboard and sync project context in real-time.

This allows viewing and editing your project context from the web dashboard,
mobile, or any device with the Live dashboard open.

The context will be synced bidirectionally - changes made in Live will be
written back to your local .gptcode/context/ files.`,
	RunE: runContextLive,
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(contextInitCmd)
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextShowCmd)
	contextCmd.AddCommand(contextSyncCmd)
	contextCmd.AddCommand(contextExportCmd)
	contextCmd.AddCommand(contextLiveCmd)
}

type ContextConfig struct {
	Context struct {
		Shared  string `yaml:"shared"`
		Next    string `yaml:"next"`
		Roadmap string `yaml:"roadmap"`
	} `yaml:"context"`
	AutoLoad     []string `yaml:"auto_load"`
	Integrations struct {
		Warp struct {
			Enabled  bool   `yaml:"enabled"`
			RulePath string `yaml:"rule_path"`
		} `yaml:"warp"`
		Cursor struct {
			Enabled bool   `yaml:"enabled"`
			DocPath string `yaml:"doc_path"`
		} `yaml:"cursor"`
	} `yaml:"integrations"`
}

func getGPTCodeDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Search for .gptcode in current dir and parents
	dir := cwd
	for {
		gptcodePath := filepath.Join(dir, ".gptcode")
		if _, err := os.Stat(gptcodePath); err == nil {
			return gptcodePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".gptcode directory not found (run 'gptcode context init')")
}

func runContextInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	gptcodeDir := filepath.Join(cwd, ".gptcode")
	contextDir := filepath.Join(gptcodeDir, "context")

	if _, err := os.Stat(gptcodeDir); err == nil {
		return fmt.Errorf(".gptcode directory already exists")
	}

	fmt.Println("🚀 Initializing GPTCode context layer...")

	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	sharedContent := `# Project Context

## Architecture
<!-- Describe your system architecture, main components, how services communicate -->

## Stack
<!-- List technologies: languages, frameworks, databases, infrastructure -->

## Patterns
<!-- Document coding patterns, conventions, best practices -->

## Development
<!-- Setup instructions, common commands, debugging tips -->
`

	nextContent := `# Next Tasks

## In Progress
<!-- Tasks currently being worked on -->

## This Week
<!-- Tasks planned for this week -->

## Backlog
<!-- Upcoming tasks in priority order -->
`

	roadmapContent := `# Roadmap

## Current Quarter
<!-- Major initiatives for this quarter -->

## Next Quarter
<!-- Planned work for next quarter -->

## Future
<!-- Long-term vision and goals -->
`

	files := map[string]string{
		"shared.md":  sharedContent,
		"next.md":    nextContent,
		"roadmap.md": roadmapContent,
	}

	for filename, content := range files {
		path := filepath.Join(contextDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	config := ContextConfig{}
	config.Context.Shared = "context/shared.md"
	config.Context.Next = "context/next.md"
	config.Context.Roadmap = "context/roadmap.md"
	config.AutoLoad = []string{"shared", "next"}
	config.Integrations.Warp.Enabled = true
	config.Integrations.Warp.RulePath = "WARP.md"
	config.Integrations.Cursor.Enabled = false
	config.Integrations.Cursor.DocPath = ".cursor/docs"

	configPath := filepath.Join(gptcodeDir, "config.yml")
	configData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	gitignorePath := filepath.Join(gptcodeDir, ".gitignore")
	gitignoreContent := "# Add files to ignore within .gptcode/\n# sessions/\n"
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	fmt.Println("✅ Context layer initialized!")
	fmt.Println("")
	fmt.Println("📁 Structure created:")
	fmt.Println("  .gptcode/")
	fmt.Println("    context/")
	fmt.Println("      shared.md   - Technical context")
	fmt.Println("      next.md     - Next tasks")
	fmt.Println("      roadmap.md  - Roadmap")
	fmt.Println("    config.yml    - Configuration")
	fmt.Println("")
	fmt.Println("📝 Next steps:")
	fmt.Println("  1. Edit context files: vi .gptcode/context/shared.md")
	fmt.Println("  2. Show context: gptcode context show")
	fmt.Println("  3. Export for use: gptcode context export clipboard")
	fmt.Println("")
	fmt.Println("💡 Tip: Context is version-controlled. Commit .gptcode/ to share with team.")

	return nil
}

func runContextAdd(cmd *cobra.Command, args []string) error {
	contextType := args[0]
	content := args[1]

	validTypes := map[string]string{
		"shared":  "shared.md",
		"next":    "next.md",
		"roadmap": "roadmap.md",
	}

	filename, ok := validTypes[contextType]
	if !ok {
		return fmt.Errorf("invalid context type. Use: shared, next, roadmap")
	}

	gptcodeDir, err := getGPTCodeDir()
	if err != nil {
		return err
	}

	contextPath := filepath.Join(gptcodeDir, "context", filename)

	currentContent, err := os.ReadFile(contextPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	newContent := string(currentContent) + "\n" + content + "\n"
	if err := os.WriteFile(contextPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}

	fmt.Printf("✅ Added to %s\n", contextType)
	return nil
}

func runContextShow(cmd *cobra.Command, args []string) error {
	gptcodeDir, err := getGPTCodeDir()
	if err != nil {
		return err
	}

	contextType := "all"
	if len(args) > 0 {
		contextType = args[0]
	}

	files := map[string]string{
		"shared":  "shared.md",
		"next":    "next.md",
		"roadmap": "roadmap.md",
	}

	if contextType != "all" {
		filename, ok := files[contextType]
		if !ok {
			return fmt.Errorf("invalid context type. Use: shared, next, roadmap, all")
		}
		return showContextFile(gptcodeDir, contextType, filename)
	}

	for name, filename := range files {
		if err := showContextFile(gptcodeDir, name, filename); err != nil {
			fmt.Printf("⚠️  Failed to read %s: %v\n", name, err)
		}
		fmt.Println("")
	}

	return nil
}

func showContextFile(gptcodeDir, name, filename string) error {
	contextPath := filepath.Join(gptcodeDir, "context", filename)
	content, err := os.ReadFile(contextPath)
	if err != nil {
		return err
	}

	fmt.Printf("=== %s ===\n", name)
	fmt.Println(string(content))
	return nil
}

func runContextSync(cmd *cobra.Command, args []string) error {
	gptcodeDir, err := getGPTCodeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(gptcodeDir, "config.yml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config ContextConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Println("🔄 Syncing context to integrations...")

	projectRoot := filepath.Dir(gptcodeDir)
	synced := 0

	if config.Integrations.Warp.Enabled {
		if err := syncToWarp(gptcodeDir, projectRoot, config); err != nil {
			fmt.Printf("⚠️  Warp sync failed: %v\n", err)
		} else {
			fmt.Println("✅ Synced to WARP.md")
			synced++
		}
	}

	if config.Integrations.Cursor.Enabled {
		if err := syncToCursor(gptcodeDir, projectRoot, config); err != nil {
			fmt.Printf("⚠️  Cursor sync failed: %v\n", err)
		} else {
			fmt.Println("✅ Synced to .cursor/docs/")
			synced++
		}
	}

	if synced == 0 {
		fmt.Println("ℹ️  No integrations enabled. Edit .gptcode/config.yml to enable.")
	} else {
		fmt.Printf("\n✅ Synced to %d integration(s)\n", synced)
	}

	return nil
}

func syncToWarp(gptcodeDir, projectRoot string, config ContextConfig) error {
	content, err := buildContextContent(gptcodeDir, []string{"shared", "next"})
	if err != nil {
		return err
	}

	warpPath := filepath.Join(projectRoot, config.Integrations.Warp.RulePath)
	return os.WriteFile(warpPath, []byte(content), 0644)
}

func syncToCursor(gptcodeDir, projectRoot string, config ContextConfig) error {
	cursorDocsDir := filepath.Join(projectRoot, config.Integrations.Cursor.DocPath)
	if err := os.MkdirAll(cursorDocsDir, 0755); err != nil {
		return err
	}

	files := map[string]string{
		"shared":  "shared.md",
		"next":    "next.md",
		"roadmap": "roadmap.md",
	}

	for _, filename := range files {
		srcPath := filepath.Join(gptcodeDir, "context", filename)
		dstPath := filepath.Join(cursorDocsDir, filename)

		content, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}

		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			return err
		}
	}

	return nil
}

func runContextExport(cmd *cobra.Command, args []string) error {
	format := args[0]

	gptcodeDir, err := getGPTCodeDir()
	if err != nil {
		return err
	}

	switch format {
	case "warp":
		return exportToWarp(gptcodeDir)
	case "cursor":
		return exportToCursor(gptcodeDir)
	case "clipboard":
		return exportToClipboard(gptcodeDir)
	default:
		return fmt.Errorf("invalid format. Use: warp, cursor, clipboard")
	}
}

func exportToWarp(gptcodeDir string) error {
	content, err := buildContextContent(gptcodeDir, []string{"shared", "next"})
	if err != nil {
		return err
	}

	fmt.Println(content)
	return nil
}

func exportToCursor(gptcodeDir string) error {
	content, err := buildContextContent(gptcodeDir, []string{"shared", "next", "roadmap"})
	if err != nil {
		return err
	}

	fmt.Println(content)
	return nil
}

func exportToClipboard(gptcodeDir string) error {
	content, err := buildContextContent(gptcodeDir, []string{"shared", "next"})
	if err != nil {
		return err
	}

	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard (pbcopy not available): %w", err)
	}

	fmt.Println("✅ Context copied to clipboard")
	fmt.Printf("📋 %d characters ready to paste\n", len(content))
	return nil
}

func buildContextContent(gptcodeDir string, types []string) (string, error) {
	var content strings.Builder

	files := map[string]string{
		"shared":  "shared.md",
		"next":    "next.md",
		"roadmap": "roadmap.md",
	}

	for _, contextType := range types {
		filename, ok := files[contextType]
		if !ok {
			continue
		}

		contextPath := filepath.Join(gptcodeDir, "context", filename)
		data, err := os.ReadFile(contextPath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", filename, err)
		}

		content.WriteString(string(data))
		content.WriteString("\n\n")
	}

	return content.String(), nil
}

func runContextLive(cmd *cobra.Command, args []string) error {
	gptcodeDir, err := getGPTCodeDir()
	if err != nil {
		return err
	}

	dashboardURL := live.GetDashboardURL()
	agentID := live.GetAgentID()

	fmt.Printf("🔄 Connecting to Live Dashboard at %s...\n", dashboardURL)
	fmt.Printf("   Agent ID: %s\n", agentID)

	// Create and connect live client
	client, err := live.AutoSync(dashboardURL, agentID)
	if err != nil {
		return fmt.Errorf("failed to connect to Live Dashboard: %w", err)
	}

	// Set up callback for when dashboard edits context
	client.OnContextEdit(func(contextType, content string) {
		if err := writeContextFileWithBackup(gptcodeDir, contextType, content); err != nil {
			fmt.Printf("⚠️  Failed to update local context: %v\n", err)
			return
		}
		fmt.Printf("✅ Updated local %s context from Live Dashboard\n", contextType)
	})

	fmt.Println("\n✅ Connected! Context sync active:")
	fmt.Println("   - Local changes → Live Dashboard")
	fmt.Println("   - Dashboard edits → Local files")
	fmt.Println("\n📡 Watching for changes... (Ctrl+C to stop)")

	// Block until interrupted
	select {}
}

// writeContextFileWithBackup writes context to file with backup
func writeContextFileWithBackup(gptcodeDir, contextType, content string) error {
	filename := contextType + ".md"
	path := filepath.Join(gptcodeDir, "context", filename)

	// Create backup
	backupPath := path + ".backup"
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write new content
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		// Restore backup on failure
		if _, backupErr := os.Stat(backupPath); backupErr == nil {
			os.Rename(backupPath, path) // Best effort restore
		}
		return fmt.Errorf("failed to write context: %w", err)
	}

	// Remove backup on success
	if _, err := os.Stat(backupPath); err == nil {
		os.Remove(backupPath)
	}

	return nil
}
