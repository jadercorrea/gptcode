package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jadercorrea/gptcode/internal/acp"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/maestro"
	"github.com/jadercorrea/gptcode/internal/tools"
)

var (
	acpHTTP bool
	acpPort int
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run as an ACP (Agent Client Protocol) agent",
	Long: `Start GPTCode as an ACP-compliant agent, communicating via JSON-RPC 2.0.

By default, uses stdio transport (stdin/stdout). Any ACP-compatible editor
(Zed, JetBrains, Neovim, VS Code) can launch this as a subprocess.

With --http, starts an HTTP server for remote clients (e.g., Live dashboard).

For more information, see: https://agentclientprotocol.com`,
	RunE: runACP,
}

func init() {
	acpCmd.Flags().BoolVar(&acpHTTP, "http", false, "Use HTTP transport instead of stdio")
	acpCmd.Flags().IntVar(&acpPort, "port", 8080, "HTTP port (only with --http)")
	rootCmd.AddCommand(acpCmd)
}

func runACP(cmd *cobra.Command, args []string) error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "[ACP] Signal received, shutting down")
		cancel()
	}()

	// Create the session handler that bridges ACP to our Maestro engine
	handler := &acpSessionHandler{}

	// Create the ACP server
	server := acp.NewServer(handler)
	handler.server = server

	if acpHTTP {
		// HTTP transport for remote clients (Live dashboard, Fly.io)
		fmt.Fprintf(os.Stderr, "[ACP] Starting HTTP transport on :%d\n", acpPort)
		transport := acp.NewHTTPTransport(server, acpPort)
		return transport.ListenAndServe(ctx)
	}

	// Default: stdio transport for local editors
	return server.Run(ctx)
}

// acpSessionHandler bridges ACP sessions to the GPTCode Maestro engine.
type acpSessionHandler struct {
	server *acp.Server
}

func (h *acpSessionHandler) HandlePrompt(ctx context.Context, sessionID string, content []acp.ContentBlock, emitter acp.UpdateEmitter) (acp.SessionPromptResult, error) {
	// Extract the text prompt
	var prompt string
	for _, block := range content {
		if block.Type == "text" {
			if prompt != "" {
				prompt += "\n"
			}
			prompt += block.Text
		}
	}

	if prompt == "" {
		return acp.SessionPromptResult{StopReason: "endTurn"}, nil
	}

	// Get session info for working directory
	session := h.server.GetSession(sessionID)
	cwd := "."
	if session != nil && session.WorkingDirectory != "" {
		cwd = session.WorkingDirectory
	}

	// Load setup config
	setup, err := config.LoadSetup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ACP] Failed to load setup: %v, using defaults\n", err)
		setup = &config.Setup{
			Backend: make(map[string]config.BackendConfig),
		}
		setup.Defaults.Backend = "openrouter"
	}

	// Determine mode from session config
	mode := "edit"
	if session != nil && session.ConfigOptions != nil {
		if m, ok := session.ConfigOptions["mode"]; ok {
			mode = m
		}
	}

	// Create model selector
	selector, err := config.NewModelSelector(setup)
	if err != nil {
		return acp.SessionPromptResult{}, fmt.Errorf("failed to create model selector: %w", err)
	}

	// Determine language from session or default
	language := setup.Defaults.Lang
	if language == "" {
		language = "go"
	}

	// Create the Maestro conductor
	conductor := maestro.NewConductor(selector, setup, cwd, language)

	// Create the ACP tool bridge for delegating to editor
	bridge := acp.NewToolsBridge(h.server, cwd)

	// Emit plan at start
	emitter.EmitPlan("Processing prompt", []acp.PlanStep{
		{Title: "Analyzing task", Status: "running"},
		{Title: "Planning solution", Status: "pending"},
		{Title: "Implementing changes", Status: "pending"},
	})

	// For simple queries (mode=query/research), use direct LLM call
	if mode == "query" || mode == "research" {
		emitter.EmitPlan("Research", []acp.PlanStep{
			{Title: "Researching", Status: "running"},
		})

		backendName := setup.Defaults.Backend
		if backendName == "" {
			backendName = "openrouter"
		}
		baseURL := "https://openrouter.ai/api/v1"
		if bc, ok := setup.Backend[backendName]; ok && bc.BaseURL != "" {
			baseURL = bc.BaseURL
		}

		provider := llm.NewChatCompletion(baseURL, backendName)
		model := setup.Defaults.Model
		if model == "" {
			model = "anthropic/claude-sonnet-4"
		}

		resp, err := provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: "You are GPTCode, an expert coding assistant.",
			Model:        model,
			UserPrompt:   prompt,
		})
		if err != nil {
			return acp.SessionPromptResult{}, err
		}

		emitter.EmitText(resp.Text)
		return acp.SessionPromptResult{StopReason: "endTurn"}, nil
	}

	// For edit/plan/review modes, use the full Maestro pipeline
	emitter.EmitPlan("Executing task", []acp.PlanStep{
		{Title: "Analyzing task", Status: "completed"},
		{Title: "Planning solution", Status: "running"},
		{Title: "Implementing changes", Status: "pending"},
	})

	// Execute via conductor — the bridge will delegate tools to the editor
	_ = bridge // Bridge will be used when we wire it into the conductor's tool executor
	err = conductor.ExecuteTask(ctx, prompt, "complex")
	if err != nil {
		emitter.EmitPlan("Task failed", []acp.PlanStep{
			{Title: "Analyzing task", Status: "completed"},
			{Title: "Planning solution", Status: "error"},
		})
		return acp.SessionPromptResult{}, err
	}

	emitter.EmitPlan("Task completed", []acp.PlanStep{
		{Title: "Analyzing task", Status: "completed"},
		{Title: "Planning solution", Status: "completed"},
		{Title: "Implementing changes", Status: "completed"},
	})

	emitter.EmitText("\n\n✅ Task completed successfully.")

	return acp.SessionPromptResult{StopReason: "endTurn"}, nil
}

// Ensure tools package is referenced (used by bridge)
var _ = tools.ExecuteTool
