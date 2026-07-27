package merge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type ConflictFile struct {
	Path     string
	Content  string
	Resolved string
}

type Resolver struct {
	provider llm.Provider
	model    string
}

func NewResolver(provider llm.Provider, model string) *Resolver {
	return &Resolver{
		provider: provider,
		model:    model,
	}
}

func (r *Resolver) DetectConflicts() ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	var conflicts []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UU ") || strings.HasPrefix(line, "AA ") || strings.HasPrefix(line, "DD ") {
			file := strings.TrimSpace(line[3:])
			conflicts = append(conflicts, file)
		}
	}

	return conflicts, nil
}

func (r *Resolver) ResolveAll(ctx context.Context) ([]ConflictFile, error) {
	conflicts, err := r.DetectConflicts()
	if err != nil {
		return nil, err
	}

	if len(conflicts) == 0 {
		return nil, nil
	}

	var resolved []ConflictFile
	for _, file := range conflicts {
		cf, err := r.ResolveFile(ctx, file)
		if err != nil {
			return resolved, fmt.Errorf("failed to resolve %s: %w", file, err)
		}
		resolved = append(resolved, cf)
	}

	return resolved, nil
}

func (r *Resolver) ResolveFile(ctx context.Context, path string) (ConflictFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ConflictFile{}, fmt.Errorf("failed to read file: %w", err)
	}

	if !strings.Contains(string(content), "<<<<<<<") {
		return ConflictFile{
			Path:     path,
			Content:  string(content),
			Resolved: string(content),
		}, nil
	}

	branches, err := r.getConflictBranches()
	if err != nil {
		branches = "unknown branches"
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	prompt := r.buildPrompt(path, string(content), branches)
	resp, err := r.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a helpful assistant that resolves Git merge conflicts intelligently.",
		UserPrompt:   prompt,
		Model:        r.model,
	})

	if err != nil {
		return ConflictFile{}, fmt.Errorf("LLM resolution failed: %w", err)
	}

	resolvedContent := r.cleanResponse(resp.Text)

	return ConflictFile{
		Path:     path,
		Content:  string(content),
		Resolved: resolvedContent,
	}, nil
}

func (r *Resolver) ApplyResolution(cf ConflictFile) error {
	if err := os.WriteFile(cf.Path, []byte(cf.Resolved), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	addCmd := exec.Command("git", "add", cf.Path)
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage file: %w", err)
	}

	return nil
}

func (r *Resolver) getConflictBranches() (string, error) {
	cmd := exec.Command("git", "status")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	status := string(output)
	if strings.Contains(status, "both modified") || strings.Contains(status, "both added") {
		if strings.Contains(status, "rebase") {
			return "rebase in progress", nil
		}
		if strings.Contains(status, "merge") {
			return "merge in progress", nil
		}
	}

	return "unknown conflict type", nil
}

func (r *Resolver) buildPrompt(path, content, context string) string {
	return fmt.Sprintf(`Resolve this Git merge conflict intelligently.

File: %s
Context: %s

Content with conflict markers:
%s

Your task:
1. Analyze both versions (HEAD and incoming)
2. Determine which changes should be kept
3. Merge them semantically if both are needed
4. Remove ALL conflict markers (<<<<<<<, =======, >>>>>>>)

Return ONLY the resolved file content, nothing else.
Do NOT include markdown code blocks or explanations.`, path, context, content)
}

func (r *Resolver) cleanResponse(text string) string {
	cleaned := strings.TrimSpace(text)

	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```go\n")
		cleaned = strings.TrimPrefix(cleaned, "```typescript\n")
		cleaned = strings.TrimPrefix(cleaned, "```python\n")
		cleaned = strings.TrimPrefix(cleaned, "```javascript\n")
		cleaned = strings.TrimPrefix(cleaned, "```\n")
		cleaned = strings.TrimSuffix(cleaned, "\n```")
		cleaned = strings.TrimSuffix(cleaned, "```")
	}

	return strings.TrimSpace(cleaned)
}

func (r *Resolver) ValidateResolution(cf ConflictFile) error {
	if strings.Contains(cf.Resolved, "<<<<<<<") ||
		strings.Contains(cf.Resolved, "=======") ||
		strings.Contains(cf.Resolved, ">>>>>>>") {
		return fmt.Errorf("conflict markers still present in resolution")
	}

	if cf.Resolved == "" {
		return fmt.Errorf("resolved content is empty")
	}

	return nil
}
