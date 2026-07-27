package modes

import (
	"context"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/output"

	"golang.org/x/term"
)

func RunImplement(planPath string) error {
	planContent, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("could not read plan file: %w", err)
	}

	setup, _ := config.LoadSetup()
	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]
	cwd, _ := os.Getwd()

	fmt.Fprintf(os.Stderr, "⠋ Implementing plan from: %s\n\n", planPath)

	var customExec llm.Provider
	if backendCfg.Type == "ollama" {
		customExec = llm.NewOllama(backendCfg.BaseURL)
	} else {
		customExec = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	implementPrompt := fmt.Sprintf(`Implement this approved technical plan:

---
%s
---

Execute the plan phase by phase:
1. Read all files mentioned in each phase
2. Make the required code changes
3. Verify changes work (read files to confirm)
4. Move to next phase

Focus on making the actual code changes described in the plan.`, string(planContent))

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[IMPLEMENT] Plan length: %d bytes\n", len(planContent))
		fmt.Fprintf(os.Stderr, "[IMPLEMENT] Prompt preview: %s...\n", implementPrompt[:min(200, len(implementPrompt))])
	}

	editorModel := backendCfg.GetModelForAgent("editor")
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[IMPLEMENT] Using editor model: %s\n", editorModel)
	}
	editorAgent := agents.NewEditor(customExec, cwd, editorModel)
	implementResult, _, err := editorAgent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: implementPrompt}}, nil)
	if err != nil {
		return fmt.Errorf("implementation failed: %w", err)
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		rendered, err := output.RenderMarkdown(implementResult)
		if err != nil {
			rendered = implementResult
		}
		fmt.Println(output.Separator())
		fmt.Print(rendered)
		fmt.Println(output.Separator())
	} else {
		fmt.Println(implementResult)
	}

	fmt.Fprintf(os.Stderr, "\n✓ Implementation complete\n")
	fmt.Fprintf(os.Stderr, "\nNext steps:\n")
	fmt.Fprintf(os.Stderr, "  1. Review the changes\n")
	fmt.Fprintf(os.Stderr, "  2. Run tests: make test\n")
	fmt.Fprintf(os.Stderr, "  3. Run linting: make lint\n")

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
