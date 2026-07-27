package modes

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/output"

	"golang.org/x/term"
)

func RunPlan(args []string) error {
	task := ""
	if len(args) > 0 {
		task = strings.Join(args, " ")
	}

	if task == "" {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Fprintln(os.Stderr, "Plan mode - Create detailed implementation plan")
		fmt.Fprintln(os.Stderr, "\nWhat task would you like to plan?")
		fmt.Fprint(os.Stderr, "> ")
		if scanner.Scan() {
			task = scanner.Text()
		}
	}

	if task == "" {
		return fmt.Errorf("no task provided")
	}

	setup, _ := config.LoadSetup()
	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]
	cwd, _ := os.Getwd()

	urls := extractURLs(task)
	var externalContext string

	if len(urls) > 0 {
		fmt.Fprintf(os.Stderr, "⠋ Fetching external documentation...\n")

		var orchestrator *llm.OrchestratorProvider
		if backendCfg.Type == "ollama" {
			customExec := llm.NewOllama(backendCfg.BaseURL)
			orchestrator = llm.NewOrchestrator(backendCfg.BaseURL, backendName, customExec, backendCfg.DefaultModel)
		} else {
			customExec := llm.NewChatCompletion(backendCfg.BaseURL, backendName)
			orchestrator = llm.NewOrchestrator(backendCfg.BaseURL, backendName, customExec, backendCfg.DefaultModel)
		}

		researchAgent := agents.NewResearch(orchestrator)
		for _, url := range urls {
			docPrompt := fmt.Sprintf("Visit %s and summarize key implementation details for: %s", url, task)
			docResult, err := researchAgent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: docPrompt}}, nil)
			if err == nil {
				externalContext += fmt.Sprintf("\n\n## External Documentation\n\n%s", docResult)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "⠋ Analyzing codebase...\n")

	var customExec llm.Provider
	if backendCfg.Type == "ollama" {
		customExec = llm.NewOllama(backendCfg.BaseURL)
	} else {
		customExec = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	queryModel := backendCfg.GetModelForAgent("query")
	queryAgent := agents.NewQuery(customExec, cwd, queryModel)

	codebasePrompt := fmt.Sprintf(`Brief codebase overview for: %s

1. List root directory (use list_files on ".")
2. Identify main language/framework
3. Suggest 2-3 key directories for implementation

Keep response under 150 words.`, task)

	codebaseAnalysis, err := queryAgent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: codebasePrompt}}, nil)
	if err != nil {
		return fmt.Errorf("codebase analysis failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "⠋ Creating implementation plan...\n")

	planPrompt := fmt.Sprintf(`Create a detailed implementation plan for this task:

%s

## Codebase Analysis

%s

%s

Create a structured plan with:

# [Task Name] Implementation Plan

## Overview
[Brief description of what we're implementing and why]

## Current State Analysis
[What exists now, what's missing, key constraints discovered]

## Desired End State
[Specification of the desired end state after this plan is complete, and how to verify it]

## Key Discoveries
- [Important finding with file:line reference]
- [Pattern to follow]
- [Constraint to work within]

## What We're NOT Doing
[Explicitly list out-of-scope items to prevent scope creep]

## Implementation Approach
[High-level strategy and reasoning]

## Phase 1: [Descriptive Name]

### Overview
[What this phase accomplishes]

### Changes Required

#### 1. [Component/File Group]
**Existing file**: path/to/existing_file.ext
**New file**: path/to/new_file.ext
**Changes**: [Summary of changes]

### Success Criteria

#### Automated Verification:
- [ ] Use only commands and Make targets verified in this repository

#### Manual Verification:
- [ ] Feature works as expected when tested
- [ ] No regressions in related features

**Implementation Note**: After completing this phase and all automated verification passes, pause for manual confirmation before proceeding to next phase.

---

[Repeat Phase structure for each major step]

## Testing Strategy
[What to test and how]

## References
[Links to research, similar code, etc.]

Grounding requirements:
- Mark every path as either **Existing file** or **New file**.
- Existing files must exist in the repository. Never invent a path.
- Verification commands and Make targets must exist in the repository.
- If repository evidence is insufficient, state what must be inspected instead of guessing.`, task, codebaseAnalysis, externalContext)

	editorModel := backendCfg.GetModelForAgent("editor")
	editorAgent := agents.NewEditor(customExec, cwd, editorModel)
	planResult, _, err := editorAgent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: planPrompt}}, nil)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}
	if problems := validatePlanEvidence(planResult, cwd); len(problems) > 0 {
		correctionPrompt := fmt.Sprintf(`The previous plan failed deterministic repository validation:

%s

Rewrite the plan. Preserve the task and useful reasoning, but remove invented
files and commands. Mark planned files as **New file** and cite existing paths
only when they exist. Return only the corrected plan.

Previous plan:
%s`, strings.Join(problems, "\n"), planResult)
		planResult, _, err = editorAgent.Execute(
			context.Background(),
			[]llm.ChatMessage{{Role: "user", Content: correctionPrompt}},
			nil,
		)
		if err != nil {
			return fmt.Errorf("correct ungrounded plan: %w", err)
		}
		if remaining := validatePlanEvidence(planResult, cwd); len(remaining) > 0 {
			return fmt.Errorf("generated plan failed repository validation: %s", strings.Join(remaining, "; "))
		}
	}

	home, _ := os.UserHomeDir()
	plansDir := filepath.Join(home, ".gptcode", "plans")
	_ = os.MkdirAll(plansDir, 0755)

	if term.IsTerminal(int(os.Stdout.Fd())) {
		rendered, err := output.RenderMarkdown(planResult)
		if err != nil {
			rendered = planResult
		}
		fmt.Println(output.Separator())
		fmt.Print(rendered)
		fmt.Println(output.Separator())
	} else {
		fmt.Println(planResult)
	}

	timestamp := time.Now().Format("2006-01-02")
	sanitizedTask := sanitizeFilename(task)
	filename := fmt.Sprintf("%s_%s.md", timestamp, sanitizedTask)
	planPath := filepath.Join(plansDir, filename)

	err = os.WriteFile(planPath, []byte(planResult), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Could not save plan to %s: %v\n", planPath, err)
	} else {
		fmt.Fprintf(os.Stderr, "\n✓ Plan saved to: %s\n", planPath)
		fmt.Fprintf(os.Stderr, "\nTo implement this plan, run:\n  chu implement %s\n", planPath)
	}

	return nil
}

var makeCommandPattern = regexp.MustCompile(`\bmake\s+([A-Za-z0-9_.-]+)`)

func validatePlanEvidence(plan, repository string) []string {
	var problems []string
	for _, line := range strings.Split(plan, "\n") {
		path, existing := planExistingFile(line)
		if !existing {
			continue
		}
		if _, err := os.Stat(filepath.Join(repository, filepath.FromSlash(path))); err != nil {
			problems = append(problems, fmt.Sprintf("existing file does not exist: %s", path))
		}
	}

	targets := repositoryMakeTargets(repository)
	for _, match := range makeCommandPattern.FindAllStringSubmatch(plan, -1) {
		target := match[1]
		if !targets[target] {
			problems = append(problems, fmt.Sprintf("Make target does not exist: make %s", target))
		}
	}

	sort.Strings(problems)
	return problems
}

func planExistingFile(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"**Existing file**:", "**File**:"} {
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		path = strings.Trim(path, "` ")
		return path, path != ""
	}
	return "", false
}

func repositoryMakeTargets(repository string) map[string]bool {
	targets := make(map[string]bool)
	content, err := os.ReadFile(filepath.Join(repository, "Makefile"))
	if err != nil {
		return targets
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || !strings.Contains(line, ":") {
			continue
		}
		names := strings.Fields(strings.SplitN(line, ":", 2)[0])
		for _, name := range names {
			if name != ".PHONY" && !strings.ContainsAny(name, "$%") {
				targets[name] = true
			}
		}
	}
	return targets
}
