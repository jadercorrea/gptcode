package agents

import (
	"context"

	"github.com/jadercorrea/gptcode/internal/llm"
)

type ResearchAgent struct {
	orchestrator *llm.OrchestratorProvider
}

func NewResearch(orchestrator *llm.OrchestratorProvider) *ResearchAgent {
	return &ResearchAgent{
		orchestrator: orchestrator,
	}
}

const researchPrompt = `You are a research assistant with access to external information sources.

You can use web_search to find current information, documentation, and answers to questions.

Be concise and cite sources when possible.`

func (r *ResearchAgent) Execute(ctx context.Context, history []llm.ChatMessage, statusCallback StatusCallback) (string, error) {
	if statusCallback != nil {
		statusCallback("Research: Searching/Summarizing...")
	}
	resp, err := r.orchestrator.Chat(ctx, llm.ChatRequest{
		SystemPrompt: researchPrompt,
		Messages:     history,
	})
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}
