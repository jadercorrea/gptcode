package modes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type ReviewOptions struct {
	Target string
	Focus  string
}

func RunReview(opts ReviewOptions) error {
	setup, err := config.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load setup: %w", err)
	}

	backendName := setup.Defaults.Backend

	backendCfg := setup.Backend[backendName]
	model := backendCfg.GetModelForAgent("query")
	if model == "" {
		model = backendCfg.DefaultModel
	}

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	target := opts.Target
	if target == "" {
		target = "."
	}

	targetPath := target
	if !filepath.IsAbs(target) {
		targetPath = filepath.Join(cwd, target)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("target not found: %w", err)
	}

	var fileEvidence string
	if !info.IsDir() {
		content, err := os.ReadFile(targetPath)
		if err != nil {
			return fmt.Errorf("read review target: %w", err)
		}
		fileEvidence = string(content)
	}
	reviewPrompt := buildReviewPrompt(targetPath, info.IsDir(), opts.Focus, fileEvidence)

	fmt.Printf("Reviewing: %s\n", target)
	if opts.Focus != "" {
		fmt.Printf("Focus: %s\n", opts.Focus)
	}
	fmt.Println()

	statusCallback := func(status string) {
		fmt.Fprintf(os.Stderr, "[STATUS] %s\n", status)
	}

	history := []llm.ChatMessage{
		{
			Role:    "user",
			Content: reviewPrompt,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var result string
	if fileEvidence != "" {
		statusCallback("Review: Analyzing supplied file evidence...")
		response, chatErr := provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: "You are a senior code reviewer. Review only the supplied evidence and requested focus. Do not use tools or infer unstated product requirements.",
			UserPrompt:   reviewPrompt,
			Model:        model,
			Intent:       "review",
		})
		if chatErr != nil {
			err = chatErr
		} else if response == nil {
			err = fmt.Errorf("provider returned an empty response")
		} else {
			result = response.Text
		}
	} else {
		reviewAgent := agents.NewReview(provider, cwd, model)
		result, err = reviewAgent.Execute(ctx, history, statusCallback)
	}
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("CODE REVIEW")
	fmt.Println(strings.Repeat("=", 80) + "\n")
	fmt.Println(result)
	fmt.Println()

	return nil
}

func buildReviewPrompt(targetPath string, isDir bool, focus string, fileEvidence string) string {
	var prompt strings.Builder

	if isDir {
		fmt.Fprintf(&prompt, "Review the code in directory: %s\n\n", targetPath)
		prompt.WriteString("Use project_map to get an overview, then examine key files.\n")
	} else {
		fmt.Fprintf(&prompt, "Review the code in file: %s\n\n", targetPath)
		prompt.WriteString("The exact file content is included below. Analyze this evidence directly; do not read the same file again.\n\n")
		prompt.WriteString("```text\n")
		prompt.WriteString(numberLines(fileEvidence))
		prompt.WriteString("\n```\n")
	}

	if focus != "" {
		fmt.Fprintf(&prompt, "\nSpecial focus: %s\n", focus)
		prompt.WriteString("Do not recommend changes that violate the requested focus or constraints.\n")
	}

	prompt.WriteString("\nProvide a structured review covering:\n")
	prompt.WriteString("1. Summary: Overall assessment\n")
	prompt.WriteString("2. Critical Issues: Bugs, security risks, or breaking problems\n")
	prompt.WriteString("3. Suggestions: Quality, performance, or maintainability improvements\n")
	prompt.WriteString("4. Nitpicks: Style, naming, or minor preferences\n")
	prompt.WriteString("\nFor every issue, cite file:line evidence. Separate observed defects from optional design ideas.\n")
	prompt.WriteString("Do not label an absent feature as a defect unless the supplied code or stated contract requires it.\n")
	prompt.WriteString("Do not invent lifecycle requirements, background work, or new public API methods. When the requested focus is satisfied and no defect is evidenced, say so explicitly.\n")
	prompt.WriteString("Treat an evidenced violation of the requested focus as a defect even if no broader product requirements were supplied.\n")
	prompt.WriteString("Private implementation changes do not alter the public API; do not claim that adding private synchronization changes exported constructors or methods.\n")

	return prompt.String()
}

func numberLines(content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	var numbered strings.Builder
	for index, line := range lines {
		fmt.Fprintf(&numbered, "%d | %s\n", index+1, line)
	}
	return numbered.String()
}
