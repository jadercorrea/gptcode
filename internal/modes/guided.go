package modes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/events"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/ml"
)

type GuidedMode struct {
	events       *events.Emitter
	provider     llm.Provider
	baseProvider llm.Provider
	cwd          string
	model        string
	editorModel  string
}

func NewGuidedMode(provider llm.Provider, cwd string, model string) *GuidedMode {
	return &GuidedMode{
		events:       events.NewEmitter(os.Stderr),
		provider:     provider,
		baseProvider: provider,
		cwd:          cwd,
		model:        model,
		editorModel:  model,
	}
}

func NewGuidedModeWithCustomModel(provider llm.Provider, baseProvider llm.Provider, cwd string, model string, editorModel string) *GuidedMode {
	return &GuidedMode{
		events:       events.NewEmitter(os.Stderr),
		provider:     provider,
		baseProvider: baseProvider,
		cwd:          cwd,
		model:        model,
		editorModel:  editorModel,
	}
}

func (g *GuidedMode) Execute(ctx context.Context, userMessage string) error {
	_, err := g.ExecuteAndReturnPlan(ctx, userMessage)
	return err
}

func (g *GuidedMode) ExecuteAndReturnPlan(ctx context.Context, userMessage string) (string, error) {
	_ = g.events.Status("Analyzing task...")

	draftPlan, err := g.createDraftPlan(ctx, userMessage)
	if err != nil {
		return "", fmt.Errorf("failed to create draft: %w", err)
	}

	draftPath, err := g.saveDraft(draftPlan)
	if err != nil {
		return "", fmt.Errorf("failed to save draft: %w", err)
	}

	_ = g.events.OpenPlan(draftPath)
	_ = g.events.Message("Draft plan created.")

	_ = g.events.Status("Creating detailed plan...")

	fullPlan, err := g.createDetailedPlan(ctx, userMessage, draftPlan)
	if err != nil {
		return "", fmt.Errorf("failed to create plan: %w", err)
	}

	planPath, err := g.savePlan(fullPlan)
	if err != nil {
		return "", fmt.Errorf("failed to save plan: %w", err)
	}

	_ = g.events.OpenPlan(planPath)
	_ = g.events.Message("Detailed plan ready. Send 'implement' to start implementation.")

	return fullPlan, nil
}

func (g *GuidedMode) createDraftPlan(ctx context.Context, task string) (string, error) {
	prompt := fmt.Sprintf(`You are creating a DRAFT implementation plan.

Task: %s

Create a brief outline covering:
1. Overview (2-3 sentences)
2. Key steps (3-5 bullet points)
3. Files likely affected

Keep it concise - this is just a draft for user approval.`, task)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You create concise technical plans.",
		UserPrompt:   prompt,
		Model:        g.model,
	})
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func (g *GuidedMode) createDetailedPlan(ctx context.Context, task string, draft string) (string, error) {
	research := "No research available"

	if orchestrator, ok := g.provider.(*llm.OrchestratorProvider); ok {
		researchAgent := agents.NewResearch(orchestrator)

		statusCallback := func(status string) {
			_ = g.events.Status(status)
		}

		history := []llm.ChatMessage{
			{Role: "user", Content: fmt.Sprintf("Research this codebase for: %s", task)},
		}

		if result, err := researchAgent.Execute(ctx, history, statusCallback); err == nil {
			research = result
		}
	}

	prompt := fmt.Sprintf(`Create a SIMPLE, DIRECT implementation plan.

Task: %s

Draft outline:
%s

Codebase research:
%s

IMPORTANT CONSTRAINTS:
- Keep it MINIMAL - only what's strictly necessary
- NO extra features, NO tests unless explicitly requested
- NO scripts, NO automation unless asked
- ONLY modify files that already exist OR that are explicitly requested
- Focus on the EXACT task, nothing more

Create a brief plan:
# Plan

## What to do
[1-2 sentences]

## Files to modify
[List ONLY files that exist or are explicitly requested]

## Changes
[Specific, minimal changes to make]`, task, draft, research)

	resp, err := g.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You create MINIMAL, DIRECT plans. Focus ONLY on what's asked. NO extra features.",
		UserPrompt:   prompt,
		Model:        g.model,
	})
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

func (g *GuidedMode) Implement(ctx context.Context, plan string) error {
	allowedFiles := extractFilesFromPlan(plan)

	var editorAgent *agents.EditorAgent
	if len(allowedFiles) > 0 {
		editorAgent = agents.NewEditorWithFileValidation(g.baseProvider, g.cwd, g.editorModel, allowedFiles)
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[IMPLEMENT] Allowed files: %v\n", allowedFiles)
		}
	} else {
		editorAgent = agents.NewEditor(g.baseProvider, g.cwd, g.editorModel)
	}

	statusCallback := func(status string) {
		_ = g.events.Status(status)
	}

	reviewerAgent := agents.NewReviewer(g.baseProvider, g.cwd, g.model)

	// Higher retry limit for autonomous error fixing
	// Allows multiple fix-verify-fix cycles
	maxRetries := 9 // 10 total attempts (0-9)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[IMPLEMENT] Attempt %d/%d\n", attempt+1, maxRetries+1)
		}

		implementPrompt := fmt.Sprintf(`Implement EXACTLY what this plan says - NOTHING MORE:

---
%s
---

RULES:
- ONLY modify files explicitly listed in the plan
- ONLY make the specific changes described
- Do NOT create extra files (scripts, tests, configs) unless the plan says to
- Do NOT add features not mentioned in the plan
- If a file doesn't exist and isn't explicitly requested, DON'T create it
- When done, stop - do NOT keep iterating

Execute the plan directly and minimally.`, plan)

		history := []llm.ChatMessage{
			{Role: "user", Content: implementPrompt},
		}

		result, modifiedFiles, err := editorAgent.Execute(ctx, history, statusCallback)
		if err != nil {
			return err
		}

		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[IMPLEMENT] Editor result: %s\n", result)
		}

		if strings.Contains(result, "reached max iterations") {
			return fmt.Errorf("editor reached max iterations without completing task")
		}

		// Use actually modified files for review if available, otherwise fallback to plan files
		filesToValidate := allowedFiles
		if len(modifiedFiles) > 0 {
			filesToValidate = modifiedFiles
		}

		reviewResult, err := reviewerAgent.Review(ctx, plan, filesToValidate, statusCallback)
		if err != nil {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[IMPLEMENT] Validation failed: %v\n", err)
			}
			return err
		}

		if reviewResult.Success {
			_ = g.events.Message("Implementation validated successfully.")
			return nil
		}

		if attempt < maxRetries {
			_ = g.events.Message(fmt.Sprintf("Validation failed. Retrying... (attempt %d/%d)", attempt+2, maxRetries+1))

			feedback := "VALIDATION FAILED. Issues found:\n"
			for _, issue := range reviewResult.Issues {
				feedback += "- " + issue + "\n"
			}
			feedback += "\nFix these issues and try again."

			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[IMPLEMENT] Feedback: %s\n", feedback)
			}

			editorAgent = agents.NewEditorWithFileValidation(g.baseProvider, g.cwd, g.editorModel, allowedFiles)
		} else {
			_ = g.events.Message("Implementation completed but review failed after max retries.")
			for _, issue := range reviewResult.Issues {
				_ = g.events.Message("  - " + issue)
			}
			return fmt.Errorf("review failed: %v", reviewResult.Issues)
		}
	}

	return nil
}

func (g *GuidedMode) saveDraft(content string) (string, error) {
	home, _ := os.UserHomeDir()
	dir := fmt.Sprintf("%s/.gptcode/plans", home)
	_ = os.MkdirAll(dir, 0755)

	path := fmt.Sprintf("%s/draft.md", dir)
	return path, os.WriteFile(path, []byte(content), 0644)
}

func (g *GuidedMode) savePlan(content string) (string, error) {
	home, _ := os.UserHomeDir()
	dir := fmt.Sprintf("%s/.gptcode/plans", home)
	_ = os.MkdirAll(dir, 0755)

	timestamp := time.Now().Format("2006-01-02-150405")
	path := fmt.Sprintf("%s/%s-plan.md", dir, timestamp)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	currentPath := fmt.Sprintf("%s/.gptcode/current_plan.txt", home)
	_ = os.WriteFile(currentPath, []byte(content), 0644)

	return path, nil
}

func IsComplexTask(message string) bool {
	// Try to load ML model
	p, err := ml.LoadEmbedded("complexity_detection")
	if err != nil {
		return false
	}
	label, probs := p.Predict(message)
	if label == "multistep" {
		return true
	}
	if label == "complex" {
		threshold := 0.55
		if setup, err2 := config.LoadSetup(); err2 == nil {
			if setup.Defaults.MLComplexThreshold > 0 {
				threshold = setup.Defaults.MLComplexThreshold
			}
		}
		if v, ok := probs["complex"]; ok && v >= threshold {
			return true
		}
	}
	return false
}

func extractFilesFromPlan(plan string) []string {
	filePattern := regexp.MustCompile(`(?m)(?:[^\s]+/)?[^\s/]+\.(go|md|ts|tsx|js|jsx|py|rb|java|c|cpp|h|hpp|rs|yaml|yml|json|toml|txt|sh|sql|html|css|scss)`)
	matches := filePattern.FindAllString(plan, -1)

	seen := make(map[string]bool)
	var files []string

	for _, match := range matches {
		cleanPath := strings.Trim(match, "`:*")
		if !seen[cleanPath] {
			seen[cleanPath] = true
			files = append(files, cleanPath)
		}
	}

	for i, file := range files {
		if !filepath.IsAbs(file) && !strings.HasPrefix(file, "./") {
			files[i] = file
		}
	}

	return files
}
