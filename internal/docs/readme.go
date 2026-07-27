package docs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type ReadmeUpdater struct {
	provider llm.Provider
	model    string
	workDir  string
}

type UpdateResult struct {
	Updated bool
	Changes []string
	NewText string
	Error   error
}

func NewReadmeUpdater(provider llm.Provider, model, workDir string) *ReadmeUpdater {
	return &ReadmeUpdater{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

func (u *ReadmeUpdater) UpdateReadme(ctx context.Context) (*UpdateResult, error) {
	readmePath := filepath.Join(u.workDir, "README.md")

	currentReadme, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read README: %w", err)
	}

	changes, err := u.detectChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	if len(changes) == 0 {
		return &UpdateResult{
			Updated: false,
			Changes: []string{},
		}, nil
	}

	updatedReadme, err := u.generateUpdate(ctx, string(currentReadme), changes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate update: %w", err)
	}

	result := &UpdateResult{
		Updated: true,
		Changes: changes,
		NewText: updatedReadme,
	}

	return result, nil
}

func (u *ReadmeUpdater) detectChanges() ([]string, error) {
	var changes []string

	recentCommits, err := u.getRecentCommits()
	if err != nil {
		return nil, err
	}

	for _, commit := range recentCommits {
		if strings.HasPrefix(commit, "feat:") || strings.HasPrefix(commit, "feat(") {
			changes = append(changes, commit)
		}
	}

	newFiles, err := u.getNewFiles()
	if err != nil {
		return nil, err
	}
	if len(newFiles) > 0 {
		changes = append(changes, fmt.Sprintf("Added %d new file(s)", len(newFiles)))
	}

	newCommands, err := u.detectNewCommands()
	if err == nil && len(newCommands) > 0 {
		for _, cmd := range newCommands {
			changes = append(changes, fmt.Sprintf("New command: %s", cmd))
		}
	}

	return changes, nil
}

func (u *ReadmeUpdater) getRecentCommits() ([]string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-10", "--no-merges")
	cmd.Dir = u.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []string
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, parts[1])
		}
	}

	return commits, nil
}

func (u *ReadmeUpdater) getNewFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-status", "HEAD~5..HEAD")
	cmd.Dir = u.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var newFiles []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "A\t") {
			file := strings.TrimPrefix(line, "A\t")
			if strings.HasSuffix(file, ".go") && strings.Contains(file, "cmd/") {
				newFiles = append(newFiles, file)
			}
		}
	}

	return newFiles, nil
}

func (u *ReadmeUpdater) detectNewCommands() ([]string, error) {
	cmdPath := filepath.Join(u.workDir, "cmd/gptcode")

	entries, err := os.ReadDir(cmdPath)
	if err != nil {
		return nil, err
	}

	var commands []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			name := strings.TrimSuffix(entry.Name(), ".go")
			if name != "main" {
				commands = append(commands, name)
			}
		}
	}

	return commands, nil
}

func (u *ReadmeUpdater) generateUpdate(ctx context.Context, currentReadme string, changes []string) (string, error) {
	changesText := strings.Join(changes, "\n- ")

	prompt := fmt.Sprintf(`Update this README.md based on recent changes.

Current README:
%s

Recent changes:
- %s

Rules:
1. Keep existing structure and sections
2. Update feature lists and capabilities
3. Add/update examples for new commands
4. Maintain professional tone
5. Keep badges and links intact
6. Update version/status if significant features added
7. DO NOT remove important content
8. Add brief descriptions for new features

Return ONLY the complete updated README.md, no explanations.`, currentReadme, changesText)

	resp, err := u.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a README expert that updates documentation based on code changes.",
		UserPrompt:   prompt,
		Model:        u.model,
	})

	if err != nil {
		return "", err
	}

	updated := strings.TrimSpace(resp.Text)

	if strings.HasPrefix(updated, "```markdown") {
		updated = strings.TrimPrefix(updated, "```markdown\n")
		updated = strings.TrimSuffix(updated, "```")
	} else if strings.HasPrefix(updated, "```") {
		updated = strings.TrimPrefix(updated, "```\n")
		updated = strings.TrimSuffix(updated, "```")
	}

	return strings.TrimSpace(updated), nil
}

func (u *ReadmeUpdater) ApplyUpdate(readmePath, newContent string) error {
	backupPath := readmePath + ".backup"

	if err := os.Rename(readmePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if err := os.WriteFile(readmePath, []byte(newContent), 0644); err != nil {
		os.Rename(backupPath, readmePath)
		return fmt.Errorf("failed to write README: %w", err)
	}

	os.Remove(backupPath)
	return nil
}
