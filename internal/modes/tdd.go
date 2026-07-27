package modes

import (
	"context"
	"fmt"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/prompt"
	"strings"
)

func RunTDD(builder *prompt.Builder, provider llm.Provider, model string, description string) error {
	sys := builder.BuildSystemPrompt(prompt.BuildOptions{
		Mode: "tdd",
		Hint: description,
	})

	user := fmt.Sprintf(`
Task:
Generate tests first, then the minimum implementation to pass them.

Format:
Use standard markdown code blocks (e.g., `+"```python"+` or `+"```go"+`).
Include the file path in a comment at the top of each block.

Details:
%s
`, description)

	resp, err := provider.Chat(context.Background(), llm.ChatRequest{
		SystemPrompt: sys,
		UserPrompt:   user,
		Model:        model,
	})
	if err != nil {
		return fmt.Errorf("chat error: %w", err)
	}

	fmt.Println(strings.TrimSpace(resp.Text))
	return nil
}
