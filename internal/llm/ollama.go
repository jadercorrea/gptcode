package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type OllamaProvider struct {
	BaseURL string
}

func NewOllama(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if !strings.HasSuffix(baseURL, "/api/chat") {
		baseURL = baseURL + "/api/chat"
	}
	return &OllamaProvider{BaseURL: baseURL}
}

type ollamaReq struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []interface{}   `json:"tools,omitempty"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumCtx int `json:"num_ctx,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaResp struct {
	Message struct {
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
}

type ollamaToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Index     int                    `json:"index"`
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

func (o *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest, callback func(chunk string)) error {
	messages := []ollamaMessage{
		{Role: "system", Content: req.SystemPrompt},
	}

	for _, msg := range req.Messages {
		if msg.Role != "tool" {
			messages = append(messages, ollamaMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	if req.UserPrompt != "" {
		messages = append(messages, ollamaMessage{
			Role:    "user",
			Content: req.UserPrompt,
		})
	}

	body := ollamaReq{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
		Tools:    req.Tools,
		Options:  configuredOllamaOptions(),
	}
	b, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL, bytes.NewReader(b))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk ollamaResp
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			callback(chunk.Message.Content)
		}
	}

	return scanner.Err()
}

func (o *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := []ollamaMessage{
		{Role: "system", Content: req.SystemPrompt},
	}

	for _, msg := range req.Messages {
		ollamaMsg := ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			ollamaMsg.ToolCalls = make([]ollamaToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				ollamaMsg.ToolCalls[i].Function.Name = tc.Name
				var args map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Arguments), &args)
				ollamaMsg.ToolCalls[i].Function.Arguments = args
			}
		}

		messages = append(messages, ollamaMsg)
	}

	if req.UserPrompt != "" {
		messages = append(messages, ollamaMessage{
			Role:    "user",
			Content: req.UserPrompt,
		})
	}

	body := ollamaReq{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
		Tools:    req.Tools,
		Options:  configuredOllamaOptions(),
	}
	b, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL, bytes.NewReader(b))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: configuredOllamaTimeout()}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var or ollamaResp
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return nil, err
	}

	response := &ChatResponse{
		Text: or.Message.Content,
	}

	if len(or.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]ChatToolCall, len(or.Message.ToolCalls))
		for i, tc := range or.Message.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			response.ToolCalls[i] = ChatToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: string(argsJSON),
			}
		}
	} else if strings.Contains(or.Message.Content, "<function=") {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "\n### XML DETECTED in response:\n%s\n\n", or.Message.Content)
		}
		parsedCalls := parseXMLToolCalls(or.Message.Content)
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "### PARSED %d XML tool calls\n\n", len(parsedCalls))
		}
		if len(parsedCalls) > 0 {
			response.ToolCalls = parsedCalls
			response.Text = strings.Split(or.Message.Content, "<function=")[0]
		}
	}

	return response, nil
}

func configuredOllamaOptions() ollamaOptions {
	raw := strings.TrimSpace(os.Getenv("GPTCODE_OLLAMA_CONTEXT_LENGTH"))
	if raw == "" {
		return ollamaOptions{}
	}

	numCtx, err := strconv.Atoi(raw)
	if err != nil || numCtx <= 0 {
		return ollamaOptions{}
	}
	return ollamaOptions{NumCtx: numCtx}
}

func configuredOllamaTimeout() time.Duration {
	const defaultTimeout = 10 * time.Minute

	raw := strings.TrimSpace(os.Getenv("GPTCODE_OLLAMA_TIMEOUT"))
	if raw == "" {
		return defaultTimeout
	}

	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultTimeout
	}
	return timeout
}

func parseXMLToolCalls(text string) []ChatToolCall {
	var calls []ChatToolCall

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "### parseXMLToolCalls input:\n%s\n\n", text)
	}

	funcRe := regexp.MustCompile(`<function=([^>]+)>(.*?)</function>`)
	funcMatches := funcRe.FindAllStringSubmatch(text, -1)

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "### Found %d function matches\n", len(funcMatches))
	}

	for idx, match := range funcMatches {
		if len(match) >= 3 {
			funcName := match[1]
			funcBody := match[2]

			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "### Function %d: %s, body: %s\n", idx, funcName, funcBody)
			}

			args := make(map[string]interface{})

			paramRe := regexp.MustCompile(`<parameter=([^>]+)>([^<]*)</parameter>`)
			paramMatches := paramRe.FindAllStringSubmatch(funcBody, -1)

			for _, pm := range paramMatches {
				if len(pm) >= 3 {
					paramName := strings.TrimSpace(pm[1])
					paramValue := strings.TrimSpace(pm[2])
					if paramValue != "" {
						args[paramName] = paramValue
					}
				}
			}

			argsJSON, _ := json.Marshal(args)
			calls = append(calls, ChatToolCall{
				ID:        fmt.Sprintf("xml_%d", idx),
				Name:      funcName,
				Arguments: string(argsJSON),
			})
		}
	}

	return calls
}
