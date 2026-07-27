package modes

import (
	"context"
	"fmt"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/autonomous"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/events"
	"github.com/jadercorrea/gptcode/internal/live"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/maestro"
)

// AutonomousExecutor wraps autonomous execution for use across modes
type AutonomousExecutor struct {
	events    *events.Emitter
	provider  llm.Provider
	cwd       string
	model     string
	executor  *autonomous.Executor
	conductor *maestro.Conductor
}

// NewAutonomousExecutor creates a new autonomous executor (no Live integration)
func NewAutonomousExecutor(provider llm.Provider, cwd string, model string, language string) *AutonomousExecutor {
	return NewAutonomousExecutorWithLive(provider, cwd, model, language, nil, nil, "")
}

// NewAutonomousExecutorWithLive creates executor with Live Dashboard integration
func NewAutonomousExecutorWithLive(provider llm.Provider, cwd string, model string, language string, liveClient *live.Client, liveConfig *live.ReportConfig, backendName string) *AutonomousExecutor {
	// Load setup
	setup, err := config.LoadSetup()
	if err != nil {
		fmt.Printf("[WARN] Failed to load setup: %v, using defaults\n", err)
		setup = &config.Setup{
			Backend: make(map[string]config.BackendConfig),
		}
		setup.Defaults.Backend = "groq"
	}

	// Override backend if specified (for retry system)
	if backendName != "" {
		setup.Defaults.Backend = backendName
	}

	// Create model selector
	selector, err := config.NewModelSelector(setup)
	if err != nil {
		fmt.Printf("[WARN] Failed to create model selector: %v\n", err)
	}

	// Create Maestro
	conductor := maestro.NewConductor(selector, setup, cwd, language)

	// Set up Live Dashboard integration
	if liveClient != nil {
		conductor.SetLiveClient(liveClient)
	}
	if liveConfig != nil {
		conductor.SetLiveReportConfig(liveConfig)
	}

	// Create autonomous components
	classifier := agents.NewClassifier(provider, model)
	analyzer := autonomous.NewTaskAnalyzer(classifier, provider, cwd, model)

	executor := autonomous.NewExecutor(analyzer, conductor, cwd)

	return &AutonomousExecutor{
		events:    events.NewEmitter(nil),
		provider:  provider,
		cwd:       cwd,
		model:     model,
		executor:  executor,
		conductor: conductor,
	}
}

// Execute runs autonomous execution with Symphony pattern if task is complex
func (a *AutonomousExecutor) Execute(ctx context.Context, task string) error {
	// Delegate to autonomous executor
	return a.executor.Execute(ctx, task)
}

// ShouldUseAutonomous determines if a task should use autonomous mode
// This is a lightweight heuristic check before full analysis.
// The real complexity scoring happens in TaskAnalyzer.estimateComplexity()
func ShouldUseAutonomous(ctx context.Context, task string) bool {
	// Always return false - let the ML-based complexity analysis decide
	// TaskAnalyzer.Analyze() will trigger Symphony if complexity >= 7
	return false
}
