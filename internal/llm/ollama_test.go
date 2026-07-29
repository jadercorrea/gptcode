package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaChatReturnsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"model is required"}`))
	}))
	defer server.Close()

	provider := NewOllama(server.URL)
	_, err := provider.Chat(context.Background(), ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("expected Ollama server error, got %v", err)
	}
}

func TestOllamaChatUsesConfiguredContextWindow(t *testing.T) {
	t.Setenv("GPTCODE_OLLAMA_CONTEXT_LENGTH", "32768")

	var request struct {
		Options struct {
			NumCtx int `json:"num_ctx"`
		} `json:"options"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"ok"}}`))
	}))
	defer server.Close()

	provider := NewOllama(server.URL)
	if _, err := provider.Chat(context.Background(), ChatRequest{Model: "local"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if request.Options.NumCtx != 32768 {
		t.Fatalf("num_ctx = %d, want 32768", request.Options.NumCtx)
	}
}

func TestOllamaChatUsesConfiguredTimeout(t *testing.T) {
	t.Setenv("GPTCODE_OLLAMA_TIMEOUT", "10ms")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"message":{"content":"late"}}`))
	}))
	defer server.Close()

	provider := NewOllama(server.URL)
	_, err := provider.Chat(context.Background(), ChatRequest{Model: "local"})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected configured timeout, got %v", err)
	}
}
