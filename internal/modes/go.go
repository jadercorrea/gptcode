package modes

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/prompt"
)

func RunGo(builder *prompt.Builder, provider llm.Provider, model string, args []string) error {
	query := strings.Join(args, " ")

	if query == "" {
		fmt.Fprintln(os.Stderr, "gt go - Direct execution mode")
		fmt.Fprintln(os.Stderr, "Usage: gt go <task or code>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gt go hello world")
		fmt.Fprintln(os.Stderr, "  gt go \"explain what Go is\"")
		fmt.Fprintln(os.Stderr, "  gt go \"write a hello world in Python\"")
		return nil
	}

	sys := builder.BuildSystemPrompt(prompt.BuildOptions{
		Lang: "general",
		Mode: "chat",
		Hint: query,
	})

	req := llm.ChatRequest{
		SystemPrompt: sys,
		Model:        model,
		Messages: []llm.ChatMessage{
			{Role: "user", Content: query},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	if resp.Text != "" {
		fmt.Println(strings.TrimSpace(resp.Text))
	}

	return nil
}
