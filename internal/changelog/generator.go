package changelog

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type ChangelogGenerator struct {
	provider llm.Provider
	model    string
	workDir  string
}

type CommitGroup struct {
	Type    string
	Commits []Commit
}

type Commit struct {
	Hash     string
	Type     string
	Scope    string
	Message  string
	Body     string
	Breaking bool
}

func NewChangelogGenerator(provider llm.Provider, model, workDir string) *ChangelogGenerator {
	return &ChangelogGenerator{
		provider: provider,
		model:    model,
		workDir:  workDir,
	}
}

var commitPattern = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.+)$`)

func (g *ChangelogGenerator) Generate(ctx context.Context, fromTag, toTag string) (string, error) {
	commits, err := g.getCommits(fromTag, toTag)
	if err != nil {
		return "", fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found between %s and %s", fromTag, toTag)
	}

	parsed := g.parseCommits(commits)
	grouped := g.groupCommits(parsed)

	changelog := g.formatChangelog(grouped, fromTag, toTag)

	improved, err := g.improveWithLLM(ctx, changelog, commits)
	if err != nil {
		return changelog, nil
	}

	return improved, nil
}

func (g *ChangelogGenerator) getCommits(fromTag, toTag string) ([]string, error) {
	var args []string
	if fromTag == "" {
		args = []string{"log", "--pretty=format:%H|||%s|||%b", toTag}
	} else {
		args = []string{"log", "--pretty=format:%H|||%s|||%b", fmt.Sprintf("%s..%s", fromTag, toTag)}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var commits []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			commits = append(commits, line)
		}
	}

	return commits, nil
}

func (g *ChangelogGenerator) parseCommits(commits []string) []Commit {
	var parsed []Commit

	for _, commit := range commits {
		parts := strings.Split(commit, "|||")
		if len(parts) < 2 {
			continue
		}

		hash := parts[0]
		subject := parts[1]
		body := ""
		if len(parts) > 2 {
			body = parts[2]
		}

		matches := commitPattern.FindStringSubmatch(subject)
		if matches == nil {
			continue
		}

		commitType := matches[1]
		scope := matches[2]
		breaking := matches[3] == "!"
		message := matches[4]

		if strings.Contains(body, "BREAKING CHANGE:") {
			breaking = true
		}

		parsed = append(parsed, Commit{
			Hash:     hash[:7],
			Type:     commitType,
			Scope:    scope,
			Message:  message,
			Body:     body,
			Breaking: breaking,
		})
	}

	return parsed
}

func (g *ChangelogGenerator) groupCommits(commits []Commit) map[string][]Commit {
	groups := make(map[string][]Commit)

	for _, commit := range commits {
		groups[commit.Type] = append(groups[commit.Type], commit)
	}

	return groups
}

func (g *ChangelogGenerator) formatChangelog(groups map[string][]Commit, fromTag, toTag string) string {
	var sb strings.Builder

	version := toTag
	if version == "HEAD" || version == "" {
		version = "Unreleased"
	}

	fmt.Fprintf(&sb, "## [%s] - %s\n\n", version, time.Now().Format("2006-01-02"))

	typeOrder := []string{"feat", "fix", "perf", "refactor", "docs", "test", "chore", "build", "ci"}
	typeNames := map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"perf":     "Performance",
		"refactor": "Code Refactoring",
		"docs":     "Documentation",
		"test":     "Tests",
		"chore":    "Chores",
		"build":    "Build System",
		"ci":       "CI/CD",
	}

	breaking := []Commit{}
	for _, commits := range groups {
		for _, commit := range commits {
			if commit.Breaking {
				breaking = append(breaking, commit)
			}
		}
	}

	if len(breaking) > 0 {
		sb.WriteString("### ⚠ BREAKING CHANGES\n\n")
		for _, commit := range breaking {
			fmt.Fprintf(&sb, "- **%s**: %s (%s)\n", commit.Scope, commit.Message, commit.Hash)
		}
		sb.WriteString("\n")
	}

	for _, typ := range typeOrder {
		commits, ok := groups[typ]
		if !ok || len(commits) == 0 {
			continue
		}

		sort.Slice(commits, func(i, j int) bool {
			return commits[i].Scope < commits[j].Scope
		})

		name := typeNames[typ]
		if name == "" {
			if len(typ) > 0 {
				name = strings.ToUpper(typ[:1]) + typ[1:]
			}
		}

		fmt.Fprintf(&sb, "### %s\n\n", name)

		for _, commit := range commits {
			if commit.Scope != "" {
				fmt.Fprintf(&sb, "- **%s**: %s (%s)\n", commit.Scope, commit.Message, commit.Hash)
			} else {
				fmt.Fprintf(&sb, "- %s (%s)\n", commit.Message, commit.Hash)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *ChangelogGenerator) improveWithLLM(ctx context.Context, changelog string, commits []string) (string, error) {
	prompt := fmt.Sprintf(`You are a technical writer. Improve this CHANGELOG entry for clarity and professionalism.

Rules:
- Keep the structure (headings, bullets)
- Keep commit hashes
- Improve wording for clarity
- Group related changes if appropriate
- Add brief context where helpful
- Keep it concise

Original CHANGELOG:
%s

Return ONLY the improved CHANGELOG, no explanations.`, changelog)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a technical writer that improves CHANGELOG entries for clarity and professionalism.",
		UserPrompt:   prompt,
		Model:        g.model,
	})

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text), nil
}
