package elixir

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/prompt"
)

func RunFeatureElixir(builder *prompt.Builder, provider llm.Provider, model string) error {
	desc := readAllStdin()
	if desc == "" {
		return fmt.Errorf("empty feature description")
	}

	proj, err := Detect("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: could not detect mix project, using defaults:", err)
		proj = &Project{
			Root:       ".",
			AppName:    "app",
			ModuleBase: "App",
		}
	} else {
		fmt.Fprintln(os.Stderr, "GPTCode: detected Mix project at", proj.Root, "app:", proj.AppName, "module:", proj.ModuleBase)
	}

	slug := SlugForDescription(desc)
	testPath, implPath := PathsForSlug(proj, slug)
	moduleName := ModuleNameForSlug(slug)

	hint := desc
	if len(hint) > 200 {
		hint = hint[:200]
	}
	sys := builder.BuildSystemPrompt(prompt.BuildOptions{
		Lang: "elixir",
		Mode: "tdd",
		Hint: hint,
	})

	user := fmt.Sprintf(`You are GPTCode, a strict TDD-first coding assistant for Elixir.

We are in a Mix project with:

- root: %s
- app: %s
- module base: %s

The user described this feature:

%s

We will implement this feature in a single module.

CONSTRAINTS:

- Use ExUnit tests.
- Use the module namespace "%s.%s".
- Keep functions small and intention-revealing.
- Handle edge cases explicitly (do not rely on defaults without tests).
- Do not introduce unnecessary abstractions.

1) First, restate the feature clearly in one or two sentences.
2) Then, use the following file paths exactly:

- tests at: %s
- implementation at: %s

3) Generate the following fenced blocks exactly:

`+"```"+`tests
# path: %s
# ExUnit tests for %s.%s
# Cover at least:
# - happy path(s)
# - empty list or nil inputs (if relevant)
# - any domain rules explicitly mentioned in the description
`+"```"+`

`+"```"+`impl
# path: %s
# Implementation of %s.%s module.
# Use pure functions where possible.
`+"```"+`

Do NOT use any other fences.
Do NOT include explanations outside those blocks.
`, proj.Root, proj.AppName, proj.ModuleBase, desc, proj.ModuleBase, moduleName,
		testPath, implPath,
		testPath, proj.ModuleBase, moduleName,
		implPath, proj.ModuleBase, moduleName)

	resp, err := provider.Chat(context.Background(), llm.ChatRequest{
		SystemPrompt: sys,
		UserPrompt:   user,
		Model:        model,
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	out := strings.TrimSpace(resp.Text)

	fmt.Println(out)

	writeElixirFilesFromBlocks(proj.Root, out)
	return nil
}

func readAllStdin() string {
	info, _ := os.Stdin.Stat()
	if (info.Mode() & os.ModeCharDevice) != 0 {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	var b strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		b.WriteString(scanner.Text())
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

type fencedBlock struct {
	path string
	body string
}

func extractBlock(text, tag string) fencedBlock {
	var result fencedBlock

	start := "```" + tag
	i := strings.Index(text, start)
	if i == -1 {
		return result
	}
	rest := text[i+len(start):]
	j := strings.Index(rest, "```")
	if j == -1 {
		return result
	}
	block := rest[:j]

	lines := strings.Split(block, "\n")
	var bodyLines []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if result.path == "" && strings.HasPrefix(trim, "# path:") {
			result.path = strings.TrimSpace(strings.TrimPrefix(trim, "# path:"))
			continue
		}
		if ln == "" && len(bodyLines) == 0 {
			continue
		}
		bodyLines = append(bodyLines, ln)
	}
	result.body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return result
}

func writeElixirFilesFromBlocks(root, raw string) {
	tests := extractBlock(raw, "tests")
	impl := extractBlock(raw, "impl")

	if tests.path != "" && tests.body != "" {
		writeFileUnderRoot(root, tests.path, tests.body)
	}

	if impl.path != "" && impl.body != "" {
		writeFileUnderRoot(root, impl.path, impl.body)
	}
}

func writeFileUnderRoot(root, relPath, body string) {
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to mkdir for", full, ":", err)
		return
	}
	if err := os.WriteFile(full, []byte(body+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to write", full, ":", err)
		return
	}
	fmt.Fprintln(os.Stderr, "GPTCode: wrote", full)
}
