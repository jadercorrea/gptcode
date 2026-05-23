package modes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gptcode/internal/agents"
	"gptcode/internal/config"
	"gptcode/internal/llm"
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
	modelAlias := setup.Defaults.Model

	backendCfg := setup.Backend[backendName]
	model := backendCfg.DefaultModel
	if alias, ok := backendCfg.Models[modelAlias]; ok {
		model = alias
	} else if modelAlias != "" {
		model = modelAlias
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

	reviewAgent := agents.NewReview(provider, cwd, model)

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

	reviewPrompt := buildReviewPrompt(targetPath, info.IsDir(), opts.Focus)

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

	ctx := context.Background()
	result, err := reviewAgent.Execute(ctx, history, statusCallback)
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

func buildReviewPrompt(targetPath string, isDir bool, focus string) string {
	var prompt strings.Builder

	if isDir {
		fmt.Fprintf(&prompt, "Review the code in directory: %s\n\n", targetPath)
		prompt.WriteString("Use project_map to get an overview, then examine key files.\n")
	} else {
		fmt.Fprintf(&prompt, "Review the code in file: %s\n\n", targetPath)
		prompt.WriteString("Read and analyze the file thoroughly.\n")
	}

	if focus != "" {
		fmt.Fprintf(&prompt, "\nSpecial focus: %s\n", focus)
	}

	prompt.WriteString("\nProvide a structured review covering:\n")
	prompt.WriteString("1. Summary: Overall assessment\n")
	prompt.WriteString("2. Critical Issues: Bugs, security risks, or breaking problems\n")
	prompt.WriteString("3. Suggestions: Quality, performance, or maintainability improvements\n")
	prompt.WriteString("4. Nitpicks: Style, naming, or minor preferences\n")

	return prompt.String()
}
