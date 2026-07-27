package agents

import (
	"context"
	"fmt"
	"os"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type Coordinator struct {
	classifier *Classifier
	editor     *EditorAgent
	query      *QueryAgent
	research   *ResearchAgent
	review     *ReviewAgent
}

func NewCoordinator(
	provider llm.Provider,
	orchestrator *llm.OrchestratorProvider,
	cwd string,
	routerModel string,
	editorModel string,
	queryModel string,
	researchModel string,
) *Coordinator {
	return &Coordinator{
		classifier: NewClassifier(provider, routerModel),
		editor:     NewEditor(provider, cwd, editorModel),
		query:      NewQuery(provider, cwd, queryModel),
		research:   NewResearch(orchestrator),
		review:     NewReview(provider, cwd, queryModel),
	}
}

func (c *Coordinator) Execute(ctx context.Context, history []llm.ChatMessage, statusCallback StatusCallback) (string, error) {
	// Use the last user message for intent classification
	lastMessage := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			lastMessage = history[i].Content
			break
		}
	}

	if statusCallback != nil {
		statusCallback("Classifier: Analyzing intent...")
	}

	intent, err := c.classifier.ClassifyIntent(ctx, lastMessage)
	if err != nil {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[COORDINATOR] Classifier error: %v, defaulting to query\n", err)
		}
		intent = IntentQuery
	}

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[COORDINATOR] Intent classified as: %s\n", intent)
	}

	if statusCallback != nil {
		statusCallback(fmt.Sprintf("Coordinator: Routing to %s agent...", intent))
	}

	switch intent {
	case IntentEdit:
		res, _, err := c.editor.Execute(ctx, history, statusCallback)
		return res, err
	case IntentQuery:
		return c.query.Execute(ctx, history, statusCallback)
	case IntentResearch:
		return c.research.Execute(ctx, history, statusCallback)
	case IntentTest:
		res, _, err := c.editor.Execute(ctx, history, statusCallback)
		return res, err
	case IntentReview:
		return c.review.Execute(ctx, history, statusCallback)
	default:
		return c.query.Execute(ctx, history, statusCallback)
	}
}
