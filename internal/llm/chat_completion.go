package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jadercorrea/gptcode/internal/config"
	"io"
	"net/http"
	"os"
	"strings"
)

type ChatCompletionProvider struct {
	APIKey  string
	BaseURL string
}

func NewChatCompletion(baseURL, backendName string) *ChatCompletionProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = baseURL + "/chat/completions"
	}

	apiKey := config.GetAPIKey(backendName)

	return &ChatCompletionProvider{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}

type providerPreferences struct {
	RequireParameters bool `json:"require_parameters,omitempty"`
}

type chatCompletionRequest struct {
	Model       string               `json:"model"`
	Messages    []chatCompletionMsg  `json:"messages"`
	Tools       []interface{}        `json:"tools,omitempty"`
	ToolChoice  *string              `json:"tool_choice,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	Temperature float64              `json:"temperature"`
	Provider    *providerPreferences `json:"provider,omitempty"`
}

type compoundChatRequest struct {
	Model          string              `json:"model"`
	Messages       []chatCompletionMsg `json:"messages"`
	CompoundCustom *compoundCustom     `json:"compound_custom,omitempty"`
	Stream         bool                `json:"stream,omitempty"`
	Temperature    float64             `json:"temperature"`
}

type compoundCustom struct {
	Tools *compoundTools `json:"tools,omitempty"`
}

type compoundTools struct {
	EnabledTools []string `json:"enabled_tools,omitempty"`
}

func extractToolNames(tools []interface{}) []string {
	if len(tools) == 0 {
		return nil
	}

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}

		if toolMap["type"] != "function" {
			continue
		}

		funcDef, ok := toolMap["function"].(map[string]interface{})
		if !ok {
			continue
		}

		if name, ok := funcDef["name"].(string); ok {
			names = append(names, name)
		}
	}

	return names
}

type chatCompletionMsg struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *ChatCompletionProvider) ChatStream(ctx context.Context, req ChatRequest, callback func(chunk string)) error {
	if c.APIKey == "" {
		return errors.New(`API key not defined for this backend

To get started for free:
1. Get a free API key from https://openrouter.ai/keys
2. Run: gt key openrouter

Or use Groq (free tier available):
1. Get a free API key from https://console.groq.com/keys
2. Run: gt key groq

Or use local models with Ollama:
1. Install Ollama: https://ollama.ai
2. Run: gt setup`)
	}

	messages := []chatCompletionMsg{
		{Role: "system", Content: req.SystemPrompt},
	}

	for _, msg := range req.Messages {
		messages = append(messages, chatCompletionMsg{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		})
	}

	if req.UserPrompt != "" {
		messages = append(messages, chatCompletionMsg{
			Role:    "user",
			Content: req.UserPrompt,
		})
	}

	isCompound := strings.Contains(req.Model, "compound")

	var b []byte
	if isCompound {
		toolNames := extractToolNames(req.Tools)
		compoundBody := compoundChatRequest{
			Model:    req.Model,
			Messages: messages,
			CompoundCustom: &compoundCustom{
				Tools: &compoundTools{
					EnabledTools: toolNames,
				},
			},
			Stream:      true,
			Temperature: 0.0,
		}
		b, _ = json.Marshal(compoundBody)
	} else {
		body := chatCompletionRequest{
			Model:       req.Model,
			Messages:    messages,
			Stream:      true,
			Temperature: 0.0,
		}
		if len(req.Tools) > 0 {
			body.Tools = req.Tools
			auto := "auto"
			body.ToolChoice = &auto
			body.Provider = &providerPreferences{RequireParameters: true}
		}
		b, _ = json.Marshal(body)
	}

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Title", "gptcode-agent")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "\n[HTTP %d] %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			callback(chunk.Choices[0].Delta.Content)
		}
	}

	return scanner.Err()
}

func (c *ChatCompletionProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c.APIKey == "" {
		return nil, errors.New(`API key not defined for this backend

To get started for free:
1. Get a free API key from https://openrouter.ai/keys
2. Run: gt key openrouter

Or use Groq (free tier available):
1. Get a free API key from https://console.groq.com/keys
2. Run: gt key groq

Or use local models with Ollama:
1. Install Ollama: https://ollama.ai
2. Run: gt setup`)
	}

	messages := []chatCompletionMsg{
		{Role: "system", Content: req.SystemPrompt},
	}

	for _, msg := range req.Messages {
		chatMsg := chatCompletionMsg{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}

		if len(msg.ToolCalls) > 0 {
			chatMsg.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				chatMsg.ToolCalls[i] = ToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				chatMsg.ToolCalls[i].Function.Name = tc.Name
				chatMsg.ToolCalls[i].Function.Arguments = tc.Arguments
			}
		}

		messages = append(messages, chatMsg)
	}

	if req.UserPrompt != "" {
		messages = append(messages, chatCompletionMsg{
			Role:    "user",
			Content: req.UserPrompt,
		})
	}

	isCompound := strings.Contains(req.Model, "compound")

	var b []byte
	if isCompound {
		toolNames := extractToolNames(req.Tools)
		compoundBody := compoundChatRequest{
			Model:    req.Model,
			Messages: messages,
			CompoundCustom: &compoundCustom{
				Tools: &compoundTools{
					EnabledTools: toolNames,
				},
			},
			Temperature: 0.0,
		}
		b, _ = json.Marshal(compoundBody)
	} else {
		body := chatCompletionRequest{
			Model:       req.Model,
			Messages:    messages,
			Temperature: 0.0,
		}
		if len(req.Tools) > 0 {
			body.Tools = req.Tools
			auto := "auto"
			body.ToolChoice = &auto
			body.Provider = &providerPreferences{RequireParameters: true}
		}
		b, _ = json.Marshal(body)
	}

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "\n=== REQUEST TO %s ===\n%s\n\n", c.BaseURL, string(b))
	}

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Title", "gptcode-agent")

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[HTTP] Making request to %s\n", c.BaseURL)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responseBody []byte
	responseBody, _ = io.ReadAll(resp.Body)

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "=== RESPONSE ===\n%s\n\n", string(responseBody))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Error != nil {
		var errorDetail struct {
			Message          string `json:"message"`
			FailedGeneration string `json:"failed_generation"`
		}
		if err := json.Unmarshal(responseBody, &struct {
			Error *struct {
				Message          string `json:"message"`
				FailedGeneration string `json:"failed_generation"`
			} `json:"error"`
		}{Error: &errorDetail}); err == nil && errorDetail.FailedGeneration != "" {
			parsedCalls := ParseToolCallsFromText(errorDetail.FailedGeneration)
			if len(parsedCalls) > 0 {
				return &ChatResponse{
					Text:      "",
					ToolCalls: parsedCalls,
				}, nil
			}
		}
		return nil, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, errors.New("empty response from API")
	}

	response := &ChatResponse{
		Text: apiResp.Choices[0].Message.Content,
	}

	if len(apiResp.Choices[0].Message.ToolCalls) > 0 {
		response.ToolCalls = make([]ChatToolCall, len(apiResp.Choices[0].Message.ToolCalls))
		for i, tc := range apiResp.Choices[0].Message.ToolCalls {
			response.ToolCalls[i] = ChatToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	// Populate token usage if available
	if apiResp.Usage != nil {
		response.TokenUsage = &TokenUsage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		}
	}

	return response, nil
}
