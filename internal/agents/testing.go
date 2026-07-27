package agents

import (
	"context"

	"github.com/jadercorrea/gptcode/internal/llm"
)

// MockProvider simulates LLM responses for testing
type MockProvider struct {
	// Simple mode (single response)
	Response  string
	ToolCalls []llm.ChatToolCall

	// Multi-response mode (for sequential calls)
	Responses []llm.ChatResponse
	CallCount int

	// Legacy compatibility
	ToolCallsAt [][]llm.ChatToolCall
}

func (m *MockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	// Multi-response mode
	if len(m.Responses) > 0 {
		if m.CallCount >= len(m.Responses) {
			return &llm.ChatResponse{Text: "No more responses configured"}, nil
		}
		resp := m.Responses[m.CallCount]
		m.CallCount++
		return &resp, nil
	}

	// Legacy compatibility mode (review_test.go format)
	if len(m.ToolCallsAt) > 0 && m.CallCount < len(m.ToolCallsAt) {
		resp := &llm.ChatResponse{
			Text: m.Response,
		}
		if m.CallCount < len(m.ToolCallsAt) {
			resp.ToolCalls = m.ToolCallsAt[m.CallCount]
		}
		m.CallCount++
		return resp, nil
	}

	// Simple mode (single response)
	return &llm.ChatResponse{
		Text:      m.Response,
		ToolCalls: m.ToolCalls,
	}, nil
}

func (m *MockProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback func(string)) error {
	return nil
}
