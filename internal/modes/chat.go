package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"gptcode/internal/agents"
	"gptcode/internal/config"
	"gptcode/internal/graph"
	"gptcode/internal/llm"
	"gptcode/internal/output"
	"gptcode/internal/prompt"
)

type ChatHistory struct {
	Messages []llm.ChatMessage `json:"messages"`
}

func isOpsQuery(s string) bool {
	q := strings.ToLower(s)
	keys := []string{
		"system data",
		"disk usage",
		"storage",
		"disk space",
		"out of space",
		"investigate",
		"troubleshoot",
		"diagnose",
		"macos",
		"apfs",
		"snapshot",
		"time machine",
		"cache",
		"xcode",
		"docker",
		"high usage",
		"dados do sistema",
		"armazenamento",
		"disco",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func Chat(input string, args []string) {
	os.Stdout.Sync()

	fmt.Fprintf(os.Stderr, "[CHAT] Starting Chat function, input len=%d\n", len(input))

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[CHAT] Input: %s\n", input[:min(100, len(input))])
	}

	setup, _ := config.LoadSetup()

	var history ChatHistory
	if input != "" {
		// Try to unmarshal as JSON history
		if err := json.Unmarshal([]byte(input), &history); err != nil {
			// If input looks like JSON but failed to parse, log error and return
			// This prevents sending raw JSON as a user message which blows up context
			if len(input) > 0 && input[0] == '{' {
				fmt.Fprintf(os.Stderr, "Error parsing chat history: %v\n", err)
				return
			}
			// Otherwise treat as a single new message (CLI usage)
			history.Messages = []llm.ChatMessage{{Role: "user", Content: input}}
		}
	}

	// Truncate history to avoid context limits
	history.Messages = truncateHistory(history.Messages, 20)

	backendName := setup.Defaults.Backend

	if len(args) >= 2 && args[1] != "" {
		backendName = args[1]
	}

	backendCfg := setup.Backend[backendName]

	cwd, _ := os.Getwd()

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	researchModel := backendCfg.GetModelForAgent("research")
	orchestrator := llm.NewOrchestrator(backendCfg.BaseURL, backendName, provider, researchModel)

	if len(history.Messages) == 0 || history.Messages[len(history.Messages)-1].Role != "user" {
		fmt.Fprintln(os.Stderr, "\nERROR: Invalid message history")
		fmt.Println("Erro: Invalid message history - must have at least one user message")
		return
	}

	lastUserMessage := history.Messages[len(history.Messages)-1].Content

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[CHAT] Checking isOpsQuery for: %s\n", lastUserMessage)
		fmt.Fprintf(os.Stderr, "[CHAT] isOpsQuery result: %v\n", isOpsQuery(lastUserMessage))
	}

	// Check if this is an ops/troubleshooting query - route to run mode
	if isOpsQuery(lastUserMessage) {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "[CHAT] Ops query detected, routing to run mode")
		}
		builder := prompt.NewDefaultBuilder(nil)
		queryModel := backendCfg.GetModelForAgent("query")
		RunExecute(builder, provider, queryModel, []string{lastUserMessage}, nil, nil)
		return
	}

	if strings.ToLower(strings.TrimSpace(lastUserMessage)) == "implement" {
		fmt.Fprintln(os.Stderr, "[CHAT] Implement command detected")

		home, _ := os.UserHomeDir()
		planPath := filepath.Join(home, ".gptcode", "current_plan.txt")
		planContent, err := os.ReadFile(planPath)
		if err != nil {
			fmt.Println("No active plan found. Please create a plan first.")
			return
		}

		fmt.Fprintln(os.Stderr, "[CHAT] Starting implementation")
		queryModel := backendCfg.GetModelForAgent("query")
		guided := NewGuidedMode(orchestrator, cwd, queryModel)

		_ = guided.events.Status("Implementing plan...")
		if err := guided.Implement(context.Background(), string(planContent)); err != nil {
			fmt.Fprintln(os.Stderr, "Implementation error:", err)
			fmt.Println("Error:", err)
		} else {
			_ = guided.events.Complete()
			_ = guided.events.Message("Implementation complete. Review files and run tests.")
		}

		os.Stdout.Sync()
		time.Sleep(200 * time.Millisecond)
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}

	var stopSpinner chan bool
	if os.Getenv("GPTCODE_DEBUG") != "1" {
		stopSpinner = make(chan bool, 1)
		go showSpinner(stopSpinner)
	}

	routerModel := backendCfg.GetModelForAgent("router")
	editorModel := backendCfg.GetModelForAgent("editor")
	queryModel := backendCfg.GetModelForAgent("query")

	// Dependency Graph Integration
	// We build the graph and find relevant context to prepend to the message
	// This is a simple MVP integration
	if os.Getenv("GPTCODE_GRAPH") != "false" {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "[GRAPH] Building dependency graph...")
		}

		// Build graph
		builder := graph.NewBuilder(cwd)
		if g, err := builder.Build(); err == nil {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[GRAPH] Built graph: %d nodes, %d edges\n", len(g.Nodes), countEdges(g))
			}
			g.PageRank(0.85, 20)

			// Optimize context
			optimizer := graph.NewOptimizer(g)
			maxFiles := setup.Defaults.GraphMaxFiles
			if maxFiles == 0 {
				maxFiles = 5 // default
			}
			relevantFiles := optimizer.OptimizeContext(lastUserMessage, maxFiles)

			if len(relevantFiles) > 0 {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[GRAPH] Selected %d files:\n", len(relevantFiles))
					for i, f := range relevantFiles {
						fmt.Fprintf(os.Stderr, "[GRAPH]   %d. %s (score: %.3f)\n", i+1, f, g.Nodes[g.Paths[f]].Score)
					}
				}

				// Read file contents
				var contextBuilder strings.Builder
				contextBuilder.WriteString("\n\n[Context from Dependency Graph]\n")

				for _, file := range relevantFiles {
					content, err := os.ReadFile(filepath.Join(cwd, file))
					if err == nil {
						text := string(content)

						// Smart truncation: keep ~3000 chars (rough ~750 tokens)
						// For large files, try to keep relevant parts (imports + key functions)
						if len(text) > 3000 {
							lines := strings.Split(text, "\n")

							// Keep first 30 lines (imports, package decl)
							head := strings.Join(lines[:min(30, len(lines))], "\n")

							// Keep last 20 lines (often important functions)
							tailStart := max(30, len(lines)-20)
							tail := strings.Join(lines[tailStart:], "\n")

							text = fmt.Sprintf("%s\n\n... (%d lines omitted) ...\n\n%s", head, len(lines)-50, tail)
						}

						fmt.Fprintf(&contextBuilder, "File: %s\n```\n%s\n```\n", file, text)
					}
				}

				// Append to the last user message
				// We modify the history in place
				history.Messages[len(history.Messages)-1].Content += contextBuilder.String()
			}
		} else {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[GRAPH] Failed to build graph: %v\n", err)
			}
		}
	}

	coordinator := agents.NewCoordinator(provider, orchestrator, cwd, routerModel, editorModel, queryModel, researchModel)

	statusCallback := func(status string) {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[STATUS] %s\n", status)
		} else {
			fmt.Fprintf(os.Stderr, "\r[STATUS] %s", status)
		}
	}

	result, err := coordinator.Execute(context.Background(), history.Messages, statusCallback)

	if os.Getenv("GPTCODE_DEBUG") != "1" {
		stopSpinner <- true
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(os.Stderr, "\r\033[K")
	}

	if err != nil {
		fmt.Println("Erro:", err)
		return
	}

	isTerminal := isInteractiveTerminal()

	if isTerminal {
		parsed := output.ParseMarkdown(result)

		rendered, err := output.RenderMarkdown(parsed.RenderedText)
		if err != nil {
			rendered = result
		}

		fmt.Println(output.Separator())
		fmt.Print(rendered)
		fmt.Println(output.Separator())

		if len(parsed.CodeBlocks) > 0 {
			for _, block := range parsed.CodeBlocks {
				action := output.PromptCodeBlock(block, len(parsed.CodeBlocks))
				_ = output.HandleCodeBlock(action, block.Code)
			}
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, output.Success("All commands processed."))
			fmt.Fprintln(os.Stderr, "")
			fmt.Println(output.Separator())
		}
	} else {
		fmt.Println(result)
	}
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func showSpinner(done chan bool) {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			return
		default:
			fmt.Fprintf(os.Stderr, "\r%s Thinking...", spinner[i%len(spinner)])
			os.Stderr.Sync()
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func RunChat(builder *prompt.Builder, provider llm.Provider, model string, cliArgs []string) error {
	input, _ := io.ReadAll(os.Stdin)
	Chat(string(input), cliArgs)
	return nil
}

// ChatWithResponse executes chat and returns the response instead of printing it
// This is used by the REPL to capture responses for conversation history
func ChatWithResponse(input string, args []string) (string, error) {
	os.Stdout.Sync()

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[CHAT] ChatWithResponse: input len=%d\n", len(input))
	}

	setup, _ := config.LoadSetup()

	var history ChatHistory
	if input != "" {
		if err := json.Unmarshal([]byte(input), &history); err != nil {
			if len(input) > 0 && input[0] == '{' {
				return "", fmt.Errorf("error parsing chat history: %w", err)
			}
			history.Messages = []llm.ChatMessage{{Role: "user", Content: input}}
		}
	}

	history.Messages = truncateHistory(history.Messages, 20)

	backendName := setup.Defaults.Backend
	if len(args) >= 2 && args[1] != "" {
		backendName = args[1]
	}

	backendCfg := setup.Backend[backendName]
	cwd, _ := os.Getwd()

	var provider llm.Provider
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	researchModel := backendCfg.GetModelForAgent("research")
	orchestrator := llm.NewOrchestrator(backendCfg.BaseURL, backendName, provider, researchModel)

	if len(history.Messages) == 0 || history.Messages[len(history.Messages)-1].Role != "user" {
		return "", fmt.Errorf("invalid message history - must have at least one user message")
	}

	lastUserMessage := history.Messages[len(history.Messages)-1].Content

	// Check if this is an ops/troubleshooting query - route to run mode
	if isOpsQuery(lastUserMessage) {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "[CHAT] Ops query detected, routing to run mode")
		}
		builder := prompt.NewDefaultBuilder(nil)
		queryModel := backendCfg.GetModelForAgent("query")
		// For REPL, we can't use RunExecute as it prints directly
		// This is a limitation - ops queries in REPL will print instead of returning
		// TODO: Refactor RunExecute to support response capture
		RunExecute(builder, provider, queryModel, []string{lastUserMessage}, nil, nil)
		return "[Executed operational command]", nil
	}

	routerModel := backendCfg.GetModelForAgent("router")
	editorModel := backendCfg.GetModelForAgent("editor")
	queryModel := backendCfg.GetModelForAgent("query")

	// Build dependency graph context if enabled
	if os.Getenv("GPTCODE_GRAPH") != "false" {
		builder := graph.NewBuilder(cwd)
		if g, err := builder.Build(); err == nil {
			g.PageRank(0.85, 20)
			optimizer := graph.NewOptimizer(g)
			maxFiles := setup.Defaults.GraphMaxFiles
			if maxFiles == 0 {
				maxFiles = 5
			}
			relevantFiles := optimizer.OptimizeContext(lastUserMessage, maxFiles)

			if len(relevantFiles) > 0 {
				var contextBuilder strings.Builder
				contextBuilder.WriteString("\n\n[Context from Dependency Graph]\n")

				for _, file := range relevantFiles {
					content, err := os.ReadFile(filepath.Join(cwd, file))
					if err == nil {
						text := string(content)
						if len(text) > 3000 {
							lines := strings.Split(text, "\n")
							head := strings.Join(lines[:min(30, len(lines))], "\n")
							tailStart := max(30, len(lines)-20)
							tail := strings.Join(lines[tailStart:], "\n")
							text = fmt.Sprintf("%s\n\n... (%d lines omitted) ...\n\n%s", head, len(lines)-50, tail)
						}
						fmt.Fprintf(&contextBuilder, "File: %s\n```\n%s\n```\n", file, text)
					}
				}
				history.Messages[len(history.Messages)-1].Content += contextBuilder.String()
			}
		}
	}

	coordinator := agents.NewCoordinator(provider, orchestrator, cwd, routerModel, editorModel, queryModel, researchModel)

	statusCallback := func(status string) {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[STATUS] %s\n", status)
		}
	}

	result, err := coordinator.Execute(context.Background(), history.Messages, statusCallback)
	if err != nil {
		return "", fmt.Errorf("chat error: %w", err)
	}

	return result, nil
}

func truncateHistory(messages []llm.ChatMessage, maxMessages int) []llm.ChatMessage {
	if len(messages) <= maxMessages {
		return messages
	}

	// Always keep the system prompt if it exists (though usually it's added later)
	// For now, just keep the last N messages
	return messages[len(messages)-maxMessages:]
}

func countEdges(g *graph.Graph) int {
	count := 0
	for _, edges := range g.OutEdges {
		count += len(edges)
	}
	return count
}
